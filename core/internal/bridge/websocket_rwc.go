package bridge

import (
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketRWC wraps a gorilla websocket connection to implement net.Conn
// so it can be used with Yamux.
type WebSocketRWC struct {
	conn *websocket.Conn
	r    io.Reader
}

func NewWebSocketRWC(conn *websocket.Conn) *WebSocketRWC {
	return &WebSocketRWC{conn: conn}
}

func (w *WebSocketRWC) Read(p []byte) (int, error) {
	for {
		if w.r == nil {
			_, r, err := w.conn.NextReader()
			if err != nil {
				return 0, err
			}
			w.r = r
		}
		n, err := w.r.Read(p)
		if err == io.EOF {
			w.r = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (w *WebSocketRWC) Write(p []byte) (int, error) {
	err := w.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *WebSocketRWC) Close() error {
	return w.conn.Close()
}

func (w *WebSocketRWC) LocalAddr() net.Addr {
	return w.conn.LocalAddr()
}

func (w *WebSocketRWC) RemoteAddr() net.Addr {
	return w.conn.RemoteAddr()
}

func (w *WebSocketRWC) SetDeadline(t time.Time) error {
	if err := w.conn.SetReadDeadline(t); err != nil {
		return err
	}
	return w.conn.SetWriteDeadline(t)
}

func (w *WebSocketRWC) SetReadDeadline(t time.Time) error {
	return w.conn.SetReadDeadline(t)
}

func (w *WebSocketRWC) SetWriteDeadline(t time.Time) error {
	return w.conn.SetWriteDeadline(t)
}
