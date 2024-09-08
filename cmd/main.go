package main

import (
	"flag"
	"log"

	"github.com/citizenwallet/engine/internal/api"
	"github.com/citizenwallet/engine/internal/ws"
)

func main() {
	log.Default().Println("starting engine...")

	port := flag.Int("port", 3001, "port to listen on")

	flag.Parse()

	pools := make(map[string]*ws.ConnectionPool)

	pools["0x123:0x456"] = ws.NewConnectionPool("0x123:0x456")
	go pools["0x123:0x456"].Run()

	s := api.NewServer(pools)

	wsr := s.CreateRoutes()

	err := s.Start(*port, wsr)
	if err != nil {
		log.Default().Fatalf("error starting server: %v", err)
	}

	log.Default().Println("engine stopped")
}
