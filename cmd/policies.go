package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runPolicies(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kraai policies <subcommand>\n\nSubcommands:\n  list      List OPA policies in the active workspace\n  create    Create an OPA policy from a Rego file\n  delete    Delete an OPA policy")
	}

	switch args[0] {
	case "list":
		return policiesList(args[1:])
	case "create":
		return policiesCreate(args[1:])
	case "delete":
		return policiesDelete(args[1:])
	default:
		return fmt.Errorf("policies: unknown subcommand %q", args[0])
	}
}

func policiesList(args []string) error {
	_ = args // no flags needed
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	wsID := creds.WorkspaceID
	if wenv := os.Getenv("KRAAI_WORKSPACE_ID"); wenv != "" {
		wsID = wenv
	}
	if wsID == "" {
		return fmt.Errorf("no active workspace")
	}

	c := client.New(apiBaseURL, creds.Token)
	policies, err := c.ListPolicies(wsID)
	if err != nil {
		return fmt.Errorf("policies list: %w", err)
	}

	if len(policies) == 0 {
		fmt.Println("No policies found.")
		return nil
	}
	for _, p := range policies {
		enabled := "enabled"
		if !p.Enabled {
			enabled = "disabled"
		}
		fmt.Printf("  %s  %-20s  %s  %s\n", p.ID, p.Name, enabled, p.CreatedAt)
	}
	return nil
}

func policiesCreate(args []string) error {
	fs := flag.NewFlagSet("policies create", flag.ContinueOnError)
	name := fs.String("name", "", "Policy name (required)")
	regoFile := fs.String("file", "", "Rego source file (required)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" || *regoFile == "" {
		return fmt.Errorf("usage: kraai policies create --name <name> --file <policy.rego>")
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}
	wsID := creds.WorkspaceID
	if wenv := os.Getenv("KRAAI_WORKSPACE_ID"); wenv != "" {
		wsID = wenv
	}
	if wsID == "" {
		return fmt.Errorf("no active workspace")
	}

	data, err := os.ReadFile(*regoFile)
	if err != nil {
		return fmt.Errorf("policies create: read file: %w", err)
	}

	c := client.New(apiBaseURL, creds.Token)
	p, err := c.CreatePolicy(wsID, *name, string(data))
	if err != nil {
		return fmt.Errorf("policies create: %w", err)
	}
	fmt.Printf("Created policy: %s (%s)\n", p.Name, p.ID)
	return nil
}

func policiesDelete(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: kraai policies delete <policy-id>")
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}
	wsID := creds.WorkspaceID
	if wenv := os.Getenv("KRAAI_WORKSPACE_ID"); wenv != "" {
		wsID = wenv
	}
	if wsID == "" {
		return fmt.Errorf("no active workspace")
	}

	c := client.New(apiBaseURL, creds.Token)
	if err := c.DeletePolicy(wsID, args[0]); err != nil {
		return fmt.Errorf("policies delete: %w", err)
	}
	fmt.Printf("Deleted policy %s\n", args[0])
	return nil
}
