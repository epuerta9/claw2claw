// Package client provides the claw2claw client for sending/receiving files
package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/epuerta9/claw2claw/internal/crypto"
	"github.com/epuerta9/claw2claw/internal/protocol"
	"github.com/epuerta9/claw2claw/pkg/pake"
	"github.com/gorilla/websocket"
)

var (
	ErrNotConnected     = errors.New("not connected to relay")
	ErrTransferFailed   = errors.New("file transfer failed")
	ErrPakeExchangeFailed = errors.New("PAKE key exchange failed")
)

// Config holds client configuration
type Config struct {
	RelayURL string
	Timeout  time.Duration
}

// DefaultConfig returns default client configuration
func DefaultConfig() *Config {
	return &Config{
		RelayURL: "wss://claw2claw-relay.fly.dev/ws",
		Timeout:  60 * time.Second,
	}
}

// Client is the claw2claw client for secure file transfer
type Client struct {
	config    *Config
	conn      *websocket.Conn
	connMu    sync.Mutex
	sessionKey []byte
}

// New creates a new claw2claw client
func New(config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}
	return &Client{config: config}
}

// Send sends a file to a receiver using the given code phrase
// Returns the code phrase to share with the receiver
func (c *Client) Send(ctx context.Context, filePath string, codePhrase string) error {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	filename := filepath.Base(filePath)

	// Create PAKE session as sender
	session, err := pake.NewSession(codePhrase, pake.RoleSender)
	if err != nil {
		return fmt.Errorf("failed to create PAKE session: %w", err)
	}

	// Connect to relay
	if err := c.connect(ctx); err != nil {
		return err
	}
	defer c.disconnect()

	// Create room with code hash and wait for confirmation
	codeHash := session.GetCodeHashString()
	if err := c.createRoom(ctx, codeHash); err != nil {
		return err
	}

	// Wait for receiver to join (ROOM_READY signal)
	if err := c.waitForPeer(ctx); err != nil {
		return err
	}

	// PAKE exchange
	sessionKey, err := c.performPakeExchange(ctx, session, true)
	if err != nil {
		return err
	}
	c.sessionKey = sessionKey

	// Encrypt and send content
	encryptedContent, err := crypto.Encrypt(c.sessionKey, content)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	// Encrypt filename too
	encryptedFilename, err := crypto.Encrypt(c.sessionKey, []byte(filename))
	if err != nil {
		return fmt.Errorf("filename encryption failed: %w", err)
	}

	// Send encrypted payload
	payload := &protocol.EncryptedPayload{
		Filename:   encryptedFilename,
		Data:       encryptedContent,
		TotalParts: 1,
		PartNum:    0,
	}

	msg, _ := protocol.NewMessage(protocol.MsgEncrypted, codeHash, payload)
	if err := c.sendMessage(msg); err != nil {
		return err
	}

	// Wait for ACK
	return c.waitForAck(ctx)
}

// Receive receives a file using the code phrase
func (c *Client) Receive(ctx context.Context, codePhrase string, outputDir string) (string, error) {
	// Create PAKE session as receiver
	session, err := pake.NewSession(codePhrase, pake.RoleReceiver)
	if err != nil {
		return "", fmt.Errorf("failed to create PAKE session: %w", err)
	}

	// Connect to relay
	if err := c.connect(ctx); err != nil {
		return "", err
	}
	defer c.disconnect()

	// Join room with code hash and wait for both peers ready
	codeHash := session.GetCodeHashString()
	if err := c.joinRoom(ctx, codeHash); err != nil {
		return "", err
	}

	// PAKE exchange
	sessionKey, err := c.performPakeExchange(ctx, session, false)
	if err != nil {
		return "", err
	}
	c.sessionKey = sessionKey

	// Receive encrypted content
	msg, err := c.receiveMessage(ctx)
	if err != nil {
		return "", err
	}

	if msg.Type != protocol.MsgEncrypted {
		return "", fmt.Errorf("unexpected message type: %s", msg.Type)
	}

	var payload protocol.EncryptedPayload
	if err := msg.GetPayload(&payload); err != nil {
		return "", err
	}

	// Decrypt content
	content, err := crypto.Decrypt(c.sessionKey, payload.Data)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	// Decrypt filename
	filename, err := crypto.Decrypt(c.sessionKey, payload.Filename)
	if err != nil {
		return "", fmt.Errorf("filename decryption failed: %w", err)
	}

	// Write to output
	outputPath := filepath.Join(outputDir, string(filename))
	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Send ACK
	ackMsg, _ := protocol.NewMessage(protocol.MsgAck, codeHash, nil)
	if err := c.sendMessage(ackMsg); err != nil {
		return "", err
	}

	return outputPath, nil
}

