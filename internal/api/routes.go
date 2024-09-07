package api

import (
	"github.com/citizenwallet/engine/internal/events"
	"github.com/citizenwallet/engine/internal/rpc"
	"github.com/go-chi/chi/v5"
)

func (s *Server) CreateRoutes() *chi.Mux {
	cr := chi.NewRouter()

	events := events.NewHandlers()
	go events.Manager.Run()

	rpc := rpc.NewHandlers()
	go rpc.Manager.Run()

	cr.Get("/events", events.HandleConnection) // for listening to events
	cr.Get("/rpc", rpc.HandleConnection)       // for sending RPC calls
	return cr
}
