package handlers

import (
	"net/http"

	"github.com/AVGsync/study_flow_api/internal/auth"
	"github.com/AVGsync/study_flow_api/internal/chat"
	"github.com/AVGsync/study_flow_api/internal/models"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChatHandler struct {
	hub *chat.Hub
}

func NewChatHandler(hub *chat.Hub) *ChatHandler {
	return &ChatHandler{hub: hub}
}

func (h *ChatHandler) ServeWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		client := &chat.Client{
			Hub: h.hub,
			Conn: conn,
			ID: userID,
			Send: make(chan *models.Message, 256),
		}
		h.hub.Register <- client

		go client.WritePump()
		go client.ReadPump()
	}
}