package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/evatt-labs/kraai-cli/internal/client"
	"github.com/evatt-labs/kraai-cli/internal/config"
)

func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	c := client.New(apiBaseURL, "")

	flow, err := c.InitiateDeviceFlow()
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	fmt.Printf("\nOpening Kraai in your browser...\n")
	fmt.Printf("If it doesn't open, visit: %s\n", flow.VerificationURI)
	fmt.Printf("Your code: %s\n\n", flow.UserCode)

	if err := openBrowserSafe(flow.VerificationURI); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	fmt.Printf("Waiting for authorization")

	interval := time.Duration(flow.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}

	deadline := time.Now().Add(time.Duration(flow.ExpiresIn) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(interval)
		fmt.Print(".")

		res, err := c.PollDeviceToken(flow.DeviceCode)
		if err != nil {
			return fmt.Errorf("login: poll: %w", err)
		}

		switch res.Error {
		case "":
			// Success
			fmt.Println()
			creds := &config.Credentials{
				Token:         res.Token,
				TokenID:       res.TokenID,
				WorkspaceID:   res.WorkspaceID,
				WorkspaceName: res.WorkspaceName,
				Email:         res.Email,
				CreatedAt:     time.Now(),
			}
			if err := config.Save(creds); err != nil {
				return fmt.Errorf("login: save credentials: %w", err)
			}
			fmt.Printf("✓ Logged in as %s\n\n", res.Email)
			fmt.Printf("  Workspace:  %s\n", res.WorkspaceName)
			fmt.Printf("  ID:         %s\n", res.WorkspaceID)

			authed := client.New(apiBaseURL, res.Token)
			servers, err := authed.ListServers(res.WorkspaceID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  warning: could not fetch servers: %v\n", err)
			} else if len(servers) > 0 {
				fmt.Printf("\n  Servers:\n")
				for _, s := range servers {
					fmt.Printf("    %-30s %s\n", s.Name, s.ID)
				}
			}
			fmt.Println()
			return nil
		case "authorization_pending":
			// Keep polling
		case "expired_token":
			fmt.Println()
			return fmt.Errorf("login: code expired — run 'kraai login' again")
		case "access_denied":
			fmt.Println()
			return fmt.Errorf("login: authorization denied")
		default:
			fmt.Println()
			return fmt.Errorf("login: unexpected error: %s", res.Error)
		}
	}

	fmt.Println()
	return fmt.Errorf("login: timed out waiting for authorization")
}

func openBrowserSafe(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		scheme := ""
		if u != nil {
			scheme = u.Scheme
		}
		return fmt.Errorf("refusing to open URL with scheme %q", scheme)
	}
	openBrowser(rawURL)
	return nil
}

func openBrowser(rawURL string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		return
	}
	cmd.Stderr = nil
	// Ignore errors — the user has the URL printed above.
	_ = cmd.Start()
}
