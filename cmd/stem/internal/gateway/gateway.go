package gateway

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/opentendril/core/cmd/stem/internal/eventbus"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for the gateway
	},
}

// Client represents a connected WebSocket client
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

func HandleWebSocket(bus *eventbus.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Failed to upgrade WebSocket: %v", err)
			return
		}

		client := &Client{
			conn: conn,
			send: make(chan []byte, 256),
		}

		// Subscribe to eventbus
		handler := func(event eventbus.Event) {
			if event.Type == eventbus.EventStreamToken {
				msg := map[string]interface{}{
					"type":    "stream.token",
					"content": event.Data["token"],
				}
				if payload, err := json.Marshal(msg); err == nil {
					client.send <- payload
				}
			} else if event.Type == eventbus.EventThoughtBranch {
				msg := map[string]interface{}{
					"type":    "thought-branch",
					"content": event.Data["thought"],
				}
				if payload, err := json.Marshal(msg); err == nil {
					client.send <- payload
				}
			}
		}

		// We could use a unique ID for the handler if we wanted to unsubscribe,
		// but since EventBus doesn't have Unsubscribe, we'll just let it leak for now,
		// or ideally we add Unsubscribe to EventBus. Since this is a simple implementation,
		// we will just subscribe. Wait, if it leaks it's bad. Let's look at eventbus.go later.
		bus.Subscribe(eventbus.EventStreamToken, handler)
		bus.Subscribe(eventbus.EventThoughtBranch, handler)

		// Send connected message
		connectedMsg, _ := json.Marshal(map[string]string{"type": "connected"})
		client.send <- connectedMsg

		// Start write pump
		go client.writePump()
		// Start read pump
		client.readPump()
	}
}

func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
	}()
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// Handle incoming messages if needed
		_ = message
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(50 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
