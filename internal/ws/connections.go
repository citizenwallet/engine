package ws

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	query string
	conn  *websocket.Conn
	send  chan []byte
}

type ConnectionPool struct {
	topic      string
	clients    map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mutex      sync.Mutex
	open       bool
}

func NewConnectionPool(topic string) *ConnectionPool {
	return &ConnectionPool{
		topic:      topic,
		clients:    make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
		open:       true,
	}
}

func (cm *ConnectionPool) Connect(w http.ResponseWriter, r *http.Request) {
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

	query := r.URL.RawQuery

	client := &Client{conn: conn, send: make(chan []byte, 256), query: query}
	cm.register <- client

	go cm.readPump(client)
	go cm.writePump(client)
}

func (cm *ConnectionPool) readPump(client *Client) {
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

func (cm *ConnectionPool) writePump(client *Client) {
	// Add ping-pong handlers to catch if the client disconnects
	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	ticker := time.NewTicker(54 * time.Second)
	defer ticker.Stop()

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
		case <-ticker.C:
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Run this method in a separate goroutine
// Run manages the main loop for the ConnectionPool, handling client registration,
// unregistration, and message broadcasting. This method should be run in a separate goroutine.
func (cm *ConnectionPool) Run() error {
	defer cm.Close()

	for {
		select {
		case client := <-cm.register:
			// Register a new client
			cm.mutex.Lock()
			if _, ok := cm.clients[client.query]; !ok {
				cm.clients[client.query] = make(map[*Client]bool)
			}
			cm.clients[client.query][client] = true
			cm.mutex.Unlock()
		case client := <-cm.unregister:
			// Unregister a client and close its send channel
			cm.mutex.Lock()
			if _, ok := cm.clients[client.query]; ok {
				delete(cm.clients, client.query)

				// if there are no more clients for this query, remove the query
				if len(cm.clients[client.query]) == 0 {
					delete(cm.clients, client.query)
				}

				close(client.send)
			}
			// Check if this was the last client
			if len(cm.clients) == 0 {
				cm.mutex.Unlock()
				return nil // This will trigger the deferred Close()
			}
			cm.mutex.Unlock()
			// case message := <-cm.broadcast:
			// 	// Broadcast a message to all connected clients
			// 	cm.BroadcastMessage(message)
		}
	}
}

func (cm *ConnectionPool) Close() {
	cm.open = false

	for _, clients := range cm.clients {
		for client := range clients {
			cm.unregister <- client
		}
	}

	close(cm.register)
	close(cm.unregister)
	close(cm.broadcast)
}

func (cm *ConnectionPool) IsOpen() bool {
	return cm.open
}

// returns all queries in the connection pool
func (cm *ConnectionPool) Queries() []string {
	cm.mutex.Lock()
	queries := make([]string, 0, len(cm.clients))
	for query := range cm.clients {
		queries = append(queries, query)
	}
	cm.mutex.Unlock()
	return queries
}

// broadcastMessage sends a message to all connected clients.
// If a client's send channel is full, it is unregistered.
func (cm *ConnectionPool) BroadcastMessage(query string, message []byte) {
	// Create a copy of the clients map to avoid holding the lock while sending
	cm.mutex.Lock()
	clients := make([]*Client, 0, len(cm.clients[query]))
	for client := range cm.clients[query] {
		clients = append(clients, client)
	}
	cm.mutex.Unlock()

	// Send the message to each client
	for _, client := range clients {
		select {
		case client.send <- message:
			// Message sent successfully
		default:
			// Client's send channel is full, unregister it
			go func(c *Client) {
				cm.unregister <- c
			}(client)
		}
	}
}
