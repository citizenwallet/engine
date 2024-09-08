package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/citizenwallet/engine/internal/ws"
)

type Server struct {
	pools map[string]*ws.ConnectionPool
}

func NewServer(pools map[string]*ws.ConnectionPool) *Server {
	return &Server{pools: pools}
}

func (s *Server) Start(port int, handler http.Handler) error {
	// start the server
	log.Printf("API server starting on :%v", port)
	return http.ListenAndServe(fmt.Sprintf(":%v", port), handler)
}

func (s *Server) Stop() {

}