// SendPersistentWithCallback sends a file to a persistent room, calling onRoomCreated with the UUID
// This allows the caller to display the room ID before waiting for the receiver
func (c *Client) SendPersistentWithCallback(ctx context.Context, filePath string, codePhrase string, ttlHours int, onRoomCreated func(roomID string)) error {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	filename := filepath.Base(filePath)

	// Create PAKE session (code phrase is used for encryption key derivation)
	session, err := pake.NewSession(codePhrase, pake.RoleSender)
	if err != nil {
		return fmt.Errorf("failed to create PAKE session: %w", err)
	}

	// Connect to relay
	if err := c.connect(ctx); err != nil {
		return err
	}
	defer c.disconnect()

	// Create persistent room
	roomID, err := c.createPersistentRoom(ctx, ttlHours)
	if err != nil {
		return err
	}

	// Callback with room ID so caller can display it
	if onRoomCreated != nil {
		onRoomCreated(roomID)
	}

	// Wait for receiver to join
	if err := c.waitForPeer(ctx); err != nil {
		return err
	}

	// PAKE exchange
	sessionKey, err := c.performPakeExchange(ctx, session, true)
	if err != nil {
		return err
	}
	c.sessionKey = sessionKey

	// Encrypt and send content
	encryptedContent, err := crypto.Encrypt(c.sessionKey, content)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	encryptedFilename, err := crypto.Encrypt(c.sessionKey, []byte(filename))
	if err != nil {
		return fmt.Errorf("filename encryption failed: %w", err)
	}

	payload := &protocol.EncryptedPayload{
		Filename:   encryptedFilename,
		Data:       encryptedContent,
		TotalParts: 1,
		PartNum:    0,
	}

	msg, _ := protocol.NewMessage(protocol.MsgEncrypted, roomID, payload)
	if err := c.sendMessage(msg); err != nil {
		return err
	}

	// Wait for ACK
	return c.waitForAck(ctx)
}

// ReceivePersistent receives a file from a persistent room using UUID
func (c *Client) ReceivePersistent(ctx context.Context, roomID string, codePhrase string, outputDir string) (string, error) {
	// Create PAKE session (must use same code phrase as sender)
	session, err := pake.NewSession(codePhrase, pake.RoleReceiver)
	if err != nil {
		return "", fmt.Errorf("failed to create PAKE session: %w", err)
	}

	// Connect to relay
	if err := c.connect(ctx); err != nil {
		return "", err
	}
	defer c.disconnect()

	// Join room by UUID
	if err := c.joinRoomByID(ctx, roomID); err != nil {
		return "", err
	}

	// PAKE exchange
	sessionKey, err := c.performPakeExchange(ctx, session, false)
	if err != nil {
		return "", err
	}
	c.sessionKey = sessionKey

	// Receive encrypted content
	msg, err := c.receiveMessage(ctx)
	if err != nil {
		return "", err
	}

	if msg.Type != protocol.MsgEncrypted {
		return "", fmt.Errorf("unexpected message type: %s", msg.Type)
	}

	var payload protocol.EncryptedPayload
	if err := msg.GetPayload(&payload); err != nil {
		return "", err
	}

	// Decrypt content
	content, err := crypto.Decrypt(c.sessionKey, payload.Data)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	// Decrypt filename
	filename, err := crypto.Decrypt(c.sessionKey, payload.Filename)
	if err != nil {
		return "", fmt.Errorf("filename decryption failed: %w", err)
	}

	// Write to output
	outputPath := filepath.Join(outputDir, string(filename))
	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Send ACK
	ackMsg, _ := protocol.NewMessage(protocol.MsgAck, roomID, nil)
	if err := c.sendMessage(ackMsg); err != nil {
		return "", err
	}

	return outputPath, nil
}

