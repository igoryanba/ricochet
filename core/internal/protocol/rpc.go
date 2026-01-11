package protocol

import (
	"encoding/base64"
	"encoding/json"
)

// RPCMessage represents a JSON-RPC 2.0-like message.
// It can be a notification (no ID), a request (has ID), or a response (has ID + type:"response").
type RPCMessage struct {
	ID      interface{}     `json:"id,omitempty"`      // string or number
	Type    string          `json:"type"`              // Message type (e.g. "chat_message", "ask_user")
	Payload json.RawMessage `json:"payload,omitempty"` // Typed payload
	Error   string          `json:"error,omitempty"`   // Optional error message
}

// EncodeRPC encodes any payload into a RawMessage for inclusion in an RPCMessage
func EncodeRPC(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// DecodeBase64 decodes a base64 string into a byte slice
func DecodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
