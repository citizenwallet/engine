package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/citizenwallet/engine/internal/api"
	"github.com/citizenwallet/engine/internal/bucket"
	"github.com/citizenwallet/engine/internal/config"
	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/internal/ethrequest"
	"github.com/citizenwallet/engine/internal/indexer"
	"github.com/citizenwallet/engine/internal/queue"
	"github.com/citizenwallet/engine/internal/webhook"
	"github.com/citizenwallet/engine/internal/ws"
)

func main() {
	log.Default().Println("starting engine...")

	////////////////////
	// flags
	port := flag.Int("port", 3001, "port to listen on")

	env := flag.String("env", ".env", "path to .env file")

	polling := flag.Bool("polling", false, "enable polling")

	noindex := flag.Bool("noindex", false, "disable indexing")

	useropqbf := flag.Int("buffer", 1000, "userop queue buffer size (default: 1000)")

	notify := flag.Bool("notify", false, "enable webhook notifications")

	flag.Parse()
	////////////////////

	ctx := context.Background()

	////////////////////
	// config
	conf, err := config.New(ctx, *env)
	if err != nil {
		log.Fatal(err)
	}
	////////////////////

	////////////////////
	// evm
	rpcUrl := conf.RPCURL
	if !*polling {
		log.Default().Println("running in streaming mode...")
		rpcUrl = conf.RPCWSURL
	} else {
		log.Default().Println("running in polling mode...")
	}

	evm, err := ethrequest.NewEthService(ctx, rpcUrl)
	if err != nil {
		log.Fatal(err)
	}

	chid, err := evm.ChainID()
	if err != nil {
		log.Fatal(err)
	}

	log.Default().Println("node running for chain: ", chid.String())
	////////////////////

	////////////////////
	// db
	log.Default().Println("starting internal db service...")

	d, err := db.NewDB(chid, conf.DBSecret, conf.DBUser, conf.DBPassword, conf.DBName, conf.DBPort, conf.DBHost, conf.DBReaderHost)
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	////////////////////

	////////////////////
	// main error channel
	quitAck := make(chan error)
	defer close(quitAck)
	////////////////////

	////////////////////
	// pools
	pools := ws.NewConnectionPools()
	////////////////////

	////////////////////
	// webhook
	log.Default().Println("starting webhook service...")

	w := webhook.NewMessager(conf.DiscordURL, conf.ChainName, *notify)
	defer func() {
		if r := recover(); r != nil {
			// in case of a panic, notify the webhook messager with an error notification
			err := fmt.Errorf("recovered from panic: %v", r)
			log.Default().Println(err)
			w.NotifyError(ctx, err)
			// sentry.CaptureException(err)
		}
	}()

	w.Notify(ctx, "engine started")
	////////////////////

	////////////////////
	// push queue
	log.Default().Println("starting push queue service...")

	pu := queue.NewPushService()

	pushqueue, pushqerr := queue.NewService("push", 3, *useropqbf, ctx)
	defer pushqueue.Close()

	go func() {
		for err := range pushqerr {
			// TODO: handle errors coming from the queue
			w.NotifyError(ctx, err)
			log.Default().Println(err.Error())
		}
	}()

	go func() {
		quitAck <- pushqueue.Start(pu)
	}()
	////////////////////

	////////////////////
	// indexer
	if !*noindex {
		log.Default().Println("starting indexer service...")

		idx := indexer.NewIndexer(ctx, d, w, evm, pools)
		go func() {
			quitAck <- idx.Start()
		}()
	}
	////////////////////

	////////////////////
	// userop queue
	log.Default().Println("starting userop queue service...")

	op := queue.NewUserOpService(d, evm, pushqueue, pools)

	useropq, qerr := queue.NewService("userop", 10, *useropqbf, ctx)
	defer useropq.Close()

	go func() {
		for err := range qerr {
			// TODO: handle errors coming from the queue
			w.NotifyError(ctx, err)
			log.Default().Println(err.Error())
		}
	}()

	go func() {
		quitAck <- useropq.Start(op)
	}()
	////////////////////

	////////////////////
	// timeout checker
	log.Default().Println("starting timeout checker service...")

	timeoutChecker := queue.NewTimeoutService(d, evm)
	go func() {
		quitAck <- timeoutChecker.Start(ctx)
	}()
	////////////////////

	////////////////////
	// api
	s := api.NewServer(chid, d, evm, useropq, pools)

	bu := bucket.NewBucket(conf.PinataBaseURL, conf.PinataAPIKey, conf.PinataAPISecret)

	wsr := s.CreateBaseRouter()
	wsr = s.AddMiddleware(wsr)
	wsr = s.AddRoutes(wsr, bu)

	go func() {
		quitAck <- s.Start(*port, wsr)
	}()

	log.Default().Println("listening on port: ", *port)
	////////////////////

	for err := range quitAck {
		if err != nil {
			w.NotifyError(ctx, err)
			// sentry.CaptureException(err)
			log.Fatal(err)
		}
	}

	log.Default().Println("engine stopped")
}
