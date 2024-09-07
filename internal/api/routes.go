package api

import (
	"github.com/citizenwallet/engine/internal/ws"
	"github.com/go-chi/chi/v5"
)

func (s *Server) CreateRoutes() *chi.Mux {
	cr := chi.NewRouter()

	w := ws.NewHandlers()
	go w.Manager.Run()

	cr.Get("/ws", w.HandleWebSocket)
	return cr
}
