package api

import (
	"github.com/citizenwallet/engine/internal/events"
	"github.com/citizenwallet/engine/internal/logs"
	"github.com/citizenwallet/engine/internal/rpc"
	"github.com/citizenwallet/engine/internal/version"
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

	// instantiate handlers
	v := version.NewService()
	l := logs.NewService(s.chainID, s.db, s.evm)
	events := events.NewHandlers(s.pools)
	rpc := rpc.NewHandlers()

	// configure routes
	cr.Route("/version", func(cr chi.Router) {
		cr.Get("/", v.Current)
	})

	cr.Route("/logs/v2/transfers", func(cr chi.Router) {
		cr.Route("/{token_address}", func(cr chi.Router) {
			cr.Route("/{signature}", func(cr chi.Router) {
				cr.Get("/", l.Get)
				cr.Get("/all", l.GetAll)

				cr.Get("/new", l.GetNew)
				cr.Get("/new/all", l.GetAllNew)
			})

			cr.Get("/tx/{hash}", l.GetSingle)
		})
	})

	cr.Get("/events/{contract}/{topic}", events.HandleConnection) // for listening to events
	cr.Get("/rpc", rpc.HandleConnection)                          // for sending RPC calls
	return cr
}
