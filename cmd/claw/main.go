// Package main provides the claw2claw CLI
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/epuerta9/claw2claw/internal/account"
	"github.com/epuerta9/claw2claw/internal/client"
	"github.com/epuerta9/claw2claw/internal/hooks"
	"github.com/epuerta9/claw2claw/internal/manifest"
	"github.com/epuerta9/claw2claw/internal/safereader"
	"github.com/spf13/cobra"
)

var (
	relayURL    string
	outputDir   string
	timeout     int
	ttlHours    int
	codePhrase  string
	persistent  bool
	rawOutput   bool   // For read command - skip safety wrapper
	channelName string // For channel create
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

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List received files",
		Long:  `List all files received via claw2claw in the .claw/received/ directory.`,
		RunE:  runList,
	}

	// ========================
	// Read Command (Safe Reading)
	// ========================
	readCmd := &cobra.Command{
		Use:   "read <filename>",
		Short: "Safely read a received file with prompt injection protection",
		Long: `Read a received file with safety markers that protect against prompt injection.

The content is wrapped with clear boundaries indicating it's external/untrusted data.
Suspicious patterns (instruction overrides, role manipulation) are detected and warned.`,
		Args: cobra.ExactArgs(1),
		RunE: runRead,
	}
	readCmd.Flags().BoolVar(&rawOutput, "raw", false, "Output raw content without safety wrapper (use with caution)")

	// ========================
	// New Command (What's New)
	// ========================
	newCmd := &cobra.Command{
		Use:   "new",
		Short: "Show files received since last read",
		Long:  `Show all files that have been received but not yet read, or updated since last read.`,
		RunE:  runNew,
	}

	// ========================
	// Channel Commands (Bidirectional)
	// ========================
	channelCmd := &cobra.Command{
		Use:   "channel",
		Short: "Manage bidirectional channels for ongoing context sharing",
		Long: `Channels allow two Claude instances to share context back and forth.

Unlike one-time transfers, channels persist and both parties can send messages.`,
	}

	channelCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new channel",
		RunE:  runChannelCreate,
	}
	channelCreateCmd.Flags().StringVarP(&channelName, "name", "n", "", "Optional name for the channel")

	channelJoinCmd := &cobra.Command{
		Use:   "join <channel-id> --code <code>",
		Short: "Join an existing channel",
		Args:  cobra.ExactArgs(1),
		RunE:  runChannelJoin,
	}
	channelJoinCmd.Flags().StringVar(&codePhrase, "code", "", "Channel encryption code (required)")
	channelJoinCmd.MarkFlagRequired("code")

	channelSendCmd := &cobra.Command{
		Use:   "send <channel-id> <file>",
		Short: "Send a file to a channel",
		Args:  cobra.ExactArgs(2),
		RunE:  runChannelSend,
	}

	channelListCmd := &cobra.Command{
		Use:   "list",
		Short: "List your channels",
		RunE:  runChannelList,
	}

	channelCmd.AddCommand(channelCreateCmd, channelJoinCmd, channelSendCmd, channelListCmd)

	// ========================
	// Account Commands (Optional - for sync/share)
	// ========================
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Login to sync sessions (optional)",
		Long: `Login to your claw2claw account to sync and share sessions.

This is OPTIONAL. Basic send/receive works without an account.
Account features include:
  - Session history
  - Shareable links
  - Web UI access`,
		RunE: runLogin,
	}

	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout from your account",
		RunE:  runLogout,
	}

	sessionsCmd := &cobra.Command{
		Use:   "sessions",
		Short: "List your synced sessions",
		RunE:  runSessions,
	}

	openCmd := &cobra.Command{
		Use:   "open [session-id]",
		Short: "Open session or dashboard in browser",
		Long: `Open a session in your browser, or your dashboard if no session specified.

Examples:
  claw open              # Open dashboard
  claw open abc123       # Open specific session`,
		RunE: runOpen,
	}

	whoamiCmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show current logged in user",
		RunE:  runWhoami,
	}

	rootCmd.AddCommand(sendCmd, receiveCmd, installCmd, versionCmd, listCmd, readCmd, newCmd, channelCmd)
	rootCmd.AddCommand(loginCmd, logoutCmd, sessionsCmd, openCmd, whoamiCmd)

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

	// Resolve output directory - default to .claw/received/ if not specified
	outDir := outputDir
	if outDir == "." {
		// Try to use .claw/received/ in current directory
		clawDir := ".claw/received"
		if err := os.MkdirAll(clawDir, 0755); err == nil {
			outDir = clawDir
		}
	}

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

		receivedPath, err = c.ReceivePersistent(ctx, identifier, codePhrase, outDir)
	} else {
		// Ephemeral room mode - identifier is the code phrase
		fmt.Printf("📥 Connecting with code: %s\n", identifier)
		fmt.Println("⏳ Waiting for sender...")

		receivedPath, err = c.Receive(ctx, identifier, outDir)
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

func runList(cmd *cobra.Command, args []string) error {
	clawDir := ".claw/received"

	// Check if directory exists
	if _, err := os.Stat(clawDir); os.IsNotExist(err) {
		fmt.Println("📭 No received files yet.")
		fmt.Println("   Directory .claw/received/ does not exist.")
		return nil
	}

	// List files
	entries, err := os.ReadDir(clawDir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("📭 No received files yet.")
		return nil
	}

	fmt.Println("📥 Received files in .claw/received/:")
	fmt.Println()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		// Format: filename (size) - modified time
		size := info.Size()
		mod := info.ModTime().Format("2006-01-02 15:04")
		fmt.Printf("  📄 %-30s %8d bytes  %s\n", entry.Name(), size, mod)
	}
	fmt.Println()
	fmt.Println("Read safely with: claw read <filename>")
	return nil
}

