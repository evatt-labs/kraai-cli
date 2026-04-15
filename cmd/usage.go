package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runUsage(args []string) error {
	fs := flag.NewFlagSet("usage", flag.ContinueOnError)
	serverID := fs.String("server", "", "Show usage for a specific server")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)

	// Server-level usage.
	if *serverID != "" {
		return printServerUsage(c, *serverID)
	}

	// Workspace-level usage.
	wsID := resolveWorkspace(creds.WorkspaceID, *workspaceID)
	if wsID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	usage, err := c.GetWorkspaceUsage(wsID)
	if err != nil {
		return fmt.Errorf("usage: %w", err)
	}

	fmt.Printf("Workspace: %s\n", usage.WorkspaceID)
	fmt.Printf("Plan:      %s\n", usage.Entitlements.Label)
	fmt.Printf("Requests:  %d / %d\n", usage.TotalCount, usage.PlanLimit)
	fmt.Printf("Period:    %s to %s\n", formatDate(usage.PeriodStart), formatDate(usage.PeriodEnd))
	fmt.Printf("Includes:  %d servers, %d seats, %d-day logs\n", usage.Entitlements.ActiveHostedServers, usage.Entitlements.MemberSeats, usage.Entitlements.LogRetentionDays)

	if len(usage.ByServer) > 0 {
		fmt.Println("\nBy server:")
		for _, s := range usage.ByServer {
			fmt.Printf("  %s  %d requests\n", s.ServerID, s.Count)
		}
	}

	return nil
}

func printServerUsage(c *client.Client, serverID string) error {
	usage, err := c.GetServerUsage(serverID)
	if err != nil {
		return fmt.Errorf("usage: %w", err)
	}

	fmt.Printf("Server:    %s\n", usage.ServerID)
	fmt.Printf("Plan:      %s\n", usage.Entitlements.Label)
	fmt.Printf("Requests:  %d / %d (workspace limit)\n", usage.Count, usage.PlanLimit)
	fmt.Printf("Period:    %s to %s\n", formatDate(usage.PeriodStart), formatDate(usage.PeriodEnd))
	fmt.Printf("Retention: %d-day logs\n", usage.Entitlements.LogRetentionDays)
	return nil
}

// formatDate trims a timestamp to just the date portion.
func formatDate(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}
