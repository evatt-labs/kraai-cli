package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/evatt-labs/kraai-cli/internal/client"
	"github.com/evatt-labs/kraai-cli/internal/config"
)

func runServers(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(`usage: kraai servers <subcommand>

Subcommands:
  list                            List servers in the active workspace
  deployments <slug>              List deployments for a server
  activate <slug> <deployment-id> Switch to a specific deployment version
  reissue-token <slug> <dep-id>   Reissue the static bearer token for a deployment
  delete <slug>                   Delete a server`)
	}

	switch args[0] {
	case "list":
		return runServersList(args[1:])
	case "deployments":
		return runServerDeployments(args[1:])
	case "activate":
		return runServerActivate(args[1:])
	case "reissue-token":
		return runServerReissueToken(args[1:])
	case "delete":
		return runServerDelete(args[1:])
	case "--help", "-h", "help":
		return runServers(nil) // re-invoke with no args to print usage
	default:
		return fmt.Errorf("servers: unknown subcommand %q\n\nRun 'kraai servers' for usage.", args[0])
	}
}

// runServersList lists all servers in the active workspace.
func runServersList(args []string) error {
	fs := flag.NewFlagSet("servers list", flag.ContinueOnError)
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

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
		return fmt.Errorf("servers list: %w", err)
	}

	if len(servers) == 0 {
		fmt.Println("No servers found.")
		return nil
	}
	for _, s := range servers {
		fmt.Printf("  %s  %s\n", s.ID, s.Name)
	}
	return nil
}

// runServerDeployments lists deployments for a server identified by slug or ID.
// Usage: kraai servers deployments <slug> [--server <id>] [--workspace <id>]
func runServerDeployments(args []string) error {
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

	fs := flag.NewFlagSet("servers deployments", flag.ContinueOnError)
	serverID := fs.String("server", "", "Target server ID or slug")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	// Positional arg takes precedence over --server flag for the slug/id.
	sid := *serverID
	if len(posArgs) > 0 {
		sid = posArgs[0]
	}

	sid, c, err := resolveServerAndClient(creds, sid, *workspaceID)
	if err != nil {
		return err
	}

	deployments, err := c.ListDeployments(sid)
	if err != nil {
		return fmt.Errorf("servers deployments: %w", err)
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

// runServerActivate activates a specific deployment version.
// Usage: kraai servers activate <slug> <deployment-id> [--server <id>]
func runServerActivate(args []string) error {
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

	fs := flag.NewFlagSet("servers activate", flag.ContinueOnError)
	serverID := fs.String("server", "", "Target server ID (required if workspace has multiple servers)")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	if len(posArgs) < 2 {
		return fmt.Errorf("usage: kraai servers activate <slug> <deployment-id> [--server <id>]")
	}
	slug := posArgs[0]
	deploymentID := posArgs[1]
	_ = slug // slug is used to identify the server; pass to resolver

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	sid := *serverID
	if sid == "" {
		sid = slug
	}

	sid, c, err := resolveServerAndClient(creds, sid, *workspaceID)
	if err != nil {
		return err
	}

	fmt.Printf("Activating deployment %s...", deploymentID)
	result, err := c.ActivateDeployment(sid, deploymentID)
	if err != nil {
		return fmt.Errorf("servers activate: %w", err)
	}
	fmt.Println()
	fmt.Printf("\n✓ Deployment activated at %s\n", result.MCPURL)
	fmt.Printf("  View in console: %s/servers/%s/deployments\n\n", appBaseURL, sid)
	printConnectInstructions(result)
	return nil
}

// runServerReissueToken reissues the static bearer token for a deployment.
// Usage: kraai servers reissue-token <slug> <deployment-id> [--server <id>]
func runServerReissueToken(args []string) error {
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

	fs := flag.NewFlagSet("servers reissue-token", flag.ContinueOnError)
	serverID := fs.String("server", "", "Target server ID")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	if len(posArgs) < 2 {
		return fmt.Errorf("usage: kraai servers reissue-token <slug> <deployment-id> [--server <id>]")
	}
	slug := posArgs[0]
	deploymentID := posArgs[1]

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	sid := *serverID
	if sid == "" {
		sid = slug
	}

	sid, c, err := resolveServerAndClient(creds, sid, *workspaceID)
	if err != nil {
		return err
	}

	token, err := c.ReissueDeploymentToken(sid, deploymentID)
	if err != nil {
		return fmt.Errorf("servers reissue-token: %w", err)
	}
	fmt.Printf("✓ Token reissued. Old token revoked.\n\n")
	fmt.Printf("  New token: %s\n\n", token)
	fmt.Printf("  Store it safely — it will not be shown again.\n")
	return nil
}

// runServerDelete soft-deletes a server by slug or ID.
// Usage: kraai servers delete <slug> [--server <id>]
func runServerDelete(args []string) error {
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

	fs := flag.NewFlagSet("servers delete", flag.ContinueOnError)
	serverID := fs.String("server", "", "Target server ID")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	if len(posArgs) < 1 && *serverID == "" {
		return fmt.Errorf("usage: kraai servers delete <slug> [--server <id>]")
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	sid := *serverID
	if sid == "" {
		sid = posArgs[0]
	}

	sid, c, err := resolveServerAndClient(creds, sid, *workspaceID)
	if err != nil {
		return err
	}

	if err := c.DeleteServer(sid); err != nil {
		return fmt.Errorf("servers delete: %w", err)
	}
	fmt.Printf("✓ Server %s deleted.\n", sid)
	return nil
}

// resolveServerAndClient picks a server ID from --server / KRAAI_SERVER_ID /
// single-server shortcut and constructs an API client. Used by every servers
// subcommand that requires a server context.
func resolveServerAndClient(creds *config.Credentials, serverID, workspaceID string) (string, *client.Client, error) {
	wsID := creds.WorkspaceID
	if workspaceID != "" {
		wsID = workspaceID
	}
	if wenv := os.Getenv("KRAAI_WORKSPACE_ID"); wenv != "" {
		wsID = wenv
	}

	c := client.New(apiBaseURL, creds.Token)

	// Explicit server ID / env var wins.
	if serverID == "" {
		serverID = os.Getenv("KRAAI_SERVER_ID")
	}
	if serverID != "" {
		return serverID, c, nil
	}

	// Auto-select if workspace has exactly one server.
	servers, err := c.ListServers(wsID)
	if err != nil {
		return "", nil, fmt.Errorf("list servers: %w", err)
	}
	switch len(servers) {
	case 0:
		return "", nil, fmt.Errorf("no servers in workspace")
	case 1:
		return servers[0].ID, c, nil
	default:
		fmt.Fprintln(os.Stderr, "Multiple servers found. Specify one with --server <id> or KRAAI_SERVER_ID:")
		for _, s := range servers {
			fmt.Fprintf(os.Stderr, "  %s  %s\n", s.ID, s.Name)
		}
		os.Exit(1)
		return "", nil, nil // unreachable
	}
}
