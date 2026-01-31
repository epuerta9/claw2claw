// Package manifest tracks received files and read state for incremental context
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Manifest tracks received files and their read state
type Manifest struct {
	Version   string                  `json:"version"`
	UpdatedAt time.Time               `json:"updated_at"`
	Files     map[string]*FileEntry   `json:"files"`
	Channels  map[string]*ChannelInfo `json:"channels"`
	path      string
}

// FileEntry tracks a single received file
type FileEntry struct {
	Filename    string    `json:"filename"`
	ReceivedAt  time.Time `json:"received_at"`
	LastReadAt  *time.Time `json:"last_read_at,omitempty"`
	ContentHash string    `json:"content_hash"`
	Size        int64     `json:"size"`
	FromChannel string    `json:"from_channel,omitempty"`
	Sequence    int       `json:"sequence"`
	IsNew       bool      `json:"is_new"`
}

// ChannelInfo tracks a bidirectional channel
type ChannelInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
	Code        string    `json:"code"` // Encryption code for this channel
	Role        string    `json:"role"` // "creator" or "joiner"
	MessageCount int      `json:"message_count"`
}

const manifestFile = ".claw/manifest.json"

// Load loads or creates a manifest
func Load() (*Manifest, error) {
	m := &Manifest{
		Version:  "1.0",
		Files:    make(map[string]*FileEntry),
		Channels: make(map[string]*ChannelInfo),
		path:     manifestFile,
	}

	data, err := os.ReadFile(manifestFile)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}
	m.path = manifestFile
	return m, nil
}

// Save persists the manifest to disk
func (m *Manifest) Save() error {
	m.UpdatedAt = time.Now()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.path, data, 0644)
}

// RecordReceived records a newly received file
func (m *Manifest) RecordReceived(filename string, size int64, content []byte, channelID string) {
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	// Check if this is an update to existing file
	seq := 1
	if existing, ok := m.Files[filename]; ok {
		seq = existing.Sequence + 1
	}

	m.Files[filename] = &FileEntry{
		Filename:    filename,
		ReceivedAt:  time.Now(),
		ContentHash: hashStr,
		Size:        size,
		FromChannel: channelID,
		Sequence:    seq,
		IsNew:       true,
	}
}

// MarkRead marks a file as read
func (m *Manifest) MarkRead(filename string) {
	if entry, ok := m.Files[filename]; ok {
		now := time.Now()
		entry.LastReadAt = &now
		entry.IsNew = false
	}
}

// GetUnread returns all files that haven't been read yet
func (m *Manifest) GetUnread() []*FileEntry {
	var unread []*FileEntry
	for _, entry := range m.Files {
		if entry.IsNew || entry.LastReadAt == nil {
			unread = append(unread, entry)
		}
	}
	return unread
}

// GetUpdatedSinceRead returns files that have been updated since last read
func (m *Manifest) GetUpdatedSinceRead() []*FileEntry {
	var updated []*FileEntry
	for _, entry := range m.Files {
		if entry.LastReadAt != nil && entry.ReceivedAt.After(*entry.LastReadAt) {
			updated = append(updated, entry)
		}
	}
	return updated
}

// RecordChannel records a channel
func (m *Manifest) RecordChannel(id, name, code, role string) {
	m.Channels[id] = &ChannelInfo{
		ID:          id,
		Name:        name,
		CreatedAt:   time.Now(),
		LastActivity: time.Now(),
		Code:        code,
		Role:        role,
	}
}

// UpdateChannelActivity updates the last activity time for a channel
func (m *Manifest) UpdateChannelActivity(id string) {
	if ch, ok := m.Channels[id]; ok {
		ch.LastActivity = time.Now()
		ch.MessageCount++
	}
}

// HashContent returns the SHA-256 hash of content
func HashContent(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
