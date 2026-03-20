package ws

import (
	"encoding/json"
	"log/slog"

	"github.com/AVGsync/study_flow_api/internal/model"
	"github.com/gorilla/websocket"
)

type Client struct {
	Hub  *Hub
	Conn *websocket.Conn
	ID   string
	Send chan *model.Message
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	for {
		_, messageData, err := c.Conn.ReadMessage()
		if err != nil {
			slog.Error("read error", "client_id", c.ID, "error", err)
			break
		}

		var msg model.Message
		if err := json.Unmarshal(messageData, &msg); err != nil {
			slog.Error("invalid message format", "client_id", c.ID, "error", err)
			continue
		}

		msg.FromID = c.ID
		c.Hub.broadcast <- &msg
	}
}

func (c *Client) WritePump() {
	defer func() {
		c.Conn.Close()
	}()

	for {
		message, ok := <-c.Send
		if !ok {
			c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		messageBytes, err := json.Marshal(message)
		if err != nil {
			continue
		}

		err = c.Conn.WriteMessage(websocket.TextMessage, messageBytes)
		if err != nil {
			slog.Error("write error", "client_id", c.ID, "error", err)
			return
		}
	}
}
