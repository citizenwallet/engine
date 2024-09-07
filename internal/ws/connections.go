package ws

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type ConnectionManager struct {
	topic      string
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mutex      sync.Mutex
}

func NewConnectionManager(topic string) *ConnectionManager {
	return &ConnectionManager{
		topic:      topic,
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
	}
}

func (cm *ConnectionManager) Connect(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for this example
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to WebSocket:", err)
		return
	}

	client := &Client{conn: conn, send: make(chan []byte, 256)}
	cm.register <- client

	go cm.readPump(client)
	go cm.writePump(client)
}

func (cm *ConnectionManager) readPump(client *Client) {
	defer func() {
		cm.unregister <- client
		client.conn.Close()
	}()

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		if string(message) == "ping" {
			client.send <- []byte("pong")
		}
	}
}

func (cm *ConnectionManager) writePump(client *Client) {
	defer func() {
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		}
	}
}

// Run this method in a separate goroutine
func (cm *ConnectionManager) Run() {
	for {
		select {
		case client := <-cm.register:
			cm.mutex.Lock()
			cm.clients[client] = true
			cm.mutex.Unlock()
		case client := <-cm.unregister:
			cm.mutex.Lock()
			if _, ok := cm.clients[client]; ok {
				delete(cm.clients, client)
				close(client.send)
			}
			cm.mutex.Unlock()
		case message := <-cm.broadcast:
			cm.mutex.Lock()
			for client := range cm.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(cm.clients, client)
				}
			}
			cm.mutex.Unlock()
		}
	}
}
