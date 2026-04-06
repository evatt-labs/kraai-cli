package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runTokens(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "create":
			return runTokensCreate(args[1:])
		case "revoke":
			return runTokensRevoke(args[1:])
		}
	}
	return runTokensList(args)
}

func runTokensList(args []string) error {
	fs := flag.NewFlagSet("tokens", flag.ContinueOnError)
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
	tokens, err := c.ListAPITokens(wsID)
	if err != nil {
		return fmt.Errorf("tokens: %w", err)
	}

	if len(tokens) == 0 {
		fmt.Println("No API tokens found.")
		return nil
	}

	for _, t := range tokens {
		lastUsed := "never"
		if t.LastUsedAt != nil {
			lastUsed = *t.LastUsedAt
		}
		fmt.Printf("  %s  %-20s  %s…  created %s  last used %s\n",
			t.ID, t.Name, t.Prefix, t.CreatedAt[:10], lastUsed)
	}
	return nil
}

func runTokensCreate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: kraai tokens create <name>")
	}
	name := args[0]

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	wsID := resolveWorkspace(creds.WorkspaceID, "")
	if wsID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	c := client.New(apiBaseURL, creds.Token)
	result, err := c.CreateAPIToken(wsID, name)
	if err != nil {
		return fmt.Errorf("tokens create: %w", err)
	}

	fmt.Printf("✓ Token created: %s (%s)\n\n", result.Token.Name, result.Token.ID)
	fmt.Printf("  %s\n\n", result.RawToken)
	fmt.Printf("  Store it safely — it will not be shown again.\n")
	return nil
}

func runTokensRevoke(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: kraai tokens revoke <token-id>")
	}
	tokenID := args[0]

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	wsID := resolveWorkspace(creds.WorkspaceID, "")
	if wsID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	c := client.New(apiBaseURL, creds.Token)
	if err := c.RevokeToken(wsID, tokenID); err != nil {
		return fmt.Errorf("tokens revoke: %w", err)
	}

	fmt.Printf("✓ Token %s revoked.\n", tokenID)
	return nil
}

// resolveWorkspace returns the effective workspace ID, preferring explicit
// override, then env var, then the stored credential.
func resolveWorkspace(storedWSID, flagWSID string) string {
	if flagWSID != "" {
		return flagWSID
	}
	if wenv := os.Getenv("KRAAI_WORKSPACE_ID"); wenv != "" {
		return wenv
	}
	return storedWSID
}
