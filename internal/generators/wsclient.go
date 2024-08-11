package generators

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	Conn *websocket.Conn
	url  string
	auth string
	done chan struct{}
}

func NewWSClient(url string, auth string) (*WSClient, error) {

	Conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{
		"Authorization": {auth},
	})
	defer Conn.Close()

	if err != nil {
		return nil, err
	}

	client := &WSClient{
		Conn: Conn,
		url:  url,
		auth: auth,
		done: make(chan struct{}),
	}

	go client.listenMessages()

	return client, nil
}

func (c *WSClient) listenMessages() {
	defer close(c.done)
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			return
		}
		log.Printf("Received: %s", message)
	}
}

func (c *WSClient) reConnect() error {
	Conn, _, err := websocket.DefaultDialer.Dial(c.url, http.Header{
		"Authorization": {c.auth},
	})

	if err != nil {
		return err
	}

	c.Conn = Conn

	return nil
}

func (c *WSClient) SendMessage(message string) error {
	err := c.Conn.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		if err := c.reConnect(); err != nil {
			return err
		}

		// Retry sending the message after reConnecting
		err = c.Conn.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *WSClient) ReadMessages() {
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			return
		}

		log.Print(message)
	}
}

func (c *WSClient) Close() error {
	err := c.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		return err
	}
	select {
	case <-c.done:
	case <-time.After(time.Second):
	}
	return c.Conn.Close()
}

// WaitForInterrupt waits for an interrupt signal to gracefully close the WebSocket Connection.
func (c *WSClient) WaitForInterrupt() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	select {
	case <-interrupt:
		log.Println("Interrupt received, closing Connection...")
		c.Close()
	}
}
