package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/epuerta9/claw2claw/internal/account"
	"github.com/spf13/cobra"
)

func newBoardCmd() *cobra.Command {
	boardCmd := &cobra.Command{
		Use:   "board [section]",
		Short: "View or update the shared team board",
		Long: `View or update the shared team board.

The board is a persistent shared document where team members' agents
post updates, decisions, questions, and context.

Without arguments, shows the full board.
With a section name, shows just that section.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runBoard,
	}

	updateCmd := &cobra.Command{
		Use:   "update <section> [content]",
		Short: "Update a board section",
		Long: `Update a board section with new content.

If content is provided as an argument, it is used directly.
If no content argument is given, reads from stdin.

Sections: status, questions, decisions, context, files

Examples:
  c2c board update status "All services green"
  c2c board update context "Working on CLO-274, refactored API routes"
  echo "New decision" | c2c board update decisions`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runBoardUpdate,
	}

	editCmd := &cobra.Command{
		Use:   "edit <section>",
		Short: "Edit a board section (reads content from stdin)",
		Args:  cobra.ExactArgs(1),
		RunE:  runBoardEdit,
	}

	initCmd := &cobra.Command{
		Use:   "init [members...]",
		Short: "Initialize the board with default template",
		Long: `Initialize the team board with the default template.

Provide team member names to create per-member context sections.

Example:
  c2c board init eduardo jared`,
		Args: cobra.MinimumNArgs(1),
		RunE: runBoardInit,
	}

	boardCmd.AddCommand(updateCmd, editCmd, initCmd)
	return boardCmd
}

func runBoard(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured. Run: claw login && set team_id in ~/.claw/account.json")
	}

	if len(args) == 1 {
		// Show specific section
		section := args[0]
		// Auto-expand "context" to "context:<user_id>" if no colon
		if section == "context" {
			userID := cfg.UserID
			if userID == "" {
				userID = cfg.Name
			}
			section = "context:" + userID
		}

		bs, err := account.GetBoardSection(cfg, section)
		if err != nil {
			return fmt.Errorf("failed to get section: %w", err)
		}
		if bs == nil {
			fmt.Printf("Section '%s' not found.\n", section)
			return nil
		}

		fmt.Println(bs.Content)
		fmt.Printf("\n--- Updated by %s at %s (v%d) ---\n", bs.UpdatedBy, bs.UpdatedAt.Format("2006-01-02 15:04"), bs.Version)
		return nil
	}

	// Show full board
	sections, err := account.GetBoard(cfg)
	if err != nil {
		return fmt.Errorf("failed to get board: %w", err)
	}

	if len(sections) == 0 {
		fmt.Println("Board is empty. Initialize it with: claw board init <member1> <member2>")
		return nil
	}

	for _, s := range sections {
		fmt.Println(s.Content)
		fmt.Printf("  _Updated by %s at %s_\n\n", s.UpdatedBy, s.UpdatedAt.Format("2006-01-02 15:04"))
	}

	return nil
}

func runBoardUpdate(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured")
	}

	section := args[0]

	// Auto-expand "context" to "context:<user_id>"
	if section == "context" {
		userID := cfg.UserID
		if userID == "" {
			userID = cfg.Name
		}
		section = "context:" + userID
	}

	var content string
	if len(args) > 1 {
		content = args[1]
	} else {
		// Read from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		content = strings.TrimSpace(string(data))
	}

	if content == "" {
		return fmt.Errorf("content is required")
	}

	bs, err := account.UpdateBoardSection(cfg, section, content)
	if err != nil {
		return fmt.Errorf("failed to update section: %w", err)
	}

	fmt.Printf("Updated section '%s' (v%d)\n", bs.Section, bs.Version)
	return nil
}

func runBoardEdit(cmd *cobra.Command, args []string) error {
	// Same as update but always reads from stdin
	return runBoardUpdate(cmd, args)
}

func runBoardInit(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured")
	}

	if err := account.InitBoard(cfg, args); err != nil {
		return fmt.Errorf("failed to initialize board: %w", err)
	}

	fmt.Printf("Board initialized for team '%s' with members: %s\n", cfg.TeamID, strings.Join(args, ", "))
	fmt.Println("View it with: claw board")
	return nil
}
