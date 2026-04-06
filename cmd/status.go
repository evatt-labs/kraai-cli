package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runStatus(args []string) error {
	// Separate flags from positional args.
	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && !strings.Contains(args[i], "=") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			posArgs = append(posArgs, args[i])
		}
	}

	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	projectID := fs.String("project", "", "Show deployment status for a project")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	token := fs.String("token", "", "Bearer token for the MCP endpoint health check")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	// kraai status <slug> — check if the live MCP endpoint responds.
	if len(posArgs) > 0 {
		return statusBySlug(posArgs[0], *token)
	}

	// kraai status --project <id> — show deployment info from API.
	if *projectID != "" {
		return statusByProject(*projectID)
	}

	// No args: try to resolve single project in workspace.
	creds, err := requireCreds()
	if err != nil {
		return err
	}

	wsID := resolveWorkspace(creds.WorkspaceID, *workspaceID)
	if wsID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	c := client.New(apiBaseURL, creds.Token)
	projects, err := c.ListProjects(wsID)
	if err != nil {
		return fmt.Errorf("status: list projects: %w", err)
	}
	switch len(projects) {
	case 0:
		return fmt.Errorf("status: no projects in workspace")
	case 1:
		return statusByProject(projects[0].ID)
	default:
			fmt.Fprintln(os.Stderr, "Multiple projects found. Specify one with --project <id> or pass a slug:")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "  kraai status <slug>")
			fmt.Fprintln(os.Stderr, "  kraai status --project <id>")
			fmt.Fprintln(os.Stderr)
		for _, p := range projects {
			fmt.Fprintf(os.Stderr, "  %s  %s\n", p.ID, p.Name)
		}
		os.Exit(1)
	}
	return nil
}

func statusBySlug(slug, token string) error {
	endpoint := runtimeBaseURL() + "/mcp/" + slug
	fmt.Printf("Endpoint: %s\n", endpoint)
	fmt.Printf("Checking...")

	mc := client.NewMCPClient(endpoint, token)
	info, err := mc.Initialize()
	if err != nil {
		fmt.Printf(" ✗\n\n")
		return fmt.Errorf("endpoint unreachable: %w", err)
	}

	fmt.Printf(" ✓ live\n\n")
	fmt.Printf("  Server:   %s %s\n", info.Name, info.Version)
	fmt.Printf("  Protocol: %s\n", info.ProtocolVersion)
	return nil
}

func statusByProject(projectID string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)
	deployments, err := c.ListDeployments(projectID)
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}

	if len(deployments) == 0 {
		fmt.Println("No deployments found for this project.")
		return nil
	}

	// Show the most recent deployment.
	d := deployments[0]
	fmt.Printf("Project:    %s\n", projectID)
	fmt.Printf("Deployment: %s\n", d.ID)
	fmt.Printf("Status:     %s\n", d.Status)
	fmt.Printf("Created:    %s\n", d.CreatedAt)
	return nil
}
