package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runTools(args []string) error {
	fs := flag.NewFlagSet("tools", flag.ContinueOnError)
	slug := fs.String("slug", "", "Route slug of the deployed MCP server")
	mcpURL := fs.String("url", "", "Full MCP endpoint URL (overrides --slug)")
	token := fs.String("token", "", "Bearer token for the MCP endpoint")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *slug == "" && *mcpURL == "" {
		return fmt.Errorf("usage: kraai tools --slug <slug> [--token <token>]\n       kraai tools --url <mcp-url> [--token <token>]")
	}

	// Build the MCP endpoint URL.
	endpoint := *mcpURL
	if endpoint == "" {
		endpoint = runtimeBaseURL() + "/mcp/" + *slug
	}

	c := client.NewMCPClient(endpoint, *token)
	tools, err := c.ListTools()
	if err != nil {
		return fmt.Errorf("tools: %w", err)
	}

	if len(tools) == 0 {
		fmt.Println("No tools found.")
		return nil
	}

	fmt.Printf("%d tools available:\n\n", len(tools))
	for _, t := range tools {
		fmt.Printf("  %s\n", t.Name)
		if t.Description != "" {
			fmt.Printf("    %s\n", t.Description)
		}
		if len(t.Parameters) > 0 {
			fmt.Printf("    params: %s\n", formatParams(t.Parameters))
		}
		fmt.Println()
	}
	return nil
}

// runtimeBaseURL derives the runtime base URL from the API base URL.
// Local: http://api.lvh.me → http://lvh.me
// Prod:  https://api.kraai.dev → https://mcp.kraai.dev
func runtimeBaseURL() string {
	if v := os.Getenv("KRAAI_RUNTIME_BASE_URL"); v != "" {
		return v
	}
	// Map from the API base URL.
	switch {
	case strings.Contains(apiBaseURL, "api.lvh.me"):
		return "http://lvh.me"
	case strings.Contains(apiBaseURL, "api.kraai.dev"):
		return "https://mcp.kraai.dev"
	default:
		// Best guess: strip "api." prefix.
		return strings.Replace(apiBaseURL, "api.", "", 1)
	}
}

func formatParams(params []client.MCPToolParam) string {
	parts := make([]string, 0, len(params))
	for _, p := range params {
		s := p.Name
		if p.Required {
			s += "*"
		}
		if p.Type != "" {
			s += ":" + p.Type
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}
