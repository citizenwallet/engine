package events

import (
	"net/http"

	"github.com/citizenwallet/engine/internal/ws"
)

type Handlers struct {
	Manager *ws.ConnectionManager
}

func NewHandlers() *Handlers {
	return &Handlers{
		Manager: ws.NewConnectionManager("events"),
	}
}

func (h *Handlers) HandleConnection(w http.ResponseWriter, r *http.Request) {
	h.Manager.Connect(w, r)
}
