// Package hooks provides Claude Code integration for claw2claw
package hooks

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/epuerta9/claw2claw/internal/client"
)

// ClaudeHookConfig is the Claude Code hooks configuration
type ClaudeHookConfig struct {
	Hooks []HookEntry `json:"hooks"`
}

// HookEntry represents a single hook configuration
type HookEntry struct {
	Matcher string   `json:"matcher"`
	Hooks   []string `json:"hooks"`
}

// RegisterHooks registers claw2claw hooks with Claude Code
func RegisterHooks() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(homeDir, ".claude", "hooks.json")

	// Read existing hooks
	var config ClaudeHookConfig
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &config)
	}

	// Add claw2claw hooks if not present
	hookFound := false
	for _, h := range config.Hooks {
		if h.Matcher == "/share" || h.Matcher == "/receive" {
			hookFound = true
			break
		}
	}

	if !hookFound {
		config.Hooks = append(config.Hooks,
			HookEntry{
				Matcher: "/share",
				Hooks:   []string{"claw share-hook"},
			},
			HookEntry{
				Matcher: "/receive",
				Hooks:   []string{"claw receive-hook"},
			},
		)

		// Write updated config
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			return err
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return err
		}
	}

	// Register UserPromptSubmit inbox hook in ~/.claude/settings.json
	if err := registerInboxHook(homeDir); err != nil {
		return err
	}

	return nil
}

// registerInboxHook adds a UserPromptSubmit hook to ~/.claude/settings.json
// that runs `c2c inbox --quiet --if-stale 30m` on every prompt.
func registerInboxHook(homeDir string) error {
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")

	// Read existing settings
	var settings map[string]interface{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			settings = make(map[string]interface{})
		}
	} else {
		settings = make(map[string]interface{})
	}

	const inboxCommand = "c2c inbox --quiet --if-stale 30m"

	// Check if our hook already exists
	if hooksRaw, ok := settings["hooks"]; ok {
		if hooksMap, ok := hooksRaw.(map[string]interface{}); ok {
			if upsRaw, ok := hooksMap["UserPromptSubmit"]; ok {
				if upsList, ok := upsRaw.([]interface{}); ok {
					for _, entry := range upsList {
						if entryMap, ok := entry.(map[string]interface{}); ok {
							if innerHooks, ok := entryMap["hooks"].([]interface{}); ok {
								for _, h := range innerHooks {
									if hMap, ok := h.(map[string]interface{}); ok {
										if cmd, ok := hMap["command"].(string); ok && cmd == inboxCommand {
											// Already registered
											return nil
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Build the hook entry
	hookEntry := map[string]interface{}{
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": inboxCommand,
			},
		},
	}

	// Merge into settings
	if settings["hooks"] == nil {
		settings["hooks"] = make(map[string]interface{})
	}
	hooksMap, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		hooksMap = make(map[string]interface{})
		settings["hooks"] = hooksMap
	}

	// Append to existing UserPromptSubmit list or create new
	if existing, ok := hooksMap["UserPromptSubmit"].([]interface{}); ok {
		hooksMap["UserPromptSubmit"] = append(existing, hookEntry)
	} else {
		hooksMap["UserPromptSubmit"] = []interface{}{hookEntry}
	}

	// Write back
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(settingsPath, data, 0644)
}

// ShareContext shares a file or content with another Claude user
func ShareContext(content []byte, filename string, relayURL string) (string, error) {
	// Generate code phrase
	codePhrase := GenerateCodePhrase()

	// Create temp file if content provided
	var filePath string
	if len(content) > 0 {
		tmpDir := os.TempDir()
		filePath = filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			return "", err
		}
		defer os.Remove(filePath)
	}

	// Create client and send
	cfg := client.DefaultConfig()
	if relayURL != "" {
		cfg.RelayURL = relayURL
	}
	c := client.New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Send in background goroutine (async for Claude hook)
	go func() {
		c.Send(ctx, filePath, codePhrase)
	}()

	return codePhrase, nil
}

// ReceiveContext receives shared content from another Claude user
func ReceiveContext(codePhrase string, outputDir string, relayURL string) (string, []byte, error) {
	cfg := client.DefaultConfig()
	if relayURL != "" {
		cfg.RelayURL = relayURL
	}
	c := client.New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Receive file
	outputPath, err := c.Receive(ctx, codePhrase, outputDir)
	if err != nil {
		return "", nil, err
	}

	// Read content
	content, err := os.ReadFile(outputPath)
	if err != nil {
		return "", nil, err
	}

	return outputPath, content, nil
}

// GenerateCodePhrase generates a memorable code phrase for sharing
func GenerateCodePhrase() string {
	adjectives := []string{
		"swift", "bright", "calm", "bold", "warm",
		"cool", "fast", "sharp", "soft", "wild",
	}
	nouns := []string{
		"tiger", "river", "mountain", "forest", "ocean",
		"castle", "dragon", "phoenix", "falcon", "storm",
	}
	colors := []string{
		"red", "blue", "green", "gold", "silver",
		"amber", "jade", "coral", "ivory", "onyx",
	}

	// Use crypto random for selection
	adj := adjectives[randomInt(len(adjectives))]
	noun := nouns[randomInt(len(nouns))]
	color := colors[randomInt(len(colors))]
	num := randomInt(100)

	return fmt.Sprintf("%s-%s-%s-%d", adj, noun, color, num)
}

func randomInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		// Fallback to time-based if crypto/rand fails (shouldn't happen)
		return int(time.Now().UnixNano() % int64(max))
	}
	return int(n.Int64())
}
