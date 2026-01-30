// Package hooks provides Claude Code integration for claw2claw
package hooks

import (
	"context"
	"encoding/json"
	"fmt"
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

		return os.WriteFile(configPath, data, 0644)
	}

	return nil
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
	// Simple pseudo-random for code phrase generation
	// In production, use crypto/rand
	return int(time.Now().UnixNano() % int64(max))
}
