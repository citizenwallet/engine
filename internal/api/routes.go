package api

import (
	"github.com/citizenwallet/engine/internal/chain"
	"github.com/citizenwallet/engine/internal/events"
	"github.com/citizenwallet/engine/internal/logs"
	"github.com/citizenwallet/engine/internal/paymaster"
	"github.com/citizenwallet/engine/internal/rpc"
	"github.com/citizenwallet/engine/internal/userop"
	"github.com/citizenwallet/engine/internal/version"
	"github.com/citizenwallet/engine/pkg/engine"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (s *Server) CreateBaseRouter() *chi.Mux {
	cr := chi.NewRouter()

	return cr
}

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

func (s *Server) AddRoutes(cr *chi.Mux) *chi.Mux {
	// instantiate handlers
	v := version.NewService()
	l := logs.NewService(s.chainID, s.db, s.evm)
	events := events.NewHandlers(s.pools)
	rpc := rpc.NewHandlers()
	pm := paymaster.NewService(s.evm, s.db)
	uop := userop.NewService(s.evm, s.db, s.userOpQueue, s.chainID)
	ch := chain.NewService(s.evm, s.chainID)

	// configure routes
	cr.Route("/version", func(cr chi.Router) {
		cr.Get("/", v.Current)
	})

	cr.Route("/v1", func(cr chi.Router) {

		// logs
		cr.Route("/logs/{contract_address}", func(cr chi.Router) {
			cr.Route("/{signature}", func(cr chi.Router) {
				cr.Get("/", l.Get)
				cr.Get("/all", l.GetAll)

				cr.Get("/new", l.GetNew)
				cr.Get("/new/all", l.GetAllNew)
			})

			cr.Get("/tx/{hash}", l.GetSingle)
		})

		// rpc
		cr.Route("/rpc/{pm_address}", func(cr chi.Router) {
			cr.Post("/", withJSONRPCRequest(map[string]engine.RPCHandlerFunc{
				"pm_sponsorUserOperation":   pm.Sponsor,
				"pm_ooSponsorUserOperation": pm.OOSponsor,
				"eth_sendUserOperation":     uop.Send,
				"eth_chainId":               ch.ChainId,
			}))
		})

		cr.Get("/events/{contract}/{topic}", events.HandleConnection) // for listening to events
		cr.Get("/rpc", rpc.HandleConnection)                          // for sending RPC calls
	})

	return cr
}