func runRead(cmd *cobra.Command, args []string) error {
	filename := args[0]
	clawDir := ".claw/received"

	// Build full path
	filePath := filepath.Join(clawDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Try as absolute/relative path
		filePath = filename
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", filename)
		}
	}

	if rawOutput {
		// Just cat the file
		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		fmt.Print(string(content))
		return nil
	}

	// Safe read with prompt injection protection
	sc, err := safereader.ReadSafe(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Output the wrapped content
	fmt.Print(sc.Content)

	// Update manifest to mark as read
	m, err := manifest.Load()
	if err == nil {
		m.MarkRead(sc.Filename)
		m.Save()
	}

	return nil
}

func runNew(cmd *cobra.Command, args []string) error {
	clawDir := ".claw/received"

	// Check if directory exists
	if _, err := os.Stat(clawDir); os.IsNotExist(err) {
		fmt.Println("📭 No received files yet.")
		return nil
	}

	// Load manifest
	m, err := manifest.Load()
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Get unread and updated files
	unread := m.GetUnread()
	updated := m.GetUpdatedSinceRead()

	if len(unread) == 0 && len(updated) == 0 {
		// Check for files not in manifest (first time)
		entries, err := os.ReadDir(clawDir)
		if err != nil {
			return err
		}

		var newFiles []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if _, exists := m.Files[entry.Name()]; !exists {
				newFiles = append(newFiles, entry.Name())
				// Add to manifest
				info, _ := entry.Info()
				content, _ := os.ReadFile(filepath.Join(clawDir, entry.Name()))
				m.RecordReceived(entry.Name(), info.Size(), content, "")
			}
		}

		if len(newFiles) == 0 {
			fmt.Println("✅ All caught up! No new files since last read.")
			return nil
		}

		m.Save()

		fmt.Println("🆕 New files (never read):")
		for _, f := range newFiles {
			fmt.Printf("   📄 %s\n", f)
		}
		fmt.Println()
		fmt.Println("Read safely with: claw read <filename>")
		return nil
	}

	if len(unread) > 0 {
		fmt.Println("🆕 Unread files:")
		for _, entry := range unread {
			fmt.Printf("   📄 %s (received %s)\n", entry.Filename, entry.ReceivedAt.Format("2006-01-02 15:04"))
		}
	}

	if len(updated) > 0 {
		fmt.Println("\n🔄 Updated since last read:")
		for _, entry := range updated {
			fmt.Printf("   📄 %s (updated %s, v%d)\n", entry.Filename, entry.ReceivedAt.Format("2006-01-02 15:04"), entry.Sequence)
		}
	}

	fmt.Println()
	fmt.Println("Read safely with: claw read <filename>")
	return nil
}

