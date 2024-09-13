package ws

import (
	"net/http"
	"sync"
)

type ConnectionPools struct {
	pools map[string]*ConnectionPool
	mu    sync.Mutex
}

func NewConnectionPools() *ConnectionPools {
	return &ConnectionPools{
		pools: make(map[string]*ConnectionPool),
	}
}

// Connect connects a client to a topic or creates a new topic
func (p *ConnectionPools) Connect(w http.ResponseWriter, r *http.Request, topic string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.pools[topic]; !ok || !p.pools[topic].IsOpen() {
		p.pools[topic] = NewConnectionPool(topic)

		go p.pools[topic].Run()
	}

	p.pools[topic].Connect(w, r)
}
