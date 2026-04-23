package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runWorkflows(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kraai workflows <subcommand>\n\nSubcommands:\n  list              List workflow definitions for a server\n  create            Create a workflow definition\n  delete            Delete a workflow definition\n  trigger           Trigger a workflow run\n  runs              List runs for a workflow definition\n  status            Get status and steps for a run\n  cancel            Cancel a running workflow")
	}

	switch args[0] {
	case "list":
		return workflowsList(args[1:])
	case "create":
		return workflowsCreate(args[1:])
	case "delete":
		return workflowsDelete(args[1:])
	case "trigger":
		return workflowsTrigger(args[1:])
	case "runs":
		return workflowsRuns(args[1:])
	case "status":
		return workflowsStatus(args[1:])
	case "cancel":
		return workflowsCancel(args[1:])
	default:
		return fmt.Errorf("workflows: unknown subcommand %q", args[0])
	}
}

func workflowsList(args []string) error {
	fs := flag.NewFlagSet("workflows list", flag.ContinueOnError)
	serverID := fs.String("server", "", "Server ID")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}
	pid, wsID, err := resolveServerID(creds, *serverID, "")
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)
	defs, err := c.ListWorkflowDefinitions(wsID, pid)
	if err != nil {
		return fmt.Errorf("workflows list: %w", err)
	}

	if len(defs) == 0 {
		fmt.Println("No workflow definitions found.")
		return nil
	}
	for _, d := range defs {
		fmt.Printf("  %s  %s  %s\n", d.ID, d.Name, d.CreatedAt)
	}
	return nil
}

func workflowsCreate(args []string) error {
	fs := flag.NewFlagSet("workflows create", flag.ContinueOnError)
	serverID := fs.String("server", "", "Server ID")
	name := fs.String("name", "", "Workflow name (required)")
	desc := fs.String("description", "", "Workflow description")
	defFile := fs.String("file", "", "JSON file containing the workflow definition (required)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" || *defFile == "" {
		return fmt.Errorf("usage: kraai workflows create --name <name> --file <definition.json> [--server <id>]")
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}
	pid, wsID, err := resolveServerID(creds, *serverID, "")
	if err != nil {
		return err
	}

	data, err := os.ReadFile(*defFile)
	if err != nil {
		return fmt.Errorf("workflows create: read file: %w", err)
	}
	if !json.Valid(data) {
		return fmt.Errorf("workflows create: %s is not valid JSON", *defFile)
	}

	c := client.New(apiBaseURL, creds.Token)
	d, err := c.CreateWorkflowDefinition(wsID, pid, *name, *desc, data)
	if err != nil {
		return fmt.Errorf("workflows create: %w", err)
	}
	fmt.Printf("Created workflow: %s (%s)\n", d.Name, d.ID)
	return nil
}

func workflowsDelete(args []string) error {
	fs := flag.NewFlagSet("workflows delete", flag.ContinueOnError)
	serverID := fs.String("server", "", "Server ID")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: kraai workflows delete <definition-id> [--server <id>]")
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}
	pid, wsID, err := resolveServerID(creds, *serverID, "")
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)
	if err := c.DeleteWorkflowDefinition(wsID, pid, fs.Arg(0)); err != nil {
		return fmt.Errorf("workflows delete: %w", err)
	}
	fmt.Printf("Deleted workflow definition %s\n", fs.Arg(0))
	return nil
}

func workflowsTrigger(args []string) error {
	fs := flag.NewFlagSet("workflows trigger", flag.ContinueOnError)
	serverID := fs.String("server", "", "Server ID")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: kraai workflows trigger <definition-id> [--server <id>]")
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}
	pid, wsID, err := resolveServerID(creds, *serverID, "")
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)
	run, err := c.TriggerWorkflowRun(wsID, pid, fs.Arg(0))
	if err != nil {
		return fmt.Errorf("workflows trigger: %w", err)
	}
	fmt.Printf("Triggered run %s (status: %s)\n", run.ID, run.Status)
	return nil
}

func workflowsRuns(args []string) error {
	fs := flag.NewFlagSet("workflows runs", flag.ContinueOnError)
	serverID := fs.String("server", "", "Server ID")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: kraai workflows runs <definition-id> [--server <id>]")
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}
	pid, wsID, err := resolveServerID(creds, *serverID, "")
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)
	runs, err := c.ListWorkflowRuns(wsID, pid, fs.Arg(0))
	if err != nil {
		return fmt.Errorf("workflows runs: %w", err)
	}

	if len(runs) == 0 {
		fmt.Println("No runs found.")
		return nil
	}
	for _, r := range runs {
		fmt.Printf("  %s  %-10s  %s\n", r.ID, r.Status, r.CreatedAt)
	}
	return nil
}

func workflowsStatus(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: kraai workflows status <run-id>")
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)
	run, err := c.GetWorkflowRun(args[0])
	if err != nil {
		return fmt.Errorf("workflows status: %w", err)
	}

	fmt.Printf("Run %s\n", run.ID)
	fmt.Printf("  Status:     %s\n", run.Status)
	if run.StartedAt != nil {
		fmt.Printf("  Started:    %s\n", *run.StartedAt)
	}
	if run.CompletedAt != nil {
		fmt.Printf("  Completed:  %s\n", *run.CompletedAt)
	}
	if run.FailureReason != nil {
		fmt.Printf("  Failure:    %s\n", *run.FailureReason)
	}

	steps, err := c.GetWorkflowRunSteps(args[0])
	if err != nil {
		return fmt.Errorf("workflows status: get steps: %w", err)
	}
	if len(steps) > 0 {
		fmt.Printf("\n  Steps:\n")
		for _, s := range steps {
			fmt.Printf("    %s  %-12s  %-10s  attempt %d\n", s.StepKey, s.StepKind, s.State, s.Attempt)
		}
	}
	return nil
}

func workflowsCancel(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: kraai workflows cancel <run-id>")
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)
	if err := c.CancelWorkflowRun(args[0]); err != nil {
		return fmt.Errorf("workflows cancel: %w", err)
	}
	fmt.Printf("Cancelled run %s\n", args[0])
	return nil
}

