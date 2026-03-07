package chat

import (
	"github.com/AVGsync/study_flow_api/internal/models"
)

type Hub struct {
	clients map[string]*Client
	broadcast chan *models.Message
	Register chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		broadcast:  make(chan *models.Message),
		Register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
			case client := <-h.Register:
				h.clients[client.ID] = client

			case client := <-h.unregister:
				if _, ok := h.clients[client.ID]; ok {
					delete(h.clients, client.ID)
					close(client.Send)
				}
			
			case message := <-h.broadcast:
				client, ok := h.clients[message.ToID]
				if ok {
					select {
						case client.Send <- message:
						default:
							close(client.Send)
							delete(h.clients, client.ID)
					}
				}
		}
	}
}