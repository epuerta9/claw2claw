// Package account provides board and notification API client functions
package account

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BoardSection represents a section of the shared team board
type BoardSection struct {
	ID        string    `json:"id"`
	TeamID    string    `json:"team_id"`
	Section   string    `json:"section"`
	Content   string    `json:"content"`
	UpdatedBy string    `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int       `json:"version"`
}

// Notification represents a notification
type Notification struct {
	ID         string     `json:"id"`
	TeamID     string     `json:"team_id"`
	FromUser   string     `json:"from_user"`
	ToUser     string     `json:"to_user"`
	Type       string     `json:"type"`
	Subject    string     `json:"subject"`
	Body       string     `json:"body,omitempty"`
	RefSection string     `json:"ref_section,omitempty"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// SharedFile represents a shared file
type SharedFile struct {
	ID         string    `json:"id"`
	TeamID     string    `json:"team_id"`
	UploadedBy string    `json:"uploaded_by"`
	Filename   string    `json:"filename"`
	Size       int64     `json:"size"`
	CreatedAt  time.Time `json:"created_at"`
}

// InboxSummary is the response from the inbox endpoint
type InboxSummary struct {
	UnreadCount   int            `json:"unread_count"`
	Notifications []Notification `json:"notifications,omitempty"`
	BoardChanges  []BoardSection `json:"board_changes,omitempty"`
}

// GetBoard fetches the full board for a team
func GetBoard(cfg *Config) ([]BoardSection, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return nil, fmt.Errorf("not logged in or team not configured")
	}

	req, _ := http.NewRequest("GET", cfg.BaseURL+"/api/v1/board/"+cfg.TeamID, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get board: %d", resp.StatusCode)
	}

	var result struct {
		Sections []BoardSection `json:"sections"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Sections, nil
}

// GetBoardSection fetches a single board section
func GetBoardSection(cfg *Config, section string) (*BoardSection, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return nil, fmt.Errorf("not logged in or team not configured")
	}

	req, _ := http.NewRequest("GET", cfg.BaseURL+"/api/v1/board/"+cfg.TeamID+"/"+section, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get section: %d", resp.StatusCode)
	}

	var bs BoardSection
	if err := json.NewDecoder(resp.Body).Decode(&bs); err != nil {
		return nil, err
	}
	return &bs, nil
}

// UpdateBoardSection updates a board section
func UpdateBoardSection(cfg *Config, section, content string) (*BoardSection, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return nil, fmt.Errorf("not logged in or team not configured")
	}

	// First get current version for optimistic locking
	existing, err := GetBoardSection(cfg, section)
	if err != nil {
		return nil, err
	}
	version := 0
	if existing != nil {
		version = existing.Version
	}

	body := fmt.Sprintf(`{"content":%q,"version":%d}`, content, version)
	req, _ := http.NewRequest("PUT", cfg.BaseURL+"/api/v1/board/"+cfg.TeamID+"/"+section,
		strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to update section: %d %s", resp.StatusCode, string(respBody))
	}

	var bs BoardSection
	if err := json.NewDecoder(resp.Body).Decode(&bs); err != nil {
		return nil, err
	}
	return &bs, nil
}

// InitBoard initializes the board with default sections
func InitBoard(cfg *Config, members []string) error {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured")
	}

	membersJSON, _ := json.Marshal(members)
	body := fmt.Sprintf(`{"members":%s}`, string(membersJSON))

	req, _ := http.NewRequest("POST", cfg.BaseURL+"/api/v1/board/"+cfg.TeamID+"/init",
		strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to initialize board: %d", resp.StatusCode)
	}
	return nil
}

// SendNotification creates a notification
func SendNotification(cfg *Config, toUser, notifType, subject, body string) (*Notification, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return nil, fmt.Errorf("not logged in or team not configured")
	}

	fromUser := cfg.UserID
	if fromUser == "" {
		fromUser = cfg.Name
		if fromUser == "" {
			fromUser = cfg.Email
		}
	}

	reqBody := fmt.Sprintf(`{"from_user":%q,"to_user":%q,"type":%q,"subject":%q,"body":%q}`,
		fromUser, toUser, notifType, subject, body)

	req, _ := http.NewRequest("POST", cfg.BaseURL+"/api/v1/notifications/"+cfg.TeamID,
		strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create notification: %d", resp.StatusCode)
	}

	var n Notification
	if err := json.NewDecoder(resp.Body).Decode(&n); err != nil {
		return nil, err
	}
	return &n, nil
}

// GetNotifications fetches notifications for a user
func GetNotifications(cfg *Config, userID string, unreadOnly bool) ([]Notification, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return nil, fmt.Errorf("not logged in or team not configured")
	}

	url := cfg.BaseURL + "/api/v1/notifications/" + cfg.TeamID + "/" + userID
	if unreadOnly {
		url += "?unread=true"
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Notifications []Notification `json:"notifications"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Notifications, nil
}

// MarkNotificationRead marks a notification as read
func MarkNotificationRead(cfg *Config, notificationID string) error {
	req, _ := http.NewRequest("PATCH", cfg.BaseURL+"/api/v1/notifications/"+notificationID+"/read", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to mark read: %d", resp.StatusCode)
	}
	return nil
}

// GetInbox fetches the inbox summary for session-start check
func GetInbox(cfg *Config) (*InboxSummary, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return nil, fmt.Errorf("not logged in or team not configured")
	}

	userID := cfg.UserID
	if userID == "" {
		userID = cfg.Name
		if userID == "" {
			userID = cfg.Email
		}
	}

	url := cfg.BaseURL + "/api/v1/inbox/" + cfg.TeamID + "/" + userID
	if cfg.LastBoardCheck != "" {
		url += "?since=" + cfg.LastBoardCheck
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get inbox: %d", resp.StatusCode)
	}

	var inbox InboxSummary
	if err := json.NewDecoder(resp.Body).Decode(&inbox); err != nil {
		return nil, err
	}

	// Update last check timestamp
	cfg.LastBoardCheck = time.Now().Format(time.RFC3339)
	SaveConfig(cfg)

	return &inbox, nil
}

