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
	serverID := fs.String("server", "", "Show deployment status for a server")
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

	// kraai status --server <id> — show deployment info from API.
	if *serverID != "" {
		return statusByServer(*serverID)
	}

	// No args: try to resolve single server in workspace.
	creds, err := requireCreds()
	if err != nil {
		return err
	}

	wsID := resolveWorkspace(creds.WorkspaceID, *workspaceID)
	if wsID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	c := client.New(apiBaseURL, creds.Token)
	servers, err := c.ListServers(wsID)
	if err != nil {
		return fmt.Errorf("status: list servers: %w", err)
	}
	switch len(servers) {
	case 0:
		return fmt.Errorf("status: no servers in workspace")
	case 1:
		return statusByServer(servers[0].ID)
	default:
		fmt.Fprintln(os.Stderr, "Multiple servers found. Specify one with --server <id> or pass a slug:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  kraai status <slug>")
		fmt.Fprintln(os.Stderr, "  kraai status --server <id>")
		fmt.Fprintln(os.Stderr)
		for _, s := range servers {
			fmt.Fprintf(os.Stderr, "  %s  %s\n", s.ID, s.Name)
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

func statusByServer(serverID string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)
	deployments, err := c.ListDeployments(serverID)
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}

	if len(deployments) == 0 {
		fmt.Println("No deployments found for this server.")
		return nil
	}

	// Show the most recent deployment.
	d := deployments[0]
	fmt.Printf("Server:     %s\n", serverID)
	fmt.Printf("Deployment: %s\n", d.ID)
	fmt.Printf("Status:     %s\n", d.Status)
	fmt.Printf("Created:    %s\n", d.CreatedAt)
	return nil
}
