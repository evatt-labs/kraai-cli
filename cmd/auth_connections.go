package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/evatt-labs/kraai-cli/internal/client"
	"github.com/evatt-labs/kraai-cli/internal/config"
	"golang.org/x/term"
)

func runAuthConnections(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "create":
			return runAuthConnectionsCreate(args[1:])
		case "delete":
			return runAuthConnectionsDelete(args[1:])
		}
	}
	return runAuthConnectionsList(args)
}

func runAuthConnectionsList(args []string) error {
	fs := flag.NewFlagSet("auth-connections", flag.ContinueOnError)
	serverID := fs.String("server", "", "Server ID")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	pid, wsID, err := resolveServerID(creds, *serverID, *workspaceID)
	if err != nil {
		return fmt.Errorf("auth-connections: %w", err)
	}

	c := client.New(apiBaseURL, creds.Token)
	conns, err := c.ListAuthConnections(wsID, pid)
	if err != nil {
		return fmt.Errorf("auth-connections: %w", err)
	}

	if len(conns) == 0 {
		fmt.Println("No auth connections found.")
		return nil
	}

	for _, ac := range conns {
		fmt.Printf("  %s  %-20s  %s  %s\n", ac.ID, ac.Name, ac.AuthKind, ac.CreatedAt[:10])
	}
	return nil
}

func runAuthConnectionsCreate(args []string) error {
	fs := flag.NewFlagSet("auth-connections create", flag.ContinueOnError)
	serverID := fs.String("server", "", "Server ID")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	name := fs.String("name", "", "Connection name")
	kind := fs.String("kind", "", "Auth kind: api_key, bearer_token, basic_auth")
	injectIn := fs.String("inject-in", "", "Where to inject: header, query, cookie")
	injectName := fs.String("inject-name", "", "Header/query/cookie name (e.g. Authorization, X-API-Key)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	pid, wsID, err := resolveServerID(creds, *serverID, *workspaceID)
	if err != nil {
		return fmt.Errorf("auth-connections create: %w", err)
	}

	// Prompt for missing fields interactively.
	reader := bufio.NewReader(os.Stdin)
	if *name == "" {
		*name = prompt(reader, "Connection name: ")
	}
	if *kind == "" {
		*kind = prompt(reader, "Auth kind (api_key, bearer_token, basic_auth): ")
	}
	if *injectIn == "" {
		*injectIn = prompt(reader, "Inject in (header, query, cookie): ")
	}
	if *injectName == "" {
		*injectName = prompt(reader, "Inject name (e.g. Authorization, X-API-Key): ")
	}

	// Always prompt for the secret (never accept via flag).
	// Use term.ReadPassword so the value is not echoed to the terminal.
	fmt.Print("Secret value: ")
	secretBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("auth-connections create: read secret: %w", err)
	}
	secret := string(secretBytes)
	if secret == "" {
		return fmt.Errorf("secret is required")
	}

	c := client.New(apiBaseURL, creds.Token)
	ac, err := c.CreateAuthConnection(wsID, pid, client.CreateAuthConnectionInput{
		Name:       *name,
		Kind:       *kind,
		InjectIn:   *injectIn,
		InjectName: *injectName,
		Secret:     secret,
	})
	if err != nil {
		return fmt.Errorf("auth-connections create: %w", err)
	}

	fmt.Printf("✓ Auth connection created: %s (%s)\n", ac.Name, ac.ID)
	fmt.Printf("  Secret is encrypted and will not be shown again.\n")
	return nil
}

func runAuthConnectionsDelete(args []string) error {
	fs := flag.NewFlagSet("auth-connections delete", flag.ContinueOnError)
	serverID := fs.String("server", "", "Server ID")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: kraai auth-connections delete <id> [--server <id>]")
	}
	connID := fs.Arg(0)

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	pid, wsID, err := resolveServerID(creds, *serverID, *workspaceID)
	if err != nil {
		return fmt.Errorf("auth-connections delete: %w", err)
	}

	c := client.New(apiBaseURL, creds.Token)
	if err := c.DeleteAuthConnection(wsID, pid, connID); err != nil {
		return fmt.Errorf("auth-connections delete: %w", err)
	}

	fmt.Printf("✓ Auth connection %s deleted.\n", connID)
	return nil
}

// resolveServerID resolves a server ID from explicit flag / KRAAI_SERVER_ID env,
// or auto-selects if the workspace has exactly one server.
func resolveServerID(creds *config.Credentials, serverID, workspaceID string) (string, string, error) {
	wsID := resolveWorkspace(creds.WorkspaceID, workspaceID)

	if serverID != "" {
		return serverID, wsID, nil
	}
	if env := os.Getenv("KRAAI_SERVER_ID"); env != "" {
		return env, wsID, nil
	}

	if wsID == "" {
		return "", "", fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	c := client.New(apiBaseURL, creds.Token)
	servers, err := c.ListServers(wsID)
	if err != nil {
		return "", "", fmt.Errorf("list servers: %w", err)
	}
	switch len(servers) {
	case 0:
		return "", "", fmt.Errorf("no servers in workspace")
	case 1:
		return servers[0].ID, wsID, nil
	default:
		fmt.Fprintln(os.Stderr, "Multiple servers found. Specify one with --server <id> or KRAAI_SERVER_ID:")
		for _, s := range servers {
			fmt.Fprintf(os.Stderr, "  %s  %s\n", s.ID, s.Name)
		}
		os.Exit(1)
		return "", "", nil
	}
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}
