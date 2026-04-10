package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runPlans(args []string) error {
	sub := ""
	rest := args
	if len(args) > 0 {
		sub = args[0]
		rest = args[1:]
	}

	switch sub {
	case "", "list":
		return listPlans()
	case "set":
		return runPlansSet(rest)
	case "resume":
		return runPlansResume(rest)
	default:
		return fmt.Errorf("unknown subcommand: %s\n\nUsage:\n  kraai plans [list]                Show available plans\n  kraai plans set <plan> [--yes]    Switch to a plan (idempotent)\n  kraai plans resume                Undo a pending cancellation", sub)
	}
}

func runPlansSet(args []string) error {
	fs := flag.NewFlagSet("plans set", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	yes := fs.Bool("yes", false, "Skip interactive confirmation for downgrades")
	fs.BoolVar(yes, "y", false, "Shorthand for --yes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: kraai plans set <free|indie|builder|studio> [--yes]")
	}
	return setPlan(fs.Arg(0), *yes)
}

func runPlansResume(args []string) error {
	fs := flag.NewFlagSet("plans resume", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}
	if creds.WorkspaceID == "" {
		return fmt.Errorf("no active workspace — run 'kraai workspaces use <id>'")
	}

	c := client.New(apiBaseURL, creds.Token)
	result, err := c.ResumeBilling(creds.WorkspaceID)
	if err != nil {
		return fmt.Errorf("plans resume: %w", err)
	}

	fmt.Printf("✓ Subscription resumed — staying on %s.\n", result.Plan)
	return nil
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
		fmt.Printf("Resume with:  kraai plans resume\n")
	} else {
		fmt.Printf("Switch with:  kraai plans set <plan>\n")
	}
	return nil
}

// setPlan calls the idempotent PUT /v1/workspaces/:id/billing/plan
// endpoint and dispatches on the response shape rather than pre-computing
// upgrade/downgrade semantics client-side. The backend figures out the
// transition type; we only decide the UX around it:
//
//   - IsNoOp              → already on target (or implicit resume on a canceling ws)
//   - CheckoutURL != ""   → first-time subscribe: open browser
//   - EffectiveAt != nil  → cancel scheduled for that date; resumable until then
//   - otherwise           → paid→paid change applied immediately, Stripe prorates
//
// The downgrade confirmation prompt is purely client-side UX — it fires
// before the network call, based on a monthly-price comparison, to give
// the user a chance to back out of a destructive change.
func setPlan(targetPlan string, autoConfirm bool) error {
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

	var targetEnt *client.PlanEntitlements
	for i := range plans {
		if plans[i].Plan == targetPlan {
			targetEnt = &plans[i]
			break
		}
	}
	if targetEnt == nil {
		return fmt.Errorf("invalid plan %q — must be free, indie, builder, or studio", targetPlan)
	}

	// Fetch current workspace state upfront so we can detect a downgrade
	// before any state change and show a meaningful diff at the confirm prompt.
	ws, err := c.GetWorkspace(creds.WorkspaceID)
	if err != nil {
		return fmt.Errorf("plans set: get workspace: %w", err)
	}

	// Downgrade heuristic: strictly lower monthly price than current plan.
	// Catches paid→free (cancel at period end, resumable) and paid→paid
	// (immediate proration, entitlement loss).
	isDowngrade := targetEnt.MonthlyPriceCents < ws.Entitlements.MonthlyPriceCents
	if isDowngrade && !autoConfirm {
		if err := confirmDowngrade(ws, targetEnt); err != nil {
			return err
		}
	}

	result, err := c.SetPlan(creds.WorkspaceID, targetPlan)
	if err != nil {
		return fmt.Errorf("plans set: %w", err)
	}

	switch {
	case result.IsNoOp:
		// The backend treats SetPlan(current_plan) on a canceling workspace
		// as an implicit resume. Detect that by the pre-call status.
		if ws.BillingStatus == "canceling" {
			fmt.Printf("✓ Subscription resumed — staying on %s.\n", result.Plan)
			return nil
		}
		fmt.Printf("Already on %s — no changes needed.\n", result.Plan)
		return nil

	case result.CheckoutURL != "":
		// free → paid: hosted Stripe Checkout Session. The actual plan
		// change lands asynchronously via the checkout.session.completed
		// webhook; we don't poll for it here. "kraai plans list" will
		// reflect the new state once Stripe calls back.
		fmt.Printf("Opening Stripe checkout in your browser...\n")
		fmt.Printf("If it doesn't open, visit:\n  %s\n\n", result.CheckoutURL)
		if err := openBrowserSafe(result.CheckoutURL); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
		fmt.Println("Your plan will update once payment completes.")
		fmt.Println("Run `kraai plans list` to verify.")
		return nil

	case result.EffectiveAt != nil:
		// paid → free: cancel at period end. Paid features remain active
		// until EffectiveAt; the subscription.deleted webhook will flip
		// billing_plan to free at the period boundary.
		effective := result.EffectiveAt.Format("Jan 2, 2006")
		fmt.Printf("✓ Cancellation scheduled.\n")
		fmt.Printf("  You'll keep %s features until %s.\n", result.Previous, effective)
		fmt.Printf("  Run `kraai plans resume` any time before then to undo.\n")
		return nil

	default:
		// paid → paid: applied server-side via subscription.Update with
		// create_prorations. Customer sees the delta on next invoice.
		fmt.Printf("✓ Plan changed from %s to %s.\n", result.Previous, result.Plan)
		fmt.Printf("  Stripe will prorate the change on your next invoice.\n")
		return nil
	}
}

