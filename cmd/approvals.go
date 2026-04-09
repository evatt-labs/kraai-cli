package main

import (
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runApprovals(args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "", "list":
		return approvalsList(false)
	case "pending":
		return approvalsList(true)
	case "approve":
		if len(args) < 2 {
			return fmt.Errorf("usage: kraai approvals approve <approval-id>")
		}
		return approvalsAct(args[1], true)
	case "deny":
		if len(args) < 2 {
			return fmt.Errorf("usage: kraai approvals deny <approval-id>")
		}
		return approvalsAct(args[1], false)
	default:
		return fmt.Errorf("approvals: unknown subcommand %q\n\nUsage:\n  kraai approvals [list]            List all approvals\n  kraai approvals pending           List pending approvals\n  kraai approvals approve <id>      Approve a request\n  kraai approvals deny <id>         Deny a request", sub)
	}
}

func approvalsList(pendingOnly bool) error {
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
	var approvals []client.ApprovalRequest
	if pendingOnly {
		approvals, err = c.ListPendingApprovals(wsID)
	} else {
		approvals, err = c.ListApprovals(wsID)
	}
	if err != nil {
		return fmt.Errorf("approvals: %w", err)
	}

	if len(approvals) == 0 {
		if pendingOnly {
			fmt.Println("No pending approvals.")
		} else {
			fmt.Println("No approvals found.")
		}
		return nil
	}
	for _, a := range approvals {
		fmt.Printf("  %s  %-10s  %-20s  %s  %s\n", a.ID, a.Status, a.Action, a.ResourceID, a.CreatedAt)
	}
	return nil
}

func approvalsAct(approvalID string, approve bool) error {
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
	if approve {
		if err := c.ApproveRequest(wsID, approvalID); err != nil {
			return fmt.Errorf("approvals approve: %w", err)
		}
		fmt.Printf("Approved %s\n", approvalID)
	} else {
		if err := c.DenyRequest(wsID, approvalID); err != nil {
			return fmt.Errorf("approvals deny: %w", err)
		}
		fmt.Printf("Denied %s\n", approvalID)
	}
	return nil
}
