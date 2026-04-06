package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/config"
)

func runWhoami(args []string) error {
	fs := flag.NewFlagSet("whoami", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	creds, err := config.Load()
	if err != nil {
		return fmt.Errorf("whoami: %w", err)
	}
	if creds == nil {
		fmt.Fprintln(os.Stderr, "Not logged in. Run 'kraai login'.")
		os.Exit(1)
	}

	fmt.Printf("Email:     %s\n", creds.Email)
	fmt.Printf("Workspace: %s (%s)\n", creds.WorkspaceName, creds.WorkspaceID)
	return nil
}
