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

	wsID := creds.WorkspaceID
	if *workspaceID != "" {
		wsID = *workspaceID
	}
	if wenv := os.Getenv("KRAAI_WORKSPACE_ID"); wenv != "" {
		wsID = wenv
	}

	c := client.New(apiBaseURL, creds.Token)

	pid := *serverID
	if pid == "" {
		pid = os.Getenv("KRAAI_SERVER_ID")
	}
	if pid == "" {
		servers, err := c.ListServers(wsID)
		if err != nil {
			return fmt.Errorf("publish: list servers: %w", err)
		}
		switch len(servers) {
		case 0:
			return fmt.Errorf("publish: no servers in workspace %s", wsID)
		case 1:
			pid = servers[0].ID
		default:
			fmt.Fprintln(os.Stderr, "Multiple servers found. Specify one with --server <id>:")
			for _, s := range servers {
				fmt.Fprintf(os.Stderr, "  %s  %s\n", s.ID, s.Name)
			}
			os.Exit(1)
		}
	}

	// Pre-check slug availability.
	available, slugErr := c.CheckSlugAvailability(pid, *slug)
	if slugErr == nil && !available {
		return fmt.Errorf("publish: slug %q is already taken", *slug)
	}

	fmt.Printf("Publishing server %s...\n", pid)
	result, err := c.Publish(pid, *slug, *authConn)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	fmt.Printf("✓ Deployment %s created (status: %s)\n", result.Deployment.ID, result.Deployment.Status)
	if result.MCPURL != "" {
		fmt.Printf("  MCP URL: %s\n", result.MCPURL)
	}
	return nil
}
