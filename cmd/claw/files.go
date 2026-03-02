package main

import (
	"fmt"

	"github.com/epuerta9/claw2claw/internal/account"
	"github.com/spf13/cobra"
)

func newShareCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "share <file>",
		Short: "Upload a file to the shared team board",
		Long: `Upload a file to the shared team board for other members to access.

Unlike p2p transfers (claw send), shared files are stored on the relay
server and available to all team members.

Example:
  c2c share ./schema.ts
  c2c share ./architecture.md`,
		Args: cobra.ExactArgs(1),
		RunE: runShare,
	}
}

func newFilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "files",
		Short: "List shared files on the team board",
		RunE:  runFiles,
	}
}

func newDownloadCmd() *cobra.Command {
	downloadCmd := &cobra.Command{
		Use:   "download <file-id>",
		Short: "Download a shared file from the team board",
		Args:  cobra.ExactArgs(1),
		RunE:  runDownload,
	}
	return downloadCmd
}

func runShare(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured")
	}

	sf, err := account.UploadFile(cfg, filePath)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	fmt.Printf("Uploaded: %s (%d bytes)\n", sf.Filename, sf.Size)
	fmt.Printf("File ID: %s\n", sf.ID)
	fmt.Println("Team members can download with: claw download", sf.ID)
	return nil
}

func runFiles(cmd *cobra.Command, args []string) error {
	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured")
	}

	files, err := account.ListFiles(cfg)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No shared files yet.")
		fmt.Println("Share a file: claw share <file>")
		return nil
	}

	fmt.Println("Shared files:")
	fmt.Println()
	for _, f := range files {
		fmt.Printf("  %-30s %8d bytes  by %s  %s\n",
			f.Filename, f.Size, f.UploadedBy, f.CreatedAt.Format("2006-01-02 15:04"))
		fmt.Printf("  ID: %s\n\n", f.ID)
	}
	fmt.Println("Download: claw download <file-id>")
	return nil
}

func runDownload(cmd *cobra.Command, args []string) error {
	fileID := args[0]

	cfg, err := account.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !cfg.LoggedIn || cfg.TeamID == "" {
		return fmt.Errorf("not logged in or team not configured")
	}

	outDir := ".claw/shared"
	path, err := account.DownloadFile(cfg, fileID, outDir)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Printf("Downloaded: %s\n", path)
	return nil
}