// connect establishes WebSocket connection to relay
func (c *Client) connect(ctx context.Context) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, c.config.RelayURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to relay: %w", err)
	}
	c.conn = conn
	return nil
}

// disconnect closes the WebSocket connection
func (c *Client) disconnect() {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// createRoom creates a new room on the relay and waits for confirmation
func (c *Client) createRoom(ctx context.Context, codeHash string) error {
	payload := &protocol.CreateRoomPayload{CodeHash: codeHash}
	msg, _ := protocol.NewMessage(protocol.MsgCreateRoom, codeHash, payload)
	if err := c.sendMessage(msg); err != nil {
		return err
	}

	// Wait for ROOM_JOINED confirmation
	response, err := c.receiveMessage(ctx)
	if err != nil {
		return err
	}
	if response.Type == protocol.MsgError {
		var errPayload protocol.ErrorPayload
		response.GetPayload(&errPayload)
		return fmt.Errorf("create room failed: %s", errPayload.Message)
	}
	if response.Type != protocol.MsgRoomJoined {
		return fmt.Errorf("expected ROOM_JOINED, got %s", response.Type)
	}
	return nil
}

// joinRoom joins an existing room on the relay and waits for confirmation
func (c *Client) joinRoom(ctx context.Context, codeHash string) error {
	payload := &protocol.JoinRoomPayload{CodeHash: codeHash}
	msg, _ := protocol.NewMessage(protocol.MsgJoinRoom, codeHash, payload)
	if err := c.sendMessage(msg); err != nil {
		return err
	}

	// Wait for ROOM_READY (sent when both peers have joined)
	response, err := c.receiveMessage(ctx)
	if err != nil {
		return err
	}
	if response.Type == protocol.MsgError {
		var errPayload protocol.ErrorPayload
		response.GetPayload(&errPayload)
		return fmt.Errorf("join room failed: %s", errPayload.Message)
	}
	if response.Type != protocol.MsgRoomReady {
		return fmt.Errorf("expected ROOM_READY, got %s", response.Type)
	}
	return nil
}

// createPersistentRoom creates a persistent room and returns the UUID
func (c *Client) createPersistentRoom(ctx context.Context, ttlHours int) (string, error) {
	payload := &protocol.CreatePersistentPayload{TTLHours: ttlHours}
	msg, _ := protocol.NewMessage(protocol.MsgCreatePersistent, "", payload)
	if err := c.sendMessage(msg); err != nil {
		return "", err
	}

	// Wait for ROOM_JOINED with room ID
	response, err := c.receiveMessage(ctx)
	if err != nil {
		return "", err
	}
	if response.Type == protocol.MsgError {
		var errPayload protocol.ErrorPayload
		response.GetPayload(&errPayload)
		return "", fmt.Errorf("create persistent room failed: %s", errPayload.Message)
	}
	if response.Type != protocol.MsgRoomJoined {
		return "", fmt.Errorf("expected ROOM_JOINED, got %s", response.Type)
	}

	// Extract room ID from response
	var createdPayload protocol.RoomCreatedPayload
	if err := response.GetPayload(&createdPayload); err != nil {
		// Fallback to using RoomID from message
		return response.RoomID, nil
	}

	return createdPayload.RoomID, nil
}

// joinRoomByID joins a room by its UUID (for persistent rooms)
func (c *Client) joinRoomByID(ctx context.Context, roomID string) error {
	payload := &protocol.JoinByIDPayload{RoomID: roomID}
	msg, _ := protocol.NewMessage(protocol.MsgJoinByID, roomID, payload)
	if err := c.sendMessage(msg); err != nil {
		return err
	}

	// Wait for ROOM_READY (sent when both peers have joined)
	response, err := c.receiveMessage(ctx)
	if err != nil {
		return err
	}
	if response.Type == protocol.MsgError {
		var errPayload protocol.ErrorPayload
		response.GetPayload(&errPayload)
		return fmt.Errorf("join room failed: %s", errPayload.Message)
	}
	if response.Type != protocol.MsgRoomReady {
		return fmt.Errorf("expected ROOM_READY, got %s", response.Type)
	}
	return nil
}

// waitForPeer waits for the other party to join the room
func (c *Client) waitForPeer(ctx context.Context) error {
	msg, err := c.receiveMessage(ctx)
	if err != nil {
		return err
	}
	if msg.Type != protocol.MsgRoomReady {
		return fmt.Errorf("expected ROOM_READY, got %s", msg.Type)
	}
	return nil
}

// performPakeExchange performs the PAKE key exchange
func (c *Client) performPakeExchange(ctx context.Context, session *pake.Session, isSender bool) ([]byte, error) {
	codeHash := session.GetCodeHashString()

	if isSender {
		// Sender: send PAKE_A, receive PAKE_B
		pakeMsg, _ := session.GetMessage()
		payload := &protocol.PakePayload{Data: pakeMsg}
		msg, _ := protocol.NewMessage(protocol.MsgPakeA, codeHash, payload)
		if err := c.sendMessage(msg); err != nil {
			return nil, err
		}

		// Receive PAKE_B
		response, err := c.receiveMessage(ctx)
		if err != nil {
			return nil, err
		}

		var pakePayload protocol.PakePayload
		if err := response.GetPayload(&pakePayload); err != nil {
			return nil, err
		}

		if err := session.ProcessMessage(pakePayload.Data); err != nil {
			return nil, ErrPakeExchangeFailed
		}
	} else {
		// Receiver: receive PAKE_A, send PAKE_B
		msg, err := c.receiveMessage(ctx)
		if err != nil {
			return nil, err
		}

		var pakePayload protocol.PakePayload
		if err := msg.GetPayload(&pakePayload); err != nil {
			return nil, err
		}

		if err := session.ProcessMessage(pakePayload.Data); err != nil {
			return nil, ErrPakeExchangeFailed
		}

		// Send PAKE_B
		pakeMsg, _ := session.GetMessage()
		payload := &protocol.PakePayload{Data: pakeMsg}
		response, _ := protocol.NewMessage(protocol.MsgPakeB, codeHash, payload)
		if err := c.sendMessage(response); err != nil {
			return nil, err
		}
	}

	return session.GetSharedKey()
}

// sendMessage sends a protocol message over WebSocket
func (c *Client) sendMessage(msg *protocol.Message) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn == nil {
		return ErrNotConnected
	}

	data, err := msg.Encode()
	if err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// receiveMessage receives a protocol message from WebSocket
func (c *Client) receiveMessage(ctx context.Context) (*protocol.Message, error) {
	if c.conn == nil {
		return nil, ErrNotConnected
	}

	// Set read deadline based on context
	if deadline, ok := ctx.Deadline(); ok {
		c.conn.SetReadDeadline(deadline)
	} else {
		c.conn.SetReadDeadline(time.Now().Add(c.config.Timeout))
	}

	_, data, err := c.conn.ReadMessage()
	if err != nil {
		if err == io.EOF {
			return nil, ErrNotConnected
		}
		return nil, err
	}

	return protocol.DecodeMessage(data)
}

// waitForAck waits for acknowledgment from receiver
func (c *Client) waitForAck(ctx context.Context) error {
	msg, err := c.receiveMessage(ctx)
	if err != nil {
		return err
	}
	if msg.Type != protocol.MsgAck {
		return fmt.Errorf("expected ACK, got %s", msg.Type)
	}
	return nil
}
