package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runProjects(args []string) error {
	fs := flag.NewFlagSet("projects", flag.ContinueOnError)
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
		return listProjects()
	case "new", "create":
		name := ""
		if fs.NArg() >= 2 {
			name = fs.Arg(1)
		}
		return createProject(name)
	case "rename":
		if fs.NArg() < 3 {
			return fmt.Errorf("usage: kraai projects rename <project-id> <new-name>")
		}
		return renameProject(fs.Arg(1), fs.Arg(2))
	case "delete":
		if fs.NArg() < 2 {
			return fmt.Errorf("usage: kraai projects delete <project-id>")
		}
		return deleteProject(fs.Arg(1))
	default:
		return fmt.Errorf("unknown subcommand: %s\n\nUsage:\n  kraai projects [list]                  List projects in active workspace\n  kraai projects new [name]              Create a project\n  kraai projects rename <id> <name>      Rename a project\n  kraai projects delete <id>             Delete a project", sub)
	}
}

func listProjects() error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	if creds.WorkspaceID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	c := client.New(apiBaseURL, creds.Token)
	projects, err := c.ListProjects(creds.WorkspaceID)
	if err != nil {
		return fmt.Errorf("projects: %w", err)
	}

	for _, p := range projects {
		fmt.Printf("  %s  %s\n", p.ID, p.Name)
	}
	if len(projects) == 0 {
		fmt.Println("No projects found.")
	}
	return nil
}

func createProject(name string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	if creds.WorkspaceID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}
	if name == "" {
		return fmt.Errorf("usage: kraai projects new <name>")
	}

	c := client.New(apiBaseURL, creds.Token)
	proj, err := c.CreateProject(creds.WorkspaceID, name)
	if err != nil {
		return fmt.Errorf("projects new: %w", err)
	}

	fmt.Printf("Created project: %s (%s)\n", proj.Name, proj.ID)
	return nil
}

func renameProject(id, name string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	c := client.New(apiBaseURL, creds.Token)
	if err := c.RenameProject(id, name); err != nil {
		return fmt.Errorf("projects rename: %w", err)
	}
	fmt.Printf("Renamed project %s to %q\n", id, name)
	return nil
}

func deleteProject(id string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	c := client.New(apiBaseURL, creds.Token)
	if err := c.DeleteProject(id); err != nil {
		return fmt.Errorf("projects delete: %w", err)
	}
	fmt.Printf("Deleted project %s\n", id)
	return nil
}
