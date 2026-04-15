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
	var serverSlug string
	fs.StringVar(&serverSlug, "server", "", "Server slug to open")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	// If no server specified but there are positional args, treat first as slug.
	if serverSlug == "" && fs.NArg() > 0 {
		serverSlug = fs.Arg(0)
	}

	targetURL := appBaseURL

	if serverSlug != "" {
		targetURL = fmt.Sprintf("%s/servers/%s", appBaseURL, serverSlug)
		// Check for specific sub-pages as second arg.
		if fs.NArg() > 1 {
			subPage := fs.Arg(1)
			targetURL = fmt.Sprintf("%s/%s", targetURL, subPage)
		}
	} else {
		// Try to find the active workspace to deep link to it.
		creds, _ := config.Load()
		if creds != nil && creds.WorkspaceID != "" {
			targetURL = fmt.Sprintf("%s/workspaces/%s", appBaseURL, creds.WorkspaceID)
		}
	}

	fmt.Printf("Opening %s...\n", targetURL)
	return openBrowserSafe(targetURL)
}
