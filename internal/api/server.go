package api

import (
	"fmt"
	"log"
	"math/big"
	"net/http"

	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/internal/ws"
	"github.com/citizenwallet/engine/pkg/engine"
)

type Server struct {
	chainID *big.Int
	db      *db.DB
	evm     engine.EVMRequester
	pools   map[string]*ws.ConnectionPool
}

func NewServer(chainID *big.Int, db *db.DB, evm engine.EVMRequester, pools map[string]*ws.ConnectionPool) *Server {
	return &Server{chainID: chainID, db: db, evm: evm, pools: pools}
}

func (s *Server) Start(port int, handler http.Handler) error {
	// start the server
	log.Printf("API server starting on :%v", port)
	return http.ListenAndServe(fmt.Sprintf(":%v", port), handler)
}

func (s *Server) Stop() {

}
