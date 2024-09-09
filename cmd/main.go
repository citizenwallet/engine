package main

import (
	"context"
	"flag"
	"log"
	"math/big"

	"github.com/citizenwallet/engine/internal/api"
	"github.com/citizenwallet/engine/internal/config"
	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/internal/ws"
)

func main() {
	log.Default().Println("starting engine...")

	port := flag.Int("port", 3001, "port to listen on")

	env := flag.String("env", ".env", "path to .env file")

	flag.Parse()

	ctx := context.Background()

	conf, err := config.New(ctx, *env)
	if err != nil {
		log.Fatal(err)
	}

	// chid, err := evm.ChainID()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	chid := big.NewInt(1337)

	log.Default().Println("node running for chain: ", chid.String())

	log.Default().Println("starting internal db service...")

	d, err := db.NewDB(chid, conf.DBSecret, conf.DBUser, conf.DBPassword, conf.DBName, conf.DBHost, conf.DBReaderHost)
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	pools := make(map[string]*ws.ConnectionPool)

	pools["0x123:0x456"] = ws.NewConnectionPool("0x123:0x456")
	go pools["0x123:0x456"].Run()

	s := api.NewServer(pools)

	wsr := s.CreateRoutes()

	err = s.Start(*port, wsr)
	if err != nil {
		log.Default().Fatalf("error starting server: %v", err)
	}

	log.Default().Println("engine stopped")
}
