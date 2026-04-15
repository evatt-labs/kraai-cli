package main

import (
	"fmt"
	"strings"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runPlans(args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "", "list":
		return listPlans()
	default:
		return fmt.Errorf("unknown subcommand: %s\n\nUsage:\n  kraai plans [list]    Show available plans\n\nPlan changes (subscribe, switch, cancel, resume) live in the web UI at app.kraai.dev/billing.", sub)
	}
}

func listPlans() error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	if creds.WorkspaceID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	c := client.New(apiBaseURL, creds.Token)
	ws, err := c.GetWorkspace(creds.WorkspaceID)
	if err != nil {
		return fmt.Errorf("plans: get workspace: %w", err)
	}
	plans, err := c.ListPlans()
	if err != nil {
		return fmt.Errorf("plans: list plans: %w", err)
	}

	fmt.Printf("Plans for workspace %q:\n\n", ws.Name)
	for _, p := range plans {
		marker := "  "
		if p.Plan == ws.BillingPlan {
			marker = "* "
		}
		price := "$0"
		if p.MonthlyPriceCents > 0 {
			price = fmt.Sprintf("$%d/mo", p.MonthlyPriceCents/100)
		}
		fmt.Printf("%s%-10s %-8s %s\n", marker, p.Label, price, p.Summary)
	}
	fmt.Println()
	fmt.Printf("Current plan: %s", ws.Entitlements.Label)
	if ws.BillingStatus == "canceling" {
		if ws.CancellationEffectiveAt != nil {
			fmt.Printf(" (canceling — drops to Free on %s)", ws.CancellationEffectiveAt.Format("Jan 2, 2006"))
		} else {
			fmt.Printf(" (canceling)")
		}
	}
	fmt.Println()
	fmt.Printf("Limits:       %d servers, %d seats, %s requests/mo, %d-day logs\n",
		ws.Entitlements.ActiveHostedServers,
		ws.Entitlements.MemberSeats,
		formatNumber(ws.Entitlements.IncludedRuntimeRequests),
		ws.Entitlements.LogRetentionDays,
	)
	if ws.Entitlements.UpgradeTargetPlan != "" {
		fmt.Printf("Next upgrade: %s\n", ws.Entitlements.UpgradeTargetPlan)
	}
	if ws.BillingStatus == "canceling" {
		fmt.Printf("Resume at:    https://app.kraai.dev/workspaces/%s/billing\n", creds.WorkspaceID)
	} else {
		fmt.Printf("Change at:    https://app.kraai.dev/workspaces/%s/billing\n", creds.WorkspaceID)
	}
	return nil
}

func formatNumber(v int64) string {
	raw := fmt.Sprintf("%d", v)
	if len(raw) <= 3 {
		return raw
	}
	var parts []string
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	return strings.Join(parts, ",")
}
