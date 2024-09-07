package ws

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

type Handlers struct {
	upgrader websocket.Upgrader
	Manager  *ConnectionManager
}

func NewHandlers() *Handlers {
	return &Handlers{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for this example
			},
		},
		Manager: NewConnectionManager(),
	}
}

func (h *Handlers) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to WebSocket:", err)
		return
	}

	client := &Client{conn: conn, send: make(chan []byte, 256)}
	h.Manager.register <- client

	go h.readPump(client)
	go h.writePump(client)
}
