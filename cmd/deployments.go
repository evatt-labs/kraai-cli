package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
	"github.com/evatt-labs/kraai-cli/internal/config"
)

func runDeployments(args []string) error {
	// Subcommand dispatch — peek at the first non-flag arg without consuming it
	// from the underlying flag set used by listDeployments below.
	if len(args) > 0 {
		switch args[0] {
		case "reissue-token":
			return runReissueToken(args[1:])
		case "list":
			return runListDeployments(args[1:])
		}
	}
	return runListDeployments(args)
}

func runListDeployments(args []string) error {
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
	pid, c, err := resolveProjectAndClient(creds, *projectID, *workspaceID)
	if err != nil {
		return err
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

func runReissueToken(args []string) error {
	fs := flag.NewFlagSet("deployments reissue-token", flag.ContinueOnError)
	projectID := fs.String("project", "", "Target project ID")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: kraai deployments reissue-token <deployment-id> [--project <id>]")
	}
	deploymentID := fs.Arg(0)

	creds, err := requireCreds()
	if err != nil {
		return err
	}
	pid, c, err := resolveProjectAndClient(creds, *projectID, *workspaceID)
	if err != nil {
		return err
	}

	token, err := c.ReissueDeploymentToken(pid, deploymentID)
	if err != nil {
		return fmt.Errorf("reissue-token: %w", err)
	}
	fmt.Printf("✓ Token reissued. Old token revoked.\n\n")
	fmt.Printf("  New token: %s\n\n", token)
	fmt.Printf("  Store it safely — it will not be shown again.\n")
	return nil
}

// resolveProjectAndClient picks a project ID from --project / single-project
// shortcut and constructs an API client. Used by every deployments subcommand.
func resolveProjectAndClient(creds *config.Credentials, projectID, workspaceID string) (string, *client.Client, error) {
	wsID := creds.WorkspaceID
	if workspaceID != "" {
		wsID = workspaceID
	}
	if wenv := os.Getenv("KRAAI_WORKSPACE_ID"); wenv != "" {
		wsID = wenv
	}

	c := client.New(apiBaseURL, creds.Token)

	if projectID != "" {
		return projectID, c, nil
	}
	projects, err := c.ListProjects(wsID)
	if err != nil {
		return "", nil, fmt.Errorf("list projects: %w", err)
	}
	switch len(projects) {
	case 0:
		return "", nil, fmt.Errorf("no projects in workspace")
	case 1:
		return projects[0].ID, c, nil
	default:
		fmt.Fprintln(os.Stderr, "Multiple projects found. Specify one with --project <id>:")
		for _, p := range projects {
			fmt.Fprintf(os.Stderr, "  %s  %s\n", p.ID, p.Name)
		}
		os.Exit(1)
		return "", nil, nil // unreachable
	}
}
