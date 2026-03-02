package main

import (
	"fmt"

	"github.com/epuerta9/claw2claw/internal/account"
	"github.com/spf13/cobra"
)

func newTeamCmd() *cobra.Command {
	teamCmd := &cobra.Command{
		Use:   "team",
		Short: "Manage your team for shared board and notifications",
		Long: `Create, join, and manage teams for async agent-to-agent communication.

A team gives you a shared board, notifications, and file sharing.
Only team members can access the board.`,
	}

	createCmd := &cobra.Command{
		Use:   "create <slug>",
		Short: "Create a new team",
		Long: `Create a new team and become its owner.

The slug is a short identifier used in commands (e.g., "backstop").
A join token is generated that you share with teammates.

Example:
  claw team create backstop --name "Backstop" --members eduardo,jared`,
		Args: cobra.ExactArgs(1),
		RunE: runTeamCreate,
	}
	createCmd.Flags().String("name", "", "Team display name (defaults to slug)")
	createCmd.Flags().StringSlice("members", nil, "Member names for board initialization")

	joinCmd := &cobra.Command{
		Use:   "join <token>",
		Short: "Join a team using a join token",
		Long: `Join a team using the join token provided by the team owner.

Example:
  claw team join claw_join_abc123def456`,
		Args: cobra.ExactArgs(1),
		RunE: runTeamJoin,
	}

	membersCmd := &cobra.Command{
		Use:   "members",
		Short: "List team members",
		RunE:  runTeamMembers,
	}

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show current team info",
		RunE:  runTeamInfo,
	}

	tokenCmd := &cobra.Command{
		Use:   "token",
		Short: "Generate a new join token for your team",
		RunE:  runTeamToken,
	}

	teamCmd.AddCommand(createCmd, joinCmd, membersCmd, infoCmd, tokenCmd)
	return teamCmd
}

func runTeamCreate(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn {
		return fmt.Errorf("not logged in. Run: claw login")
	}

	slug := args[0]
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		name = slug
	}
	members, _ := cmd.Flags().GetStringSlice("members")

	result, err := account.CreateTeam(cfg, name, slug, members)
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	// Save team_id to config
	cfg.TeamID = slug
	if err := account.SaveConfig(cfg); err != nil {
		fmt.Printf("Warning: failed to save team to config: %v\n", err)
	}

	fmt.Printf("Team created: %s\n", name)
	fmt.Printf("Slug: %s\n", slug)
	fmt.Printf("\nJoin token (share with teammates):\n")
	fmt.Printf("  %s\n", result.JoinToken)
	fmt.Printf("\nTeammates join with:\n")
	fmt.Printf("  claw team join %s\n", result.JoinToken)

	if len(members) > 0 {
		fmt.Printf("\nBoard initialized for: %v\n", members)
	}

	return nil
}

func runTeamJoin(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn {
		return fmt.Errorf("not logged in. Run: claw login")
	}

	token := args[0]

	result, err := account.JoinTeam(cfg, token)
	if err != nil {
		return fmt.Errorf("failed to join team: %w", err)
	}

	// Save team_id to config
	cfg.TeamID = result.Team.Slug
	if err := account.SaveConfig(cfg); err != nil {
		fmt.Printf("Warning: failed to save team to config: %v\n", err)
	}

	fmt.Printf("Joined team: %s (%s)\n", result.Team.Name, result.Team.Slug)
	fmt.Println("\nYou now have access to the shared board:")
	fmt.Println("  claw board       # View the board")
	fmt.Println("  claw inbox       # Check notifications")
	return nil
}

func runTeamMembers(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured")
	}

	members, err := account.GetTeamMembers(cfg)
	if err != nil {
		return fmt.Errorf("failed to get members: %w", err)
	}

	fmt.Printf("Team: %s\n\n", cfg.TeamID)
	for _, m := range members {
		roleIcon := "  "
		if m.Role == "owner" {
			roleIcon = "* "
		}
		fmt.Printf("  %s%s (%s) - joined %s\n", roleIcon, m.Name, m.Role, m.JoinedAt.Format("2006-01-02"))
	}
	return nil
}

func runTeamInfo(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured")
	}

	team, err := account.GetTeamInfo(cfg)
	if err != nil {
		return fmt.Errorf("failed to get team info: %w", err)
	}

	fmt.Printf("Team: %s\n", team.Name)
	fmt.Printf("Slug: %s\n", team.Slug)
	fmt.Printf("Created: %s\n", team.CreatedAt.Format("2006-01-02"))
	return nil
}

func runTeamToken(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured")
	}

	token, err := account.CreateJoinToken(cfg)
	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}

	fmt.Printf("New join token for team '%s':\n", cfg.TeamID)
	fmt.Printf("  %s\n", token)
	fmt.Printf("\nShare with: claw team join %s\n", token)
	return nil
}
