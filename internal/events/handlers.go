package events

import (
	"fmt"
	"net/http"

	"github.com/citizenwallet/engine/internal/ws"
	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	pools *ws.ConnectionPools
}

func NewHandlers(pools *ws.ConnectionPools) *Handlers {
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

	println(r.URL.RawQuery)

	println("contract", contract)
	println("topic", topic)

	poolName := fmt.Sprintf("%s/%s", contract, topic)

	h.pools.Connect(w, r, poolName)
}
