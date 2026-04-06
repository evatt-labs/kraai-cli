package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: kraai validate <file>")
	}

	path := fs.Arg(0)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("validate: read file: %w", err)
	}

	// Basic structural check: must be valid JSON and have an "openapi" field.
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		fmt.Fprintf(os.Stderr, "✗ Invalid JSON: %v\n", err)
		os.Exit(1)
	}

	openAPIVersion, ok := doc["openapi"].(string)
	if !ok {
		fmt.Fprintln(os.Stderr, "✗ Missing or invalid 'openapi' field")
		os.Exit(1)
	}

	paths, _ := doc["paths"].(map[string]any)
	opCount := 0
	for _, pathItem := range paths {
		if item, ok := pathItem.(map[string]any); ok {
			for _, method := range []string{"get", "post", "put", "patch", "delete", "head", "options"} {
				if _, exists := item[method]; exists {
					opCount++
				}
			}
		}
	}

	info, _ := doc["info"].(map[string]any)
	title, _ := info["title"].(string)

	fmt.Printf("✓ Valid OpenAPI %s spec\n", openAPIVersion)
	if title != "" {
		fmt.Printf("  Title:      %s\n", title)
	}
	fmt.Printf("  Operations: %d\n", opCount)

	// Check for security schemes hint
	components, _ := doc["components"].(map[string]any)
	secSchemes, _ := components["securitySchemes"].(map[string]any)
	if len(secSchemes) > 0 {
		for name, scheme := range secSchemes {
			if s, ok := scheme.(map[string]any); ok {
				t, _ := s["type"].(string)
				fmt.Printf("  Auth hint:  %s (%s)\n", t, name)
			}
		}
	}

	return nil
}