// UploadFile uploads a file to the team board
func UploadFile(cfg *Config, filePath string) (*SharedFile, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return nil, fmt.Errorf("not logged in or team not configured")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	writer.Close()

	req, _ := http.NewRequest("POST", cfg.BaseURL+"/api/v1/files/"+cfg.TeamID+"/upload", &buf)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload failed: %d", resp.StatusCode)
	}

	var sf SharedFile
	if err := json.NewDecoder(resp.Body).Decode(&sf); err != nil {
		return nil, err
	}
	return &sf, nil
}

// ListFiles lists shared files for the team
func ListFiles(cfg *Config) ([]SharedFile, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return nil, fmt.Errorf("not logged in or team not configured")
	}

	req, _ := http.NewRequest("GET", cfg.BaseURL+"/api/v1/files/"+cfg.TeamID, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Files []SharedFile `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Files, nil
}

// DownloadFile downloads a shared file to the specified directory
func DownloadFile(cfg *Config, fileID, outputDir string) (string, error) {
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return "", fmt.Errorf("not logged in or team not configured")
	}

	req, _ := http.NewRequest("GET", cfg.BaseURL+"/api/v1/files/"+cfg.TeamID+"/"+fileID, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	// Get filename from Content-Disposition header
	filename := "downloaded_file"
	cd := resp.Header.Get("Content-Disposition")
	if strings.Contains(cd, "filename=") {
		parts := strings.SplitN(cd, "filename=", 2)
		if len(parts) == 2 {
			filename = strings.Trim(parts[1], "\"")
		}
	}

	os.MkdirAll(outputDir, 0755)
	outPath := filepath.Join(outputDir, filename)

	out, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}

	return outPath, nil
}
