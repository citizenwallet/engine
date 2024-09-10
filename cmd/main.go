package main

import (
	"context"
	"flag"
	"log"

	"github.com/citizenwallet/engine/internal/api"
	"github.com/citizenwallet/engine/internal/config"
	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/internal/ethrequest"
	"github.com/citizenwallet/engine/internal/queue"
	"github.com/citizenwallet/engine/internal/ws"
)

func main() {
	log.Default().Println("starting engine...")

	////////////////////
	// flags
	port := flag.Int("port", 3001, "port to listen on")

	env := flag.String("env", ".env", "path to .env file")

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
	evm, err := ethrequest.NewEthService(ctx, conf.RPCURL)
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
	pools := make(map[string]*ws.ConnectionPool)

	pools["0x123:0x456"] = ws.NewConnectionPool("0x123:0x456")
	go pools["0x123:0x456"].Run()
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
	// userop queue
	log.Default().Println("starting userop queue service...")

	op := queue.NewUserOpService(d, evm, pushqueue)

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

	wsr := s.CreateRoutes()

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