// confirmDowngrade prints a diff of the entitlement changes and blocks
// until the user types the workspace name (or aborts). It is intentionally
// strict: there is no y/N shortcut for destructive operations, and
// non-interactive callers must pass --yes rather than relying on stdin
// behavior.
func confirmDowngrade(ws *client.Workspace, target *client.PlanEntitlements) error {
	current := ws.Entitlements

	fmt.Println()
	fmt.Printf("⚠️  Downgrading workspace %q from %s to %s\n", ws.Name, current.Label, target.Label)
	fmt.Println()
	fmt.Println("Changes:")
	fmt.Printf("  Active servers:     %d  →  %d\n", current.ActiveHostedServers, target.ActiveHostedServers)
	fmt.Printf("  Member seats:       %d  →  %d\n", current.MemberSeats, target.MemberSeats)
	fmt.Printf("  Monthly requests:   %s  →  %s\n", formatNumber(current.IncludedRuntimeRequests), formatNumber(target.IncludedRuntimeRequests))
	fmt.Printf("  Log retention:      %dd  →  %dd\n", current.LogRetentionDays, target.LogRetentionDays)
	if current.SupportsClientOAuth && !target.SupportsClientOAuth {
		fmt.Println("  Client OAuth:       enabled  →  disabled")
	}
	if current.SupportsUpstreamOAuth && !target.SupportsUpstreamOAuth {
		fmt.Println("  Upstream OAuth:     enabled  →  disabled")
	}
	if current.SupportsRollback && !target.SupportsRollback {
		fmt.Println("  Rollback:           enabled  →  disabled")
	}
	if current.SupportsCustomDomains && !target.SupportsCustomDomains {
		fmt.Println("  Custom domains:     enabled  →  disabled")
	}
	fmt.Println()

	if target.Plan == "free" {
		fmt.Println("This schedules a cancellation at the end of your current billing period.")
		fmt.Println("Your paid features stay active until then.")
		fmt.Println("Run `kraai plans resume` any time before the period end to undo.")
	} else {
		fmt.Println("The change is applied immediately. Stripe will prorate the difference")
		fmt.Println("on your next invoice. Features above the new limits will be restricted.")
	}
	fmt.Println()
	fmt.Println("Pass --yes to skip this prompt in scripts.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	answer := prompt(reader, fmt.Sprintf("Type the workspace name (%s) to confirm: ", ws.Name))
	if answer != ws.Name {
		return fmt.Errorf("confirmation did not match — aborted (nothing changed)")
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
