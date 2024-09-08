package api

import (
	"github.com/citizenwallet/engine/internal/events"
	"github.com/citizenwallet/engine/internal/rpc"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (s *Server) AddMiddleware(cr *chi.Mux) *chi.Mux {

	// configure middleware
	cr.Use(middleware.RequestID)
	cr.Use(middleware.Logger)

	// configure custom middleware
	cr.Use(OptionsMiddleware)
	cr.Use(HealthMiddleware)
	cr.Use(RequestSizeLimitMiddleware(10 << 20)) // Limit request bodies to 10MB
	cr.Use(middleware.Compress(9))

	return cr
}

func (s *Server) CreateRoutes() *chi.Mux {
	cr := chi.NewRouter()

	events := events.NewHandlers(s.pools)

	rpc := rpc.NewHandlers()

	cr.Get("/events/{contract}/{topic}", events.HandleConnection) // for listening to events
	cr.Get("/rpc", rpc.HandleConnection)                          // for sending RPC calls
	return cr
}
