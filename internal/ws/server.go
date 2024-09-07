package ws

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type Server struct{}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) CreateRoutes() *chi.Mux {
	cr := chi.NewRouter()

	cr.Get("/ws", handleWebSocket)
	return cr
}

func (s *Server) Start(port int, handler http.Handler) error {
	// start the server
	log.Printf("WebSocket server starting on :%v", port)
	return http.ListenAndServe(fmt.Sprintf(":%v", port), handler)
}

func (s *Server) Stop() {

}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this example
	},
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to WebSocket:", err)
		return
	}
	defer conn.Close()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			return
		}

		if string(p) == "ping" {
			err = conn.WriteMessage(messageType, []byte("pong"))
			if err != nil {
				log.Println("Error writing message:", err)
				return
			}
		}
	}
}
