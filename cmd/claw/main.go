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
	relayURL  string
	outputDir string
	timeout   int
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "claw",
		Short: "Secure peer-to-peer context sharing for Claude",
		Long:  `claw2claw enables secure, end-to-end encrypted file sharing between Claude users.`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&relayURL, "relay", "", "Relay server URL (default: wss://relay.claw2claw.io)")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 300, "Transfer timeout in seconds")

	// Send command
	sendCmd := &cobra.Command{
		Use:   "send <file>",
		Short: "Send a file securely",
		Args:  cobra.ExactArgs(1),
		RunE:  runSend,
	}

	// Receive command
	receiveCmd := &cobra.Command{
		Use:   "receive <code>",
		Short: "Receive a shared file",
		Args:  cobra.ExactArgs(1),
		RunE:  runReceive,
	}
	receiveCmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Output directory")

	// Install command (Claude Code hooks)
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install Claude Code hooks",
		RunE:  runInstall,
	}

	// Version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("claw2claw v0.1.0")
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

	// Generate code phrase
	codePhrase := hooks.GenerateCodePhrase()

	fmt.Printf("📤 Sharing: %s\n", filepath.Base(filePath))
	fmt.Printf("🔑 Share code: %s\n", codePhrase)
	fmt.Println("⏳ Waiting for receiver to connect...")

	// Create client
	cfg := client.DefaultConfig()
	if relayURL != "" {
		cfg.RelayURL = relayURL
	}
	cfg.Timeout = time.Duration(timeout) * time.Second
	c := client.New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	if err := c.Send(ctx, filePath, codePhrase); err != nil {
		return fmt.Errorf("transfer failed: %w", err)
	}

	fmt.Println("✅ Transfer complete!")
	return nil
}

func runReceive(cmd *cobra.Command, args []string) error {
	codePhrase := args[0]

	fmt.Printf("📥 Connecting with code: %s\n", codePhrase)
	fmt.Println("⏳ Waiting for sender...")

	// Create client
	cfg := client.DefaultConfig()
	if relayURL != "" {
		cfg.RelayURL = relayURL
	}
	cfg.Timeout = time.Duration(timeout) * time.Second
	c := client.New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	receivedPath, err := c.Receive(ctx, codePhrase, outputDir)
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
	fmt.Println("  /share <file>  - Share a file")
	fmt.Println("  /receive <code> - Receive a shared file")
	return nil
}