func runChannelCreate(cmd *cobra.Command, args []string) error {
	// Generate encryption code
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

	fmt.Println("📡 Creating bidirectional channel...")

	var channelID string
	onRoomCreated := func(roomID string) {
		channelID = roomID
	}

	// Create as persistent room with long TTL
	// We'll use a dummy file for now - channels will be enhanced later
	tmpFile := filepath.Join(os.TempDir(), "channel-init.txt")
	os.WriteFile(tmpFile, []byte("Channel initialized"), 0644)
	defer os.Remove(tmpFile)

	// Save channel info to manifest before waiting
	m, _ := manifest.Load()
	m.RecordChannel(channelID, channelName, code, "creator")
	m.Save()

	fmt.Printf("🔑 Channel code: %s\n", code)

	err := c.SendPersistentWithCallback(ctx, tmpFile, code, 168, onRoomCreated) // 1 week TTL
	if err != nil {
		return fmt.Errorf("channel creation failed: %w", err)
	}

	fmt.Printf("\n✅ Channel created!\n")
	fmt.Printf("🆔 Channel ID: %s\n", channelID)
	fmt.Printf("🔑 Code: %s\n", code)
	fmt.Printf("\n📋 Share with collaborator:\n")
	fmt.Printf("   claw channel join %s --code %s\n", channelID, code)

	return nil
}

func runChannelJoin(cmd *cobra.Command, args []string) error {
	channelID := args[0]

	// Create output dir for channel
	channelDir := filepath.Join(".claw", "channels", channelID)
	if err := os.MkdirAll(channelDir, 0755); err != nil {
		return err
	}

	// Create client
	cfg := client.DefaultConfig()
	if relayURL != "" {
		cfg.RelayURL = relayURL
	}
	cfg.Timeout = time.Duration(timeout) * time.Second
	c := client.New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	fmt.Printf("📡 Joining channel: %s\n", channelID)

	receivedPath, err := c.ReceivePersistent(ctx, channelID, codePhrase, channelDir)
	if err != nil {
		return fmt.Errorf("failed to join channel: %w", err)
	}

	// Save channel info
	m, _ := manifest.Load()
	m.RecordChannel(channelID, "", codePhrase, "joiner")
	m.Save()

	fmt.Printf("✅ Joined channel!\n")
	fmt.Printf("📥 Received: %s\n", receivedPath)
	fmt.Printf("\n📤 Send to this channel:\n")
	fmt.Printf("   claw channel send %s <file>\n", channelID)

	return nil
}

func runChannelSend(cmd *cobra.Command, args []string) error {
	channelID := args[0]
	filePath := args[1]

	// Load manifest to get channel code
	m, err := manifest.Load()
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	ch, exists := m.Channels[channelID]
	if !exists {
		return fmt.Errorf("channel not found: %s\nJoin it first with: claw channel join %s --code <code>", channelID, channelID)
	}

	// Create client
	cfg := client.DefaultConfig()
	if relayURL != "" {
		cfg.RelayURL = relayURL
	}
	cfg.Timeout = time.Duration(timeout) * time.Second
	c := client.New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	fmt.Printf("📤 Sending to channel: %s\n", channelID)
	fmt.Println("⏳ Waiting for receiver...")

	err = c.SendPersistentWithCallback(ctx, filePath, ch.Code, 168, func(roomID string) {
		// Channel uses same ID
	})
	if err != nil {
		return fmt.Errorf("send failed: %w", err)
	}

	// Update activity
	m.UpdateChannelActivity(channelID)
	m.Save()

	fmt.Println("✅ Sent!")
	return nil
}

