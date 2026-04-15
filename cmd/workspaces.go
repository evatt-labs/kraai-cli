package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
	"github.com/evatt-labs/kraai-cli/internal/config"
)

func runWorkspaces(args []string) error {
	fs := flag.NewFlagSet("workspaces", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	sub := ""
	if fs.NArg() > 0 {
		sub = fs.Arg(0)
	}

	switch sub {
	case "", "list":
		return listWorkspaces()
	case "new", "create":
		name := ""
		if fs.NArg() >= 2 {
			name = fs.Arg(1)
		}
		return createWorkspace(name)
	case "use":
		if fs.NArg() < 2 {
			return fmt.Errorf("usage: kraai workspaces use <workspace-id>")
		}
		return useWorkspace(fs.Arg(1))
	case "rename":
		if fs.NArg() < 3 {
			return fmt.Errorf("usage: kraai workspaces rename <workspace-id> <new-name>")
		}
		return renameWorkspace(fs.Arg(1), fs.Arg(2))
	default:
		return fmt.Errorf("unknown subcommand: %s\n\nUsage:\n  kraai workspaces [list]              List workspaces\n  kraai workspaces new [name]          Create a workspace\n  kraai workspaces use <id>            Switch active workspace\n  kraai workspaces rename <id> <name>  Rename a workspace\n\nWorkspace deletion lives in the web UI at app.kraai.dev — it's irreversible and gated on a confirmation flow.", sub)
	}
}

func listWorkspaces() error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)
	workspaces, err := c.ListWorkspaces()
	if err != nil {
		return fmt.Errorf("workspaces: %w", err)
	}

	for _, ws := range workspaces {
		active := "  "
		if ws.ID == creds.WorkspaceID {
			active = "* "
		}
		fmt.Printf("%s%s (%s)   %s\n", active, ws.Name, ws.ID, ws.Entitlements.Label)
	}
	if len(workspaces) == 0 {
		fmt.Println("No workspaces found.")
	}
	return nil
}

func useWorkspace(id string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)
	workspaces, err := c.ListWorkspaces()
	if err != nil {
		return fmt.Errorf("workspaces use: %w", err)
	}

	for _, ws := range workspaces {
		if ws.ID == id {
			creds.WorkspaceID = ws.ID
			creds.WorkspaceName = ws.Name
			if err := config.Save(creds); err != nil {
				return fmt.Errorf("workspaces use: save: %w", err)
			}
			fmt.Printf("Switched to workspace: %s\n", ws.Name)
			return nil
		}
	}
	return fmt.Errorf("workspace %q not found", id)
}

func createWorkspace(name string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}

	if name == "" {
		name = workspaceNameFromEmail(creds.Email)
	}

	c := client.New(apiBaseURL, creds.Token)
	ws, err := c.CreateWorkspace(name)
	if err != nil {
		return fmt.Errorf("workspaces new: %w", err)
	}

	// Auto-switch to the new workspace.
	creds.WorkspaceID = ws.ID
	creds.WorkspaceName = ws.Name
	if err := config.Save(creds); err != nil {
		return fmt.Errorf("workspaces new: save: %w", err)
	}

	fmt.Printf("Created workspace: %s (%s)\n", ws.Name, ws.ID)
	fmt.Printf("Switched to workspace: %s\n", ws.Name)
	return nil
}

func renameWorkspace(id, name string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	c := client.New(apiBaseURL, creds.Token)
	if err := c.RenameWorkspace(id, name); err != nil {
		return fmt.Errorf("workspaces rename: %w", err)
	}
	fmt.Printf("Renamed workspace %s to %q\n", id, name)
	return nil
}

func requireCreds() (*config.Credentials, error) {
	// KRAAI_TOKEN env var overrides credentials file.
	if tok := os.Getenv("KRAAI_TOKEN"); tok != "" {
		wsID := os.Getenv("KRAAI_WORKSPACE_ID")
		return &config.Credentials{Token: tok, WorkspaceID: wsID}, nil
	}

	creds, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load credentials: %w", err)
	}
	if creds == nil {
		fmt.Fprintln(os.Stderr, "Not logged in. Run 'kraai login'.")
		os.Exit(1)
	}
	return creds, nil
}
