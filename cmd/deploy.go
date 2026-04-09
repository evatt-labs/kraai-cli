package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/evatt-labs/kraai-cli/internal/client"
)

func runDeploy(args []string) error {
	// Check for subcommands first.
	if len(args) > 0 && args[0] == "activate" {
		return runDeployActivate(args[1:])
	}

	// Separate flags from positional args so flags can appear in any position.
	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			// If the flag takes a value (next arg isn't a flag), consume it too.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && strings.Contains(args[i], "=") == false {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			posArgs = append(posArgs, args[i])
		}
	}

	fs := flag.NewFlagSet("deploy", flag.ContinueOnError)
	slug := fs.String("slug", "", "Route slug (default: derived from spec title)")
	name := fs.String("name", "", "Project name (default: derived from spec title)")
	wsID := fs.String("workspace", "", "Workspace ID (default: active workspace)")
	projID := fs.String("project-id", "", "Deploy to an existing project (skip project creation)")
	baseURL := fs.String("base-url", "", "Override upstream base URL from spec")
	fromURL := fs.String("from-url", "", "Fetch spec from this HTTPS URL instead of a local file")
	authConn := fs.String("auth-connection", "", "Attach an auth connection ID at publish time")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	if *fromURL == "" && len(posArgs) < 1 {
		return fmt.Errorf("usage: kraai deploy [flags] <spec-file>\n       kraai deploy --from-url <https://...> [flags]")
	}
	specFile := ""
	if len(posArgs) > 0 {
		specFile = posArgs[0]
	}

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	var data []byte
	var specTitle string
	if *fromURL != "" {
		specTitle = "server"
		if *slug == "" {
			// Derive slug from URL path
			parts := strings.Split(strings.TrimRight(*fromURL, "/"), "/")
			base := parts[len(parts)-1]
			if i := strings.LastIndexByte(base, '.'); i > 0 {
				base = base[:i]
			}
			*slug = toSlug(base)
		}
	} else {
		var readErr error
		data, readErr = os.ReadFile(specFile)
		if readErr != nil {
			return fmt.Errorf("deploy: read spec: %w", readErr)
		}
		specTitle = specTitleFromJSON(data, specFile)
		if *slug == "" {
			*slug = toSlug(specTitle)
		}
	}
	if *name == "" {
		if specTitle != "" {
			*name = specTitle
		} else {
			*name = "server"
		}
	}

	c := client.New(apiBaseURL, creds.Token)

	// Resolve workspace
	activeWS := creds.WorkspaceID
	if *wsID != "" {
		activeWS = *wsID
	}
	if wenv := os.Getenv("KRAAI_WORKSPACE_ID"); wenv != "" {
		activeWS = wenv
	}

	if activeWS == "" {
		// Auto-create a workspace
		fmt.Printf("Creating workspace...")
		wsName := workspaceNameFromEmail(creds.Email)
		ws, err := c.CreateWorkspace(wsName)
		if err != nil {
			return fmt.Errorf("deploy: create workspace: %w", err)
		}
		activeWS = ws.ID
		fmt.Printf(" %s\n", ws.Name)
	}

	// Resolve or create project
	var projectID string
	if *projID != "" {
		projectID = *projID
		fmt.Printf("Using existing project %s\n", projectID)
	} else {
		fmt.Printf("Creating project %q...", *name)
		proj, err := c.CreateProject(activeWS, *name)
		if err != nil {
			return fmt.Errorf("deploy: create project: %w", err)
		}
		projectID = proj.ID
		fmt.Printf(" done\n")
	}

	// Upload or fetch spec
	var src *client.APISource
	if *fromURL != "" {
		fmt.Printf("Fetching spec from URL...")
		var fetchErr error
		src, fetchErr = c.FetchSpec(projectID, *fromURL)
		if fetchErr != nil {
			return fmt.Errorf("deploy: fetch spec: %w", fetchErr)
		}
		fmt.Printf(" done (source: %s)\n", src.ID)
	} else {
		fmt.Printf("Uploading spec...")
		uploadURL := apiBaseURL + "/v1/projects/" + projectID + "/api-sources/upload"
		if *baseURL != "" {
			params := url.Values{}
			params.Set("base_url", *baseURL)
			uploadURL += "?" + params.Encode()
		}
		var uploadErr error
		src, uploadErr = c.UploadSpecRaw(projectID, data, uploadURL)
		if uploadErr != nil {
			return fmt.Errorf("deploy: upload spec: %w", uploadErr)
		}
		fmt.Printf(" done (source: %s)\n", src.ID)
	}

	// Poll until worker finishes parsing (up to 2 minutes)
	fmt.Printf("Processing spec")
	deadline := time.Now().Add(2 * time.Minute)
	ready := false
	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)
		fmt.Print(".")
		sources, err := c.GetSources(projectID)
		if err != nil {
			continue
		}
		for _, s := range sources {
			if s.ID == src.ID && s.IngestStatus == "ready" {
				ready = true
			}
		}
		if ready {
			break
		}
	}
	fmt.Println()
	if !ready {
		return fmt.Errorf("deploy: timed out waiting for spec processing")
	}

	// Check slug availability
	available, slugErr := c.CheckSlugAvailability(projectID, *slug)
	if slugErr == nil && !available {
		return fmt.Errorf("deploy: slug %q is already taken — use --slug to choose another", *slug)
	}

	// Publish
	fmt.Printf("Publishing as %q...", *slug)
	result, err := c.Publish(projectID, *slug, *authConn)
	if err != nil {
		return fmt.Errorf("deploy: publish: %w", err)
	}
	fmt.Println()
	fmt.Printf("\n✓ MCP server live at %s\n\n", result.MCPURL)
	printConnectInstructions(result)
	return nil
}

