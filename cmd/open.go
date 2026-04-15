package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/evatt-labs/kraai-cli/internal/config"
)

func runConsole(args []string) error {
	fs := flag.NewFlagSet("console", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return openBrowserSafe(appBaseURL)
}

func runOpen(args []string) error {
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	var projectSlug string
	fs.StringVar(&projectSlug, "project", "", "Project slug to open")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	// If no project specified but there are positional args, treat first as slug
	if projectSlug == "" && fs.NArg() > 0 {
		projectSlug = fs.Arg(0)
	}

	targetURL := appBaseURL

	if projectSlug != "" {
		targetURL = fmt.Sprintf("%s/projects/%s", appBaseURL, projectSlug)
		// Check for specific sub-pages as second arg
		if fs.NArg() > 1 {
			subPage := fs.Arg(1)
			targetURL = fmt.Sprintf("%s/%s", targetURL, subPage)
		}
	} else {
		// Try to find the active workspace to deep link to it
		creds, _ := config.Load()
		if creds != nil && creds.WorkspaceID != "" {
			targetURL = fmt.Sprintf("%s/workspaces/%s", appBaseURL, creds.WorkspaceID)
		}
	}

	fmt.Printf("Opening %s...\n", targetURL)
	return openBrowserSafe(targetURL)
}
