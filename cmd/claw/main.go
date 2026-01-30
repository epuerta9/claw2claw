// Package main provides the claw2claw CLI
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/epuerta9/claw2claw/internal/client"
	"github.com/epuerta9/claw2claw/internal/hooks"
	"github.com/spf13/cobra"
)

var (
	relayURL   string
	outputDir  string
	timeout    int
	ttlHours   int
	codePhrase string
	persistent bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "claw",
		Short: "Secure peer-to-peer context sharing for Claude",
		Long:  `claw2claw enables secure, end-to-end encrypted file sharing between Claude users.`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&relayURL, "relay", "", "Relay server URL")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 300, "Transfer timeout in seconds")

	// ========================
	// Send Command
	// ========================
	sendCmd := &cobra.Command{
		Use:   "send <file>",
		Short: "Send a file securely",
		Long: `Send a file securely to another user.

By default, creates an ephemeral room with a memorable code phrase.
Use --persistent to create a persistent room with a UUID (harder to guess).`,
		Args: cobra.ExactArgs(1),
		RunE: runSend,
	}
	sendCmd.Flags().BoolVarP(&persistent, "persistent", "p", false, "Create a persistent room (UUID-based, longer lived)")
	sendCmd.Flags().IntVar(&ttlHours, "ttl", 24, "TTL for persistent rooms in hours (-1 for permanent)")

	// ========================
	// Receive Command
	// ========================
	receiveCmd := &cobra.Command{
		Use:   "receive <code-or-uuid>",
		Short: "Receive a shared file",
		Long: `Receive a shared file using a code phrase or room UUID.

For ephemeral rooms: use the code phrase (e.g., swift-tiger-gold-42)
For persistent rooms: use --code flag with the UUID`,
		Args: cobra.ExactArgs(1),
		RunE: runReceive,
	}
	receiveCmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Output directory")
	receiveCmd.Flags().StringVar(&codePhrase, "code", "", "Encryption code (required for persistent rooms)")

	// ========================
	// Utility Commands
	// ========================
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install Claude Code hooks",
		RunE:  runInstall,
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("claw2claw v0.2.0")
		},
	}

	rootCmd.AddCommand(sendCmd, receiveCmd, installCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runSend(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// Check file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Generate code phrase for encryption
	code := hooks.GenerateCodePhrase()

	// Create client
	cfg := client.DefaultConfig()
	if relayURL != "" {
		cfg.RelayURL = relayURL
	}
	cfg.Timeout = time.Duration(timeout) * time.Second
	c := client.New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	if persistent {
		// Persistent room mode - uses UUID
		fmt.Printf("📤 Sharing: %s (persistent room)\n", filepath.Base(filePath))
		fmt.Printf("🔑 Encryption code: %s\n", code)

		var createdRoomID string
		onRoomCreated := func(roomID string) {
			createdRoomID = roomID
			fmt.Printf("🆔 Room ID: %s\n", roomID)
			fmt.Println("⏳ Waiting for receiver to connect...")
			fmt.Printf("\n📋 Share with receiver:\n")
			fmt.Printf("   claw receive %s --code %s\n\n", roomID, code)
		}

		err := c.SendPersistentWithCallback(ctx, filePath, code, ttlHours, onRoomCreated)
		if err != nil {
			return fmt.Errorf("transfer failed: %w", err)
		}

		fmt.Println("✅ Transfer complete!")
		_ = createdRoomID // Used in callback output
	} else {
		// Ephemeral room mode - uses code phrase
		fmt.Printf("📤 Sharing: %s\n", filepath.Base(filePath))
		fmt.Printf("🔑 Share code: %s\n", code)
		fmt.Println("⏳ Waiting for receiver to connect...")

		if err := c.Send(ctx, filePath, code); err != nil {
			return fmt.Errorf("transfer failed: %w", err)
		}

		fmt.Println("✅ Transfer complete!")
	}

	return nil
}

func runReceive(cmd *cobra.Command, args []string) error {
	identifier := args[0]

	// Create client
	cfg := client.DefaultConfig()
	if relayURL != "" {
		cfg.RelayURL = relayURL
	}
	cfg.Timeout = time.Duration(timeout) * time.Second
	c := client.New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	var receivedPath string
	var err error

	// Detect if it's a UUID (persistent room) or code phrase (ephemeral)
	if codePhrase != "" {
		// Persistent room mode - identifier is UUID, code is for encryption
		fmt.Printf("📥 Connecting to room: %s\n", identifier)
		fmt.Println("⏳ Waiting for sender...")

		receivedPath, err = c.ReceivePersistent(ctx, identifier, codePhrase, outputDir)
	} else {
		// Ephemeral room mode - identifier is the code phrase
		fmt.Printf("📥 Connecting with code: %s\n", identifier)
		fmt.Println("⏳ Waiting for sender...")

		receivedPath, err = c.Receive(ctx, identifier, outputDir)
	}

	if err != nil {
		return fmt.Errorf("receive failed: %w", err)
	}

	fmt.Printf("✅ Received: %s\n", receivedPath)
	return nil
}

func runInstall(cmd *cobra.Command, args []string) error {
	fmt.Println("📦 Installing Claude Code hooks...")

	if err := hooks.RegisterHooks(); err != nil {
		return fmt.Errorf("failed to install hooks: %w", err)
	}

	fmt.Println("✅ Hooks installed!")
	fmt.Println("\nYou can now use:")
	fmt.Println("  /share <file>   - Share a file")
	fmt.Println("  /receive <code> - Receive a shared file")
	return nil
}
