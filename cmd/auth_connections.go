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
	projectID := fs.String("project", "", "Project ID")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	pid, err := resolveProjectID(creds, *projectID, *workspaceID)
	if err != nil {
		return fmt.Errorf("auth-connections: %w", err)
	}

	c := client.New(apiBaseURL, creds.Token)
	conns, err := c.ListAuthConnections(pid)
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
	projectID := fs.String("project", "", "Project ID")
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

	pid, err := resolveProjectID(creds, *projectID, *workspaceID)
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
	ac, err := c.CreateAuthConnection(pid, client.CreateAuthConnectionInput{
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
	projectID := fs.String("project", "", "Project ID")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: kraai auth-connections delete <id> [--project <id>]")
	}
	connID := fs.Arg(0)

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	pid, err := resolveProjectID(creds, *projectID, *workspaceID)
	if err != nil {
		return fmt.Errorf("auth-connections delete: %w", err)
	}

	c := client.New(apiBaseURL, creds.Token)
	if err := c.DeleteAuthConnection(pid, connID); err != nil {
		return fmt.Errorf("auth-connections delete: %w", err)
	}

	fmt.Printf("✓ Auth connection %s deleted.\n", connID)
	return nil
}

// resolveProjectID resolves a project ID from explicit flag, or auto-selects
// if the workspace has exactly one project.
func resolveProjectID(creds *config.Credentials, projectID, workspaceID string) (string, error) {
	if projectID != "" {
		return projectID, nil
	}

	wsID := resolveWorkspace(creds.WorkspaceID, workspaceID)
	if wsID == "" {
		return "", fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	c := client.New(apiBaseURL, creds.Token)
	projects, err := c.ListProjects(wsID)
	if err != nil {
		return "", fmt.Errorf("list projects: %w", err)
	}
	switch len(projects) {
	case 0:
		return "", fmt.Errorf("no projects in workspace")
	case 1:
		return projects[0].ID, nil
	default:
		fmt.Fprintln(os.Stderr, "Multiple projects found. Specify one with --project <id>:")
		for _, p := range projects {
			fmt.Fprintf(os.Stderr, "  %s  %s\n", p.ID, p.Name)
		}
		os.Exit(1)
		return "", nil
	}
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}
