package events

import (
	"fmt"
	"net/http"

	"github.com/citizenwallet/engine/internal/ws"
	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	pools map[string]*ws.ConnectionPool
}

func NewHandlers(pools map[string]*ws.ConnectionPool) *Handlers {
	return &Handlers{
		pools: pools,
	}
}

func (h *Handlers) HandleConnection(w http.ResponseWriter, r *http.Request) {
	contract := chi.URLParam(r, "contract")
	topic := chi.URLParam(r, "topic")
	if contract == "" || topic == "" {
		http.Error(w, "contract and topic are required", http.StatusBadRequest)
		return
	}

	println("contract", contract)
	println("topic", topic)

	poolName := fmt.Sprintf("%s:%s", contract, topic)

	pool, ok := h.pools[poolName]
	if !ok {
		http.Error(w, "pool not found", http.StatusNotFound)
		return
	}

	pool.Connect(w, r)

	pool.BroadcastMessage([]byte("Hello World"))
}
