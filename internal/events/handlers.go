package events

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/citizenwallet/engine/internal/db"
	"github.com/citizenwallet/engine/internal/ws"
	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	db    *db.DB
	pools *ws.ConnectionPools
}

func NewHandlers(db *db.DB, pools *ws.ConnectionPools) *Handlers {
	return &Handlers{
		db:    db,
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

	exists, err := h.db.EventDB.EventExists(contract)
	if err != nil || !exists {
		http.Error(w, "event does not exist", http.StatusNotFound)
		return
	}

	poolName := fmt.Sprintf("%s/%s", contract, topic)

	h.pools.Connect(w, r, strings.ToLower(poolName))
}
