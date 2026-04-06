package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/evatt-labs/kraai-cli/internal/client"
	"github.com/evatt-labs/kraai-cli/internal/config"
)

func runPlans(args []string) error {
	fs := flag.NewFlagSet("plans", flag.ContinueOnError)
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
		return listPlans()
	case "set":
		if fs.NArg() < 2 {
			return fmt.Errorf("usage: kraai plans set <free|builder|studio>")
		}
		return setPlan(fs.Arg(1))
	default:
		return fmt.Errorf("unknown subcommand: %s\n\nUsage:\n  kraai plans [list]           Show available plans\n  kraai plans set <plan>       Switch to a plan", sub)
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
	fmt.Printf("Current plan: %s\n", ws.Entitlements.Label)
	fmt.Printf("Limits:       %d servers, %d seats, %s requests/mo, %d-day logs\n",
		ws.Entitlements.ActiveHostedServers,
		ws.Entitlements.MemberSeats,
		formatNumber(ws.Entitlements.IncludedRuntimeRequests),
		ws.Entitlements.LogRetentionDays,
	)
	if ws.Entitlements.UpgradeTargetPlan != "" {
		fmt.Printf("Next upgrade: %s\n", ws.Entitlements.UpgradeTargetPlan)
	}
	fmt.Printf("Switch with:  kraai plans set <plan>\n")
	return nil
}

func setPlan(targetPlan string) error {
	creds, err := requireCreds()
	if err != nil {
		return err
	}
	if creds.WorkspaceID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	c := client.New(apiBaseURL, creds.Token)
	plans, err := c.ListPlans()
	if err != nil {
		return fmt.Errorf("plans set: list plans: %w", err)
	}

	valid := false
	for _, p := range plans {
		if p.Plan == targetPlan {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid plan %q — must be free, builder, or studio", targetPlan)
	}

	result, err := c.CreateCheckout(creds.WorkspaceID, targetPlan)
	if err != nil {
		return fmt.Errorf("plans set: %w", err)
	}

	if result.IsNoOp {
		fmt.Printf("Already on %s — no changes needed.\n", targetPlan)
		return nil
	}

	if result.IsFree {
		fmt.Printf("✓ Downgraded to free plan.\n")
		return nil
	}

	fmt.Printf("Opening Stripe checkout in your browser...\n")
	fmt.Printf("If it doesn't open, visit:\n  %s\n\n", result.URL)
	if err := openBrowserSafe(result.URL); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	fmt.Printf("Waiting for payment")
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)
		fmt.Print(".")

		ws, err := c.GetWorkspace(creds.WorkspaceID)
		if err != nil {
			continue
		}
		if ws.BillingPlan == targetPlan {
			fmt.Println()
			fmt.Printf("\n✓ Workspace upgraded to %s!\n", ws.Entitlements.Label)

			creds.WorkspaceName = ws.Name
			_ = config.Save(creds)
			return nil
		}
	}

	fmt.Println()
	return fmt.Errorf("timed out waiting for payment confirmation — check your Stripe dashboard")
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