func runChannelList(cmd *cobra.Command, args []string) error {
	m, err := manifest.Load()
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	if len(m.Channels) == 0 {
		fmt.Println("📭 No channels yet.")
		fmt.Println("\nCreate one with: claw channel create")
		fmt.Println("Or join one with: claw channel join <id> --code <code>")
		return nil
	}

	fmt.Println("📡 Your channels:")
	fmt.Println()

	// Sort by last activity
	var channels []*manifest.ChannelInfo
	for _, ch := range m.Channels {
		channels = append(channels, ch)
	}
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].LastActivity.After(channels[j].LastActivity)
	})

	for _, ch := range channels {
		roleIcon := "👤"
		if ch.Role == "creator" {
			roleIcon = "👑"
		}
		name := ch.ID[:8] + "..."
		if ch.Name != "" {
			name = ch.Name
		}
		fmt.Printf("  %s %-20s  %d msgs  last: %s\n",
			roleIcon, name, ch.MessageCount, ch.LastActivity.Format("2006-01-02 15:04"))
		fmt.Printf("     ID: %s\n", ch.ID)
	}

	return nil
}

// ========================
// Account Commands
// ========================

func runLogin(cmd *cobra.Command, args []string) error {
	// Load existing config or create new
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.LoggedIn {
		fmt.Printf("✅ Already logged in as %s (%s)\n", cfg.Name, cfg.Email)
		fmt.Println("   Use 'claw logout' to switch accounts.")
		return nil
	}

	// Start device auth flow
	newCfg, err := account.Login(cfg.BaseURL)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Save config
	if err := account.SaveConfig(newCfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if newCfg.Name != "" {
		fmt.Printf("👤 Welcome, %s!\n", newCfg.Name)
	}
	fmt.Println("\n🎉 You can now sync and share sessions.")
	fmt.Println("   View your dashboard: claw open")
	fmt.Println("   List sessions: claw sessions")

	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.LoggedIn {
		fmt.Println("ℹ️  Not currently logged in.")
		return nil
	}

	// Clear credentials but keep baseURL
	cfg.Token = ""
	cfg.Email = ""
	cfg.Name = ""
	cfg.LoggedIn = false

	if err := account.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("👋 Logged out successfully.")
	fmt.Println("   Basic send/receive still works without an account.")
	return nil
}

func runSessions(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.LoggedIn {
		fmt.Println("ℹ️  Not logged in. Sessions are stored locally only.")
		fmt.Println("   Login to sync: claw login")
		return nil
	}

	sessions, err := account.ListSessions(cfg)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("📭 No sessions yet.")
		fmt.Println("   Send files to create sessions.")
		return nil
	}

	fmt.Println("📚 Your sessions:")
	fmt.Println()

	for _, s := range sessions {
		visIcon := "🔒"
		if s.Visibility == "public" {
			visIcon = "🌐"
		} else if s.Visibility == "unlisted" {
			visIcon = "🔗"
		}

		title := s.Title
		if title == "" {
			title = s.ID[:8] + "..."
		}

		fmt.Printf("  %s %-30s  %d msgs  %s\n",
			visIcon, title, s.MessageCount, s.CreatedAt.Format("2006-01-02"))
		fmt.Printf("     ID: %s\n", s.ID)
	}

	fmt.Println()
	fmt.Println("Open in browser: claw open <session-id>")
	return nil
}

func runOpen(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.LoggedIn {
		fmt.Println("ℹ️  Not logged in.")
		fmt.Println("   Login first: claw login")
		return nil
	}

	if len(args) == 0 {
		// Open dashboard
		fmt.Println("🌐 Opening dashboard...")
		return account.OpenDashboard(cfg)
	}

	// Open specific session
	sessionID := args[0]
	fmt.Printf("🌐 Opening session %s...\n", sessionID)
	return account.OpenSession(cfg, sessionID)
}

func runWhoami(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.LoggedIn {
		fmt.Println("👤 Not logged in")
		fmt.Println("   Login: claw login")
		fmt.Println("   (Basic send/receive works without an account)")
		return nil
	}

	fmt.Println("👤 Account:")
	if cfg.Name != "" {
		fmt.Printf("   Name:  %s\n", cfg.Name)
	}
	if cfg.Email != "" {
		fmt.Printf("   Email: %s\n", cfg.Email)
	}
	fmt.Printf("   Server: %s\n", cfg.BaseURL)

	return nil
}
