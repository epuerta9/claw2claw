// Package account provides account management for claw2claw
package account

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Config holds account configuration
type Config struct {
	Token    string `json:"token"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	LoggedIn bool   `json:"logged_in"`
}

const configFileName = ".claw/account.json"

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configFileName)
}

// LoadConfig loads the account configuration
func LoadConfig() (*Config, error) {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{BaseURL: "https://claw2claw-relay.fly.dev"}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveConfig saves the account configuration
func SaveConfig(cfg *Config) error {
	path := GetConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// DeviceAuthResponse from the API
type DeviceAuthResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse from the API
type TokenResponse struct {
	Status      string `json:"status"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// Login performs the device auth flow
func Login(baseURL string) (*Config, error) {
	// Step 1: Request device code
	resp, err := http.Post(baseURL+"/api/v1/auth/device", "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start auth: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth request failed: %d", resp.StatusCode)
	}

	var authResp DeviceAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, err
	}

	// Step 2: Show user code and open browser
	verifyURL := baseURL + authResp.VerificationURI
	fmt.Println("\n🔐 Opening browser for authentication...")
	fmt.Printf("   %s\n\n", verifyURL)
	fmt.Printf("   Enter this code: %s\n\n", authResp.UserCode)

	openBrowser(verifyURL)

	// Step 3: Poll for authorization
	fmt.Print("⏳ Waiting for authorization")
	ticker := time.NewTicker(time.Duration(authResp.Interval) * time.Second)
	defer ticker.Stop()

	timeout := time.After(time.Duration(authResp.ExpiresIn) * time.Second)

	for {
		select {
		case <-timeout:
			fmt.Println("\n❌ Authorization timed out")
			return nil, fmt.Errorf("authorization timed out")
		case <-ticker.C:
			fmt.Print(".")

			tokenResp, err := pollForToken(baseURL, authResp.DeviceCode)
			if err != nil {
				continue // Keep polling
			}

			if tokenResp.Status == "authorized" {
				fmt.Println("\n✅ Logged in successfully!")

				cfg := &Config{
					Token:    tokenResp.AccessToken,
					BaseURL:  baseURL,
					LoggedIn: true,
				}

				// Fetch user info
				if user, err := fetchUser(baseURL, tokenResp.AccessToken); err == nil {
					cfg.Email = user.Email
					cfg.Name = user.Name
				}

				return cfg, nil
			}
		}
	}
}

func pollForToken(baseURL, deviceCode string) (*TokenResponse, error) {
	body := fmt.Sprintf(`{"device_code":"%s"}`, deviceCode)
	resp, err := http.Post(baseURL+"/api/v1/auth/device/poll", "application/json",
		bufio.NewReader(stringReader(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

type stringReader string

func (s stringReader) Read(p []byte) (n int, err error) {
	return copy(p, s), nil
}

type UserInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func fetchUser(baseURL, token string) (*UserInfo, error) {
	req, _ := http.NewRequest("GET", baseURL+"/api/v1/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

// Session represents a session from the API
type Session struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Visibility   string    `json:"visibility"`
	MessageCount int       `json:"message_count"`
	TotalBytes   int64     `json:"total_bytes"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ListSessions fetches sessions from the API
func ListSessions(cfg *Config) ([]Session, error) {
	if !cfg.LoggedIn {
		return nil, fmt.Errorf("not logged in")
	}

	req, _ := http.NewRequest("GET", cfg.BaseURL+"/api/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Sessions []Session `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Sessions, nil
}

// openBrowser opens the default browser
func openBrowser(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	exec.Command(cmd, args...).Start()
}

// OpenSession opens a session in the browser
func OpenSession(cfg *Config, sessionID string) error {
	url := cfg.BaseURL + "/sessions/" + sessionID
	openBrowser(url)
	return nil
}

// OpenDashboard opens the dashboard in the browser
func OpenDashboard(cfg *Config) error {
	url := cfg.BaseURL + "/dashboard"
	openBrowser(url)
	return nil
}

// CreateSession creates a new session via the API
func CreateSession(cfg *Config, title, roomID string) (*Session, error) {
	if !cfg.LoggedIn {
		return nil, fmt.Errorf("not logged in")
	}

	body := fmt.Sprintf(`{"title":%q,"room_id":%q,"visibility":"private"}`, title, roomID)

	req, _ := http.NewRequest("POST", cfg.BaseURL+"/api/v1/sessions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create session: %d", resp.StatusCode)
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}

	return &session, nil
}

// FindOrCreateSession finds existing session by room_id or creates a new one
func FindOrCreateSession(cfg *Config, title, roomID string) (*Session, bool, error) {
	if !cfg.LoggedIn {
		return nil, false, fmt.Errorf("not logged in")
	}

	body := fmt.Sprintf(`{"title":%q,"room_id":%q}`, title, roomID)

	req, _ := http.NewRequest("POST", cfg.BaseURL+"/api/v1/sessions/find-or-create", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("failed to find/create session: %d", resp.StatusCode)
	}

	var result struct {
		Session *Session `json:"session"`
		Created bool     `json:"created"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, err
	}

	return result.Session, result.Created, nil
}

// AddMessage adds a message to a session (preview mode - limited content)
func AddMessage(cfg *Config, sessionID, direction, filename string, fileSize int64, preview string) error {
	return AddMessageWithContent(cfg, sessionID, direction, filename, fileSize, preview, "", "preview")
}

// AddMessageWithContent adds a message with full content support
// contentMode: "none" (metadata only), "preview" (truncated), "full" (complete content)
func AddMessageWithContent(cfg *Config, sessionID, direction, filename string, fileSize int64, preview, content, contentMode string) error {
	if !cfg.LoggedIn {
		return fmt.Errorf("not logged in")
	}

	body := fmt.Sprintf(`{"direction":%q,"filename":%q,"file_size":%d,"preview":%q,"content":%q,"content_mode":%q}`,
		direction, filename, fileSize, preview, content, contentMode)

	req, _ := http.NewRequest("POST", cfg.BaseURL+"/api/v1/sessions/"+sessionID+"/messages",
		strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to add message: %d", resp.StatusCode)
	}

	return nil
}

// SessionContext represents session content for Claude to re-read
type SessionContext struct {
	Session  *Session  `json:"session"`
	Context  string    `json:"context"`  // Formatted markdown context
	Messages []Message `json:"messages"`
}

// Message represents a message in a session
type Message struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	Direction   string    `json:"direction"`
	Filename    string    `json:"filename"`
	FileSize    int64     `json:"file_size"`
	Preview     string    `json:"preview,omitempty"`
	Content     string    `json:"content,omitempty"`
	ContentMode string    `json:"content_mode"`
	CreatedAt   time.Time `json:"created_at"`
}

// GetSessionContext retrieves full session content for Claude to re-read
func GetSessionContext(cfg *Config, sessionID string) (*SessionContext, error) {
	if !cfg.LoggedIn {
		return nil, fmt.Errorf("not logged in")
	}

	req, _ := http.NewRequest("GET", cfg.BaseURL+"/api/v1/sessions/"+sessionID+"/context", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get session context: %d", resp.StatusCode)
	}

	var ctx SessionContext
	if err := json.NewDecoder(resp.Body).Decode(&ctx); err != nil {
		return nil, err
	}

	return &ctx, nil
}