func printConnectInstructions(result *client.PublishResult) {
	switch result.WorkspacePlan {
	case "pro", "business":
		// Pro/Business: full OAuth 2.1 — Claude Desktop handles auth automatically.
		// Static token is still available as a fallback.
		fmt.Printf("  OAuth 2.1 is enabled. Add to Claude Desktop:\n")
		fmt.Printf("  {\n")
		fmt.Printf("    \"mcpServers\": {\n")
		fmt.Printf("      \"kraai\": { \"url\": \"%s\" }\n", result.MCPURL)
		fmt.Printf("    }\n")
		fmt.Printf("  }\n\n")
		if result.DeploymentToken != "" {
			fmt.Printf("  Static bearer token (fallback / CI use):\n")
			fmt.Printf("  %s\n\n", result.DeploymentToken)
			fmt.Printf("  Store it safely — it will not be shown again.\n\n")
		}
	default:
		// Free: static bearer token. User configures it manually in the MCP client.
		fmt.Printf("  Free plan — connect with a static bearer token.\n\n")
		if result.DeploymentToken != "" {
			fmt.Printf("  Token: %s\n\n", result.DeploymentToken)
			fmt.Printf("  Store it safely — it will not be shown again.\n\n")
			fmt.Printf("  Add to Claude Desktop:\n")
			fmt.Printf("  {\n")
			fmt.Printf("    \"mcpServers\": {\n")
			fmt.Printf("      \"kraai\": {\n")
			fmt.Printf("        \"url\": \"%s\",\n", result.MCPURL)
			fmt.Printf("        \"headers\": { \"Authorization\": \"Bearer %s\" }\n", result.DeploymentToken)
			fmt.Printf("      }\n")
			fmt.Printf("    }\n")
			fmt.Printf("  }\n\n")
			fmt.Printf("  Upgrade to Pro for OAuth 2.1 (no manual token config).\n\n")
		}
	}
}

// specTitleFromJSON extracts info.title from the OpenAPI spec JSON.
// Falls back to the file name (without extension) if parsing fails.
func specTitleFromJSON(data []byte, filename string) string {
	var doc struct {
		Info struct {
			Title string `json:"title"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &doc); err == nil && doc.Info.Title != "" {
		return doc.Info.Title
	}
	base := filename
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.LastIndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	return base
}

// toSlug converts a string to a lowercase hyphenated slug.
func toSlug(s string) string {
	s = strings.ToLower(s)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}

func runDeployActivate(args []string) error {
	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && !strings.Contains(args[i], "=") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			posArgs = append(posArgs, args[i])
		}
	}

	fs := flag.NewFlagSet("deploy activate", flag.ContinueOnError)
	projectID := fs.String("project", "", "Project ID (required if workspace has multiple projects)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	if len(posArgs) < 1 {
		return fmt.Errorf("usage: kraai deploy activate <deployment-id> [--project <id>]")
	}
	deploymentID := posArgs[0]

	creds, err := requireCreds()
	if err != nil {
		return err
	}

	c := client.New(apiBaseURL, creds.Token)

	// Resolve project ID if not provided.
	pid := *projectID
	if pid == "" {
		wsID := creds.WorkspaceID
		if wenv := os.Getenv("KRAAI_WORKSPACE_ID"); wenv != "" {
			wsID = wenv
		}
		projects, err := c.ListProjects(wsID)
		if err != nil {
			return fmt.Errorf("deploy activate: list projects: %w", err)
		}
		switch len(projects) {
		case 0:
			return fmt.Errorf("deploy activate: no projects in workspace")
		case 1:
			pid = projects[0].ID
		default:
			fmt.Fprintln(os.Stderr, "Multiple projects found. Specify one with --project <id>:")
			for _, p := range projects {
				fmt.Fprintf(os.Stderr, "  %s  %s\n", p.ID, p.Name)
			}
			os.Exit(1)
		}
	}

	fmt.Printf("Activating deployment %s...", deploymentID)
	result, err := c.ActivateDeployment(pid, deploymentID)
	if err != nil {
		return fmt.Errorf("deploy activate: %w", err)
	}
	fmt.Println()
	fmt.Printf("\n✓ Deployment activated at %s\n\n", result.MCPURL)
	printConnectInstructions(result)
	return nil
}

func workspaceNameFromEmail(email string) string {
	if i := strings.IndexByte(email, '@'); i > 0 {
		local := email[:i]
		parts := strings.Split(local, ".")
		for i, p := range parts {
			if len(p) > 0 {
				parts[i] = strings.ToUpper(p[:1]) + p[1:]
			}
		}
		return strings.Join(parts, " ") + "'s Workspace"
	}
	return "My Workspace"
}
