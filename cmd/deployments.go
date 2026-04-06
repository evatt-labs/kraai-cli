package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runDeployments(args []string) error {
	fs := flag.NewFlagSet("deployments", flag.ContinueOnError)
	projectID := fs.String("project", "", "Target project ID")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
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
			return fmt.Errorf("deployments: list projects: %w", err)
		}
		switch len(projects) {
		case 0:
			return fmt.Errorf("deployments: no projects in workspace")
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

	deployments, err := c.ListDeployments(pid)
	if err != nil {
		return fmt.Errorf("deployments: %w", err)
	}

	if len(deployments) == 0 {
		fmt.Println("No deployments found.")
		return nil
	}

	for _, d := range deployments {
		fmt.Printf("%s  %-10s  %s\n", d.ID, d.Status, d.CreatedAt)
	}
	return nil
}
