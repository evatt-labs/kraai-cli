package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	serverID := fs.String("server", "", "Target server ID (required if workspace has multiple servers)")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	slug := fs.String("slug", "", "Route slug for the MCP URL (required)")
	authConn := fs.String("auth-connection", "", "Attach an auth connection ID")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return fmt.Errorf("publish: --slug is required")
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	pid, wsID, err := resolveServerID(creds, *serverID, *workspaceID)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	c := client.New(apiBaseURL, creds.Token)

	// Pre-check slug availability.
	available, slugErr := c.CheckSlugAvailability(wsID, pid, *slug)
	if slugErr == nil && !available {
		return fmt.Errorf("publish: slug %q is already taken", *slug)
	}

	fmt.Printf("Publishing server %s...\n", pid)
	result, err := c.Publish(wsID, pid, *slug, *authConn)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	fmt.Printf("✓ Deployment %s created (status: %s)\n", result.Deployment.ID, result.Deployment.Status)
	if result.MCPURL != "" {
		fmt.Printf("  MCP URL: %s\n", result.MCPURL)
	}
	return nil
}
