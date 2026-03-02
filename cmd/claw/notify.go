package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/epuerta9/claw2claw/internal/account"
	"github.com/spf13/cobra"
)

var (
	notifyType string
	jsonOutput bool
	quietMode  bool
	ifStale    string
)

func newNotifyCmd() *cobra.Command {
	notifyCmd := &cobra.Command{
		Use:   "notify <user> <subject> [body]",
		Short: "Send a notification to a team member",
		Long: `Send a notification to a team member's agent.

Notification types: question, blocker, mention, file

Examples:
  c2c notify jared "Caching decision needed" "Should we use KV or Redis?"
  c2c notify jared "API review" --type blocker`,
		Args: cobra.RangeArgs(2, 3),
		RunE: runNotify,
	}
	notifyCmd.Flags().StringVar(&notifyType, "type", "question", "Notification type: question, blocker, mention, file")

	return notifyCmd
}

func newInboxCmd() *cobra.Command {
	inboxCmd := &cobra.Command{
		Use:   "inbox",
		Short: "Check unread notifications and board changes",
		Long: `Check your inbox for unread notifications and recent board changes.

This is the session-start command - run it to see what's happened since you last checked.`,
		RunE: runInbox,
	}
	inboxCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON (for hooks)")
	inboxCmd.Flags().BoolVar(&quietMode, "quiet", false, "Suppress non-essential output")
	inboxCmd.Flags().StringVar(&ifStale, "if-stale", "", "Only check if last check was longer ago than this duration (e.g. 30m, 1h)")


	readCmd := &cobra.Command{
		Use:   "read <notification-id>",
		Short: "Read and mark a notification as read",
		Args:  cobra.ExactArgs(1),
		RunE:  runInboxRead,
	}

	replyCmd := &cobra.Command{
		Use:   "reply <notification-id> <body>",
		Short: "Reply to a notification",
		Args:  cobra.ExactArgs(2),
		RunE:  runInboxReply,
	}

	inboxCmd.AddCommand(readCmd, replyCmd)
	return inboxCmd
}

func runNotify(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured")
	}

	toUser := args[0]
	subject := args[1]
	body := ""
	if len(args) > 2 {
		body = args[2]
	}

	n, err := account.SendNotification(cfg, toUser, notifyType, subject, body)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	fmt.Printf("Sent %s to %s: %s\n", n.Type, n.ToUser, n.Subject)
	return nil
}

func runInbox(cmd *cobra.Command, args []string) error {
	// If --if-stale is set, check whether we need to run at all
	if ifStale != "" {
		staleDuration, err := time.ParseDuration(ifStale)
		if err != nil {
			return fmt.Errorf("invalid --if-stale duration %q: %w", ifStale, err)
		}

		cfg, err := account.LoadConfig()
		if err != nil {
			// No config yet → treat as stale, proceed with normal check
		} else if cfg.LastBoardCheck != "" {
			lastCheck, err := time.Parse(time.RFC3339, cfg.LastBoardCheck)
			if err == nil && time.Since(lastCheck) < staleDuration {
				// Not stale — exit silently with no network call
				return nil
			}
		}
		// Stale (or no LastBoardCheck) → fall through to normal inbox check
	}

	cfg, err := account.LoadConfig()
	if err != nil {
		if quietMode {
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		if quietMode {
			return nil
		}
		return fmt.Errorf("not logged in or team not configured")
	}

	inbox, err := account.GetInbox(cfg)
	if err != nil {
		if quietMode {
			return nil
		}
		return fmt.Errorf("failed to get inbox: %w", err)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(inbox, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if inbox.UnreadCount == 0 && len(inbox.BoardChanges) == 0 {
		if !quietMode {
			fmt.Println("All caught up! No new notifications or board changes.")
		}
		return nil
	}

	if inbox.UnreadCount > 0 {
		fmt.Printf("You have %d unread notification(s):\n\n", inbox.UnreadCount)
		for _, n := range inbox.Notifications {
			typeIcon := map[string]string{
				"question": "?",
				"blocker":  "!",
				"mention":  "@",
				"file":     "#",
			}[n.Type]
			if typeIcon == "" {
				typeIcon = "*"
			}

			fmt.Printf("  [%s] %s from %s: %s\n", typeIcon, n.ID[:8], n.FromUser, n.Subject)
			if n.Body != "" {
				// Show first line of body
				lines := strings.SplitN(n.Body, "\n", 2)
				fmt.Printf("      %s\n", lines[0])
			}
			fmt.Printf("      %s\n\n", n.CreatedAt.Format("2006-01-02 15:04"))
		}
		fmt.Println("Mark as read: claw inbox read <id>")
	}

	if len(inbox.BoardChanges) > 0 {
		fmt.Printf("\nBoard changes since last check:\n\n")
		for _, bs := range inbox.BoardChanges {
			preview := bs.Content
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			preview = strings.ReplaceAll(preview, "\n", " ")
			fmt.Printf("  [%s] updated by %s: %s\n", bs.Section, bs.UpdatedBy, preview)
		}
		fmt.Println("\nView full board: claw board")
	}

	return nil
}

func runInboxRead(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	notifID := args[0]

	// Try to find the full notification ID if a prefix was given
	if len(notifID) < 32 {
		// It's a prefix - get all notifications and find the match
		userID := cfg.UserID
		if userID == "" {
			userID = cfg.Name
			if userID == "" {
				userID = cfg.Email
			}
		}
		notifications, err := account.GetNotifications(cfg, userID, false)
		if err == nil {
			for _, n := range notifications {
				if strings.HasPrefix(n.ID, notifID) {
					notifID = n.ID
					break
				}
			}
		}
	}

	if err := account.MarkNotificationRead(cfg, notifID); err != nil {
		return fmt.Errorf("failed to mark read: %w", err)
	}

	fmt.Printf("Marked %s as read.\n", notifID[:8])
	return nil
}

func runInboxReply(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured")
	}

	notifID := args[0]
	body := args[1]

	// Find the original notification to get the sender
	userID := cfg.UserID
	if userID == "" {
		userID = cfg.Name
		if userID == "" {
			userID = cfg.Email
		}
	}
	notifications, err := account.GetNotifications(cfg, userID, false)
	if err != nil {
		return fmt.Errorf("failed to get notifications: %w", err)
	}

	var originalFrom string
	var originalSubject string
	for _, n := range notifications {
		if strings.HasPrefix(n.ID, notifID) {
			originalFrom = n.FromUser
			originalSubject = n.Subject
			notifID = n.ID
			break
		}
	}

	if originalFrom == "" {
		return fmt.Errorf("notification not found: %s", notifID)
	}

	// Mark original as read
	account.MarkNotificationRead(cfg, notifID)

	// Send reply as a new notification
	subject := "Re: " + originalSubject
	n, err := account.SendNotification(cfg, originalFrom, "mention", subject, body)
	if err != nil {
		return fmt.Errorf("failed to send reply: %w", err)
	}

	fmt.Printf("Replied to %s: %s\n", n.ToUser, subject)
	return nil
}
