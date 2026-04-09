package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const version = "0.1.0"

var apiBaseURL = "https://api.kraai.dev"

func init() {
	if v := os.Getenv("KRAAI_API_BASE_URL"); v != "" {
		apiBaseURL = v
		parsed, err := url.Parse(v)
		if err == nil && parsed.Scheme == "http" &&
			parsed.Hostname() != "localhost" &&
			!strings.HasSuffix(parsed.Hostname(), ".lvh.me") {
			fmt.Fprintf(os.Stderr, "warning: KRAAI_API_BASE_URL uses http:// — credentials will be sent in cleartext\n")
		}
	} else if os.Getenv("KRAAI_ENV") == "local" {
		apiBaseURL = "http://api.lvh.me"
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Global flags
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--local" {
		apiBaseURL = "http://api.lvh.me"
		args = args[1:]
	}

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	cmd := args[0]
	rest := args[1:]

	var err error
	switch cmd {
	case "login":
		err = runLogin(rest)
	case "logout":
		err = runLogout(rest)
	case "whoami":
		err = runWhoami(rest)
	case "workspaces":
		err = runWorkspaces(rest)
	case "projects":
		err = runProjects(rest)
	case "plans":
		err = runPlans(rest)
	case "validate":
		err = runValidate(rest)
	case "deploy":
		err = runDeploy(rest)
	case "publish":
		err = runPublish(rest)
	case "deployments":
		err = runDeployments(rest)
	case "tools":
		err = runTools(rest)
	case "tokens":
		err = runTokens(rest)
	case "status":
		err = runStatus(rest)
	case "usage":
		err = runUsage(rest)
	case "logs":
		err = runLogs(rest)
	case "auth-connections":
		err = runAuthConnections(rest)
	case "workflows":
		err = runWorkflows(rest)
	case "policies":
		err = runPolicies(rest)
	case "approvals":
		err = runApprovals(rest)
	case "version", "--version", "-v":
		fmt.Printf("kraai version %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "kraai: unknown command %q\n\nRun 'kraai help' for usage.\n", cmd)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`Kraai CLI — turn APIs into hosted MCP servers.

Usage:
  kraai <command> [flags]

Commands:
  login              Authenticate via browser (Device Authorization Grant)
  logout             Revoke the active CLI token
  whoami             Print the currently authenticated user and workspace
  workspaces         List, create, rename, delete, or switch workspaces
  projects           List, create, rename, or delete projects
  plans              View or change workspace billing plan
  validate           Validate an OpenAPI spec file locally
  deploy             Upload (or fetch from URL) a spec and publish
  deploy activate    Switch to a specific deployment version
  publish            Publish the latest ready source for an existing project
  deployments        List deployments for a project
  tools              List MCP tools exposed by a deployed server
  tokens             Manage workspace API tokens
  usage              View request counts and quota usage
  logs               View recent MCP request logs
  status             Check deployment health and info
  auth-connections   Manage upstream API credentials
  workflows          Manage workflow definitions and runs
  policies           Manage OPA policies
  approvals          List, approve, or deny approval requests
  version            Print version

Flags:
  --local        Target http://api.lvh.me (local development stack)

Environment variables:
  KRAAI_API_BASE_URL   Override the API base URL
  KRAAI_TOKEN          Use this token instead of ~/.kraai/credentials
  KRAAI_WORKSPACE_ID   Override the active workspace

`)
}
