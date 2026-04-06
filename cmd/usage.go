package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runUsage(args []string) error {
	fs := flag.NewFlagSet("usage", flag.ContinueOnError)
	projectID := fs.String("project", "", "Show usage for a specific project")
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

	// Project-level usage.
	if *projectID != "" {
		return printProjectUsage(c, *projectID)
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

	if len(usage.ByProject) > 0 {
		fmt.Println("\nBy project:")
		for _, p := range usage.ByProject {
			fmt.Printf("  %s  %d requests\n", p.ProjectID, p.Count)
		}
	}

	return nil
}

func printProjectUsage(c *client.Client, projectID string) error {
	usage, err := c.GetProjectUsage(projectID)
	if err != nil {
		return fmt.Errorf("usage: %w", err)
	}

	fmt.Printf("Project:   %s\n", usage.ProjectID)
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
