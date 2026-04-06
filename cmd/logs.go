package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	projectID := fs.String("project", "", "Project ID (required)")
	workspaceID := fs.String("workspace", "", "Override active workspace")
	tail := fs.Int("tail", 50, "Number of log entries to show")
	follow := fs.Bool("follow", false, "Poll for new logs every 2 seconds")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	pid := *projectID
	if pid == "" {
		pid, err = resolveProjectID(creds, "", *workspaceID)
		if err != nil {
			return fmt.Errorf("logs: %w", err)
		}
	}

	c := client.New(apiBaseURL, creds.Token)

	if *follow {
		return followLogs(c, pid, *tail)
	}

	return printLogs(c, pid, *tail)
}

func printLogs(c *client.Client, projectID string, limit int) error {
	result, err := c.ListLogs(projectID, limit, "")
	if err != nil {
		return fmt.Errorf("logs: %w", err)
	}

	if len(result.Logs) == 0 {
		fmt.Println("No request logs found.")
		return nil
	}

	printLogEntries(result.Logs)
	return nil
}

func followLogs(c *client.Client, projectID string, initialLimit int) error {
	// Initial fetch.
	result, err := c.ListLogs(projectID, initialLimit, "")
	if err != nil {
		return fmt.Errorf("logs: %w", err)
	}

	printLogEntries(result.Logs)

	// Track the newest log ID for cursor-based polling.
	// Since logs are returned newest-first, we need to track
	// what we've already seen. We'll use the first (newest) ID
	// from each batch as the "already seen" marker.
	var lastSeenID string
	if len(result.Logs) > 0 {
		lastSeenID = result.Logs[0].ID
	}

	fmt.Fprintf(os.Stderr, "\n--- following (Ctrl+C to stop) ---\n\n")

	for {
		time.Sleep(2 * time.Second)

		// Fetch recent logs (small batch).
		result, err := c.ListLogs(projectID, 50, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "poll error: %v\n", err)
			continue
		}

		// Print only new entries (those newer than lastSeenID).
		var newEntries []client.RequestLog
		for _, log := range result.Logs {
			if log.ID == lastSeenID {
				break
			}
			newEntries = append(newEntries, log)
		}

		if len(newEntries) > 0 {
			// Print in chronological order (reverse of what we got).
			for i := len(newEntries) - 1; i >= 0; i-- {
				printLogEntry(newEntries[i])
			}
			lastSeenID = newEntries[0].ID
		}
	}
}

func printLogEntries(logs []client.RequestLog) {
	// Logs come newest-first; print in chronological order.
	for i := len(logs) - 1; i >= 0; i-- {
		printLogEntry(logs[i])
	}
}

func printLogEntry(log client.RequestLog) {
	toolName := "-"
	if log.ToolName != nil {
		toolName = *log.ToolName
	}
	statusCode := "-"
	if log.StatusCode != nil {
		statusCode = fmt.Sprintf("%d", *log.StatusCode)
	}
	latency := "-"
	if log.LatencyMs != nil {
		latency = fmt.Sprintf("%dms", *log.LatencyMs)
	}
	ts := log.CreatedAt
	if len(ts) > 19 {
		ts = ts[:19]
	}
	fmt.Printf("%s  %-30s  %s  %s\n", ts, toolName, statusCode, latency)
}
