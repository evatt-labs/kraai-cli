package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/evatt-labs/kraai-cli/internal/config"
)

func runConsole(args []string) error {
	fs := flag.NewFlagSet("console", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	return openBrowser(appBaseURL)
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
	return openBrowser(targetURL)
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // "linux", "freebsd", etc.
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}
