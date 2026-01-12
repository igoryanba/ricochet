package client

import (
	"log"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/igoryan-dao/ricochet/internal/protocol"
)

// Client handles communication with Ricochet Core Daemon
type Client struct {
	conn        *websocket.Conn
	addr        string
	send        chan interface{}
	done        chan struct{}
	OnMessage   func(protocol.RPCMessage)
	OnConnected func()
	OnClosed    func()
	mu          sync.Mutex
	idCounter   int
}

func NewClient(addr string) *Client {
	return &Client{
		addr: addr,
		send: make(chan interface{}, 100),
		done: make(chan struct{}),
	}
}

func (c *Client) Connect() error {
	u := url.URL{Scheme: "ws", Host: c.addr, Path: "/ws"}
	// log.Printf("Connecting to %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	c.conn = conn

	if c.OnConnected != nil {
		c.OnConnected()
	}

	go c.readPump()
	go c.writePump()

	return nil
}

func (c *Client) Close() {
	close(c.done)
}

func (c *Client) SendCommand(method string, payload interface{}) int {
	c.mu.Lock()
	c.idCounter++
	id := c.idCounter
	c.mu.Unlock()

	msg := protocol.RPCMessage{
		ID:      id,
		Type:    method,
		Payload: protocol.EncodeRPC(payload),
	}
	c.send <- msg
	return id
}

func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
		if c.OnClosed != nil {
			c.OnClosed()
		}
	}()

	for {
		var msg protocol.RPCMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			// log.Printf("Read error: %v", err)
			return
		}
		if c.OnMessage != nil {
			c.OnMessage(msg)
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			err := c.conn.WriteJSON(msg)
			if err != nil {
				log.Printf("Write error: %v", err)
				return
			}
		case <-c.done:
			return
		}
	}
}
