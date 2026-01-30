// Package protocol defines the wire protocol for claw2claw communication
package protocol

import (
	"encoding/json"
	"time"
)

// MessageType defines the type of protocol message
type MessageType string

const (
	// Room management
	MsgCreateRoom MessageType = "CREATE_ROOM"
	MsgJoinRoom   MessageType = "JOIN_ROOM"
	MsgRoomJoined MessageType = "ROOM_JOINED"
	MsgRoomReady  MessageType = "ROOM_READY"

	// PAKE exchange
	MsgPakeA MessageType = "PAKE_A" // Sender's PAKE message
	MsgPakeB MessageType = "PAKE_B" // Receiver's PAKE response

	// Content transfer
	MsgEncrypted MessageType = "ENCRYPTED" // Encrypted content
	MsgAck       MessageType = "ACK"       // Acknowledgment

	// Control
	MsgError MessageType = "ERROR"
	MsgClose MessageType = "CLOSE"
)

// Message is the wire format for all protocol messages
type Message struct {
	Type      MessageType     `json:"type"`
	RoomID    string          `json:"room_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp int64           `json:"ts"`
}

// NewMessage creates a new protocol message with current timestamp
func NewMessage(msgType MessageType, roomID string, payload interface{}) (*Message, error) {
	var payloadJSON json.RawMessage
	if payload != nil {
		var err error
		payloadJSON, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	return &Message{
		Type:      msgType,
		RoomID:    roomID,
		Payload:   payloadJSON,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

// CreateRoomPayload is sent when creating a new room
type CreateRoomPayload struct {
	CodeHash string `json:"code_hash"` // SHA-256 hash of code phrase (base64)
}

// JoinRoomPayload is sent when joining an existing room
type JoinRoomPayload struct {
	CodeHash string `json:"code_hash"` // Must match creator's hash
}

// PakePayload contains PAKE exchange data
type PakePayload struct {
	Data []byte `json:"data"` // PAKE message bytes
}

// EncryptedPayload contains encrypted content
type EncryptedPayload struct {
	Filename   string `json:"filename"`   // Original filename (encrypted separately)
	Data       []byte `json:"data"`       // Encrypted content
	TotalParts int    `json:"total_parts"` // For chunked transfers
	PartNum    int    `json:"part_num"`    // Current part (0-indexed)
}

// ErrorPayload contains error details
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Common error codes
const (
	ErrCodeRoomNotFound    = "ROOM_NOT_FOUND"
	ErrCodeRoomFull        = "ROOM_FULL"
	ErrCodeCodeMismatch    = "CODE_MISMATCH"
	ErrCodePakeFailed      = "PAKE_FAILED"
	ErrCodeTransferFailed  = "TRANSFER_FAILED"
	ErrCodeTimeout         = "TIMEOUT"
)

// Encode serializes a message to JSON bytes
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage deserializes JSON bytes to a Message
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetPayload unmarshals the payload into the provided type
func (m *Message) GetPayload(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}
