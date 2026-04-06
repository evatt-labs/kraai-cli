package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
	"github.com/evatt-labs/kraai-cli/internal/config"
)

func runLogout(args []string) error {
	fs := flag.NewFlagSet("logout", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	creds, err := config.Load()
	if err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	if creds == nil {
		fmt.Println("Not logged in.")
		return nil
	}

	// Revoke the API token server-side if we have the ID.
	if creds.TokenID != "" && creds.WorkspaceID != "" {
		c := client.New(apiBaseURL, creds.Token)
		if err := c.RevokeToken(creds.WorkspaceID, creds.TokenID); err != nil {
			// Non-fatal: token may already be revoked or the server may be unreachable.
			fmt.Fprintf(os.Stderr, "warning: could not revoke token server-side: %v\n", err)
		}
	}

	if err := config.Delete(); err != nil {
		return fmt.Errorf("logout: delete credentials: %w", err)
	}

	fmt.Printf("Logged out (%s).\n", creds.Email)
	return nil
}
