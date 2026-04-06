package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	projectID := fs.String("project", "", "Target project ID (required if workspace has multiple projects)")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	slug := fs.String("slug", "", "Route slug for the MCP URL (required)")
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

	pid := *projectID
	if pid == "" {
		projects, err := c.ListProjects(wsID)
		if err != nil {
			return fmt.Errorf("publish: list projects: %w", err)
		}
		switch len(projects) {
		case 0:
			return fmt.Errorf("publish: no projects in workspace %s", wsID)
		case 1:
			pid = projects[0].ID
		default:
			fmt.Fprintln(os.Stderr, "Multiple projects found. Specify one with --project <id>:")
			for _, p := range projects {
				fmt.Fprintf(os.Stderr, "  %s  %s\n", p.ID, p.Name)
			}
			os.Exit(1)
		}
	}

	fmt.Printf("Publishing project %s...\n", pid)
	result, err := c.Publish(pid, *slug)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	fmt.Printf("✓ Deployment %s created (status: %s)\n", result.Deployment.ID, result.Deployment.Status)
	if result.MCPURL != "" {
		fmt.Printf("  MCP URL: %s\n", result.MCPURL)
	}
	return nil
}
