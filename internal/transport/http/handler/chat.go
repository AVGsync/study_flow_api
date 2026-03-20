package handler

import (
	"net/http"

	"github.com/AVGsync/study_flow_api/internal/authctx"
	"github.com/AVGsync/study_flow_api/internal/model"
	"github.com/AVGsync/study_flow_api/internal/transport/ws"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChatHandler struct {
	hub *ws.Hub
}

func NewChatHandler(hub *ws.Hub) *ChatHandler {
	return &ChatHandler{hub: hub}
}

func (h *ChatHandler) ServeWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := authctx.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		client := &ws.Client{
			Hub:  h.hub,
			Conn: conn,
			ID:   userID,
			Send: make(chan *model.Message, 256),
		}
		h.hub.Register <- client

		go client.WritePump()
		go client.ReadPump()
	}
}
