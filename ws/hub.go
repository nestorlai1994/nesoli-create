package ws

import (
	"encoding/json"
	"log"
	"sync"
)

// Event is the JSON envelope sent to all connected clients.
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Hub manages WebSocket client registration and fan-out broadcasting.
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
	}
}

// Run starts the hub's main event loop. Call as a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			count := len(h.clients)
			h.mu.Unlock()
			log.Printf("[ws] client connected (%d total)", count)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			count := len(h.clients)
			h.mu.Unlock()
			log.Printf("[ws] client disconnected (%d total)", count)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Slow consumer — drop the client
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients, client)
					close(client.send)
					h.mu.Unlock()
					h.mu.RLock()
					log.Printf("[ws] dropped slow client")
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastEvent marshals an event and sends it to all connected clients.
func (h *Hub) BroadcastEvent(eventType string, data interface{}) {
	evt := Event{Type: eventType, Data: data}
	msg, err := json.Marshal(evt)
	if err != nil {
		log.Printf("[ws] failed to marshal event: %v", err)
		return
	}
	h.broadcast <- msg
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}
