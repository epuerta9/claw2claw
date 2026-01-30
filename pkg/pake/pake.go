// Package pake provides Password-Authenticated Key Exchange functionality
// This wraps schollz/pake for claw2claw's secure key establishment
package pake

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"

	"github.com/schollz/pake/v3"
)

var (
	ErrKeyExchangeFailed = errors.New("PAKE key exchange failed")
	ErrInvalidMessage    = errors.New("invalid PAKE message")
)

// Session represents a PAKE session for key exchange
type Session struct {
	pake     *pake.Pake
	role     Role
	codeHash []byte
}

// Role indicates sender (A) or receiver (B) in PAKE exchange
type Role int

const (
	RoleSender   Role = 0 // "A" in PAKE terminology
	RoleReceiver Role = 1 // "B" in PAKE terminology
)

// NewSession creates a new PAKE session with the given code phrase
func NewSession(codePhrase string, role Role) (*Session, error) {
	// Hash the code phrase for relay room matching (relay only sees hash)
	codeHash := hashCode(codePhrase)

	// Initialize PAKE with code as password
	// Using P-256 curve for good security/performance balance
	weak := []byte(codePhrase)
	p, err := pake.InitCurve(weak, int(role), "p256")
	if err != nil {
		return nil, err
	}

	return &Session{
		pake:     p,
		role:     role,
		codeHash: codeHash,
	}, nil
}

// GetCodeHash returns the hash of the code phrase (for relay room matching)
func (s *Session) GetCodeHash() []byte {
	return s.codeHash
}

// GetCodeHashString returns base64-encoded code hash
func (s *Session) GetCodeHashString() string {
	return base64.URLEncoding.EncodeToString(s.codeHash)
}

// GetMessage returns the current PAKE message to send to peer
func (s *Session) GetMessage() ([]byte, error) {
	return s.pake.Bytes(), nil
}

// ProcessMessage processes a received PAKE message from peer
func (s *Session) ProcessMessage(msg []byte) error {
	return s.pake.Update(msg)
}

// GetSharedKey returns the derived shared secret after PAKE exchange completes
func (s *Session) GetSharedKey() ([]byte, error) {
	key, err := s.pake.SessionKey()
	if err != nil {
		return nil, ErrKeyExchangeFailed
	}
	return key, nil
}

// IsComplete returns true if PAKE exchange is complete
func (s *Session) IsComplete() bool {
	_, err := s.pake.SessionKey()
	return err == nil
}

// hashCode creates a SHA-256 hash of the code phrase
// This is what the relay sees - it can match rooms but not derive the key
func hashCode(code string) []byte {
	hash := sha256.Sum256([]byte(code))
	return hash[:]
}
