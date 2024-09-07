package main

import (
	"flag"
	"log"

	"github.com/citizenwallet/engine/internal/api"
)

func main() {
	log.Default().Println("starting engine...")

	port := flag.Int("port", 3001, "port to listen on")

	flag.Parse()

	s := api.NewServer()

	wsr := s.CreateRoutes()

	err := s.Start(*port, wsr)
	if err != nil {
		log.Default().Fatalf("error starting server: %v", err)
	}

	log.Default().Println("engine stopped")
}
