package main

import (
	"flag"
	"log"

	"github.com/citizenwallet/engine/internal/ws"
)

func main() {
	log.Default().Println("starting engine...")

	port := flag.Int("port", 8080, "port to listen on")

	flag.Parse()

	s := ws.NewServer()

	wsr := s.CreateRoutes()

	err := s.Start(*port, wsr)
	if err != nil {
		log.Default().Fatalf("error starting server: %v", err)
	}

	log.Default().Println("engine stopped")
}
