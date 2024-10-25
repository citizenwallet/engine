package main

import (
	"context"
	"flag"
	"log"

	"github.com/citizenwallet/engine/internal/api"
	"github.com/citizenwallet/engine/internal/bucket"
	"github.com/citizenwallet/engine/internal/config"
	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/internal/ethrequest"
	"github.com/citizenwallet/engine/internal/indexer"
	"github.com/citizenwallet/engine/internal/queue"
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

	d, err := db.NewDB(chid, conf.DBSecret, conf.DBUser, conf.DBPassword, conf.DBName, conf.DBHost, conf.DBReaderHost)
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
	// push queue
	log.Default().Println("starting push queue service...")

	pu := queue.NewPushService()

	pushqueue, pushqerr := queue.NewService("push", 3, *useropqbf, ctx)
	defer pushqueue.Close()

	go func() {
		for err := range pushqerr {
			// TODO: handle errors coming from the queue
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

		idx := indexer.NewIndexer(ctx, d, evm, pools)
		go func() {
			quitAck <- idx.Start()
		}()
	}
	////////////////////

	////////////////////
	// userop queue
	log.Default().Println("starting userop queue service...")

	op := queue.NewUserOpService(d, evm, pushqueue, pools)

	useropq, qerr := queue.NewService("userop", 3, *useropqbf, ctx)
	defer useropq.Close()

	go func() {
		for err := range qerr {
			// TODO: handle errors coming from the queue
			log.Default().Println(err.Error())
		}
	}()

	go func() {
		quitAck <- useropq.Start(op)
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
			// w.NotifyError(ctx, err)
			// sentry.CaptureException(err)
			log.Fatal(err)
		}
	}

	log.Default().Println("engine stopped")
}
