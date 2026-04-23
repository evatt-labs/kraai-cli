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
		arg := posArgs[0]
		if isUUID(arg) {
			creds, err := requireCreds()
			if err != nil {
				return err
			}
			pid, wsID, err := resolveServerID(creds, arg, *workspaceID)
			if err != nil {
				return fmt.Errorf("status: %w", err)
			}
			return statusByServer(wsID, pid)
		}
		return statusBySlug(arg, *token)
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	pid, wsID, err := resolveServerID(creds, *serverID, *workspaceID)
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}

	return statusByServer(wsID, pid)
}

func isUUID(u string) bool {
	if len(u) != 36 {
		return false
	}
	for i, c := range u {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}


func statusBySlug(slug, token string) error {
	endpoint := runtimeBaseURL() + "/" + slug
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

func statusByServer(workspaceID, serverID string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)
	deployments, err := c.ListDeployments(workspaceID, serverID)
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
