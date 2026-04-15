package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

// runProjects is kept for backward-compat muscle memory but internally delegates
// to the server model. Consider it a legacy alias — the canonical surface is
// `kraai servers`.
func runProjects(args []string) error {
	fs := flag.NewFlagSet("servers", flag.ContinueOnError)
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
		return listServers()
	case "new", "create":
		name := ""
		if fs.NArg() >= 2 {
			name = fs.Arg(1)
		}
		return createServer(name)
	case "rename":
		if fs.NArg() < 3 {
			return fmt.Errorf("usage: kraai servers rename <server-id> <new-name>")
		}
		return renameServer(fs.Arg(1), fs.Arg(2))
	case "delete":
		if fs.NArg() < 2 {
			return fmt.Errorf("usage: kraai servers delete <server-id>")
		}
		return deleteServer(fs.Arg(1))
	default:
		return fmt.Errorf("unknown subcommand: %s\n\nUsage:\n  kraai servers [list]                  List servers in active workspace\n  kraai servers new [name]              Create a server\n  kraai servers rename <id> <name>      Rename a server\n  kraai servers delete <id>             Delete a server", sub)
	}
}

func listServers() error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	if creds.WorkspaceID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	c := client.New(apiBaseURL, creds.Token)
	servers, err := c.ListServers(creds.WorkspaceID)
	if err != nil {
		return fmt.Errorf("servers: %w", err)
	}

	for _, s := range servers {
		fmt.Printf("  %s  %s\n", s.ID, s.Name)
	}
	if len(servers) == 0 {
		fmt.Println("No servers found.")
	}
	return nil
}

func createServer(name string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	if creds.WorkspaceID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}
	if name == "" {
		return fmt.Errorf("usage: kraai servers new <name>")
	}

	c := client.New(apiBaseURL, creds.Token)
	srv, err := c.CreateServer(creds.WorkspaceID, name)
	if err != nil {
		return fmt.Errorf("servers new: %w", err)
	}

	fmt.Printf("Created server: %s (%s)\n", srv.Name, srv.ID)
	return nil
}

func renameServer(id, name string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	c := client.New(apiBaseURL, creds.Token)
	if err := c.RenameServer(id, name); err != nil {
		return fmt.Errorf("servers rename: %w", err)
	}
	fmt.Printf("Renamed server %s to %q\n", id, name)
	return nil
}

func deleteServer(id string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	c := client.New(apiBaseURL, creds.Token)
	if err := c.DeleteServer(id); err != nil {
		return fmt.Errorf("servers delete: %w", err)
	}
	fmt.Printf("Deleted server %s\n", id)
	return nil
}
