package ws

import (
	"encoding/json"
	"log"
)

// Event is the JSON envelope sent to all connected clients.
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Hub manages WebSocket client registration and fan-out broadcasting.
// All state is owned exclusively by the Run() goroutine — no mutex needed.
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
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
			h.clients[client] = true
			log.Printf("[ws] client connected (%d total)", len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			log.Printf("[ws] client disconnected (%d total)", len(h.clients))

		case message := <-h.broadcast:
			var stale []*Client
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					stale = append(stale, client)
				}
			}
			for _, client := range stale {
				delete(h.clients, client)
				close(client.send)
				log.Printf("[ws] dropped slow client (%d total)", len(h.clients))
			}
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
