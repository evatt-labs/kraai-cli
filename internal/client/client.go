package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is a thin wrapper around the Kraai API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				req.Header.Del("Authorization")
				if len(via) > 3 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (c *Client) do(method, path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("client: marshal: %w", err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, r)
	if err != nil {
		return nil, fmt.Errorf("client: request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.httpClient.Do(req)
}

// apiError extracts a human-friendly error message from an API error response.
func apiError(statusCode int, body []byte) error {
	var apiErr struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
		return fmt.Errorf("%d %s", statusCode, apiErr.Message)
	}
	return fmt.Errorf("api error %d: %s", statusCode, string(body))
}

// checkStatus reads the response body and returns an error if status >= 400.
func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return apiError(resp.StatusCode, b)
	}
	return nil
}

func (c *Client) decode(resp *http.Response, v any) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return apiError(resp.StatusCode, b)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// --- Device Auth ---

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func (c *Client) InitiateDeviceFlow() (*DeviceCodeResponse, error) {
	resp, err := c.do("POST", "/oauth/device/authorize", map[string]string{})
	if err != nil {
		return nil, err
	}
	var out DeviceCodeResponse
	return &out, c.decode(resp, &out)
}

type DeviceTokenResponse struct {
	Token         string `json:"token"`
	TokenID       string `json:"token_id"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	Email         string `json:"email"`
	Error         string `json:"error"`
}

func (c *Client) PollDeviceToken(deviceCode string) (*DeviceTokenResponse, error) {
	resp, err := c.do("POST", "/oauth/device/token", map[string]string{
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out DeviceTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("client: decode poll: %w", err)
	}
	return &out, nil
}

// --- Workspaces ---

type Workspace struct {
	ID                      string           `json:"id"`
	Name                    string           `json:"name"`
	BillingPlan             string           `json:"billing_plan"`
	BillingStatus           string           `json:"billing_status"`
	OwnerID                 string           `json:"owner_id"`
	CancellationEffectiveAt *time.Time       `json:"cancellation_effective_at,omitempty"`
	Entitlements            PlanEntitlements `json:"entitlements"`
}

type PlanEntitlements struct {
	Plan                    string `json:"plan"`
	Label                   string `json:"label"`
	Summary                 string `json:"summary"`
	MonthlyPriceCents       int64  `json:"monthly_price_cents"`
	RequiresPayment         bool   `json:"requires_payment"`
	ActiveHostedServers     int    `json:"active_hosted_servers"`
	MemberSeats             int    `json:"member_seats"`
	IncludedRuntimeRequests int64  `json:"included_runtime_requests"`
	LogRetentionDays        int    `json:"log_retention_days"`
	SupportsClientOAuth     bool   `json:"supports_client_oauth"`
	SupportsUpstreamOAuth   bool   `json:"supports_upstream_oauth"`
	SupportsRollback        bool   `json:"supports_rollback"`
	SupportsCustomDomains   bool   `json:"supports_custom_domains"`
	UpgradeTargetPlan       string `json:"upgrade_target_plan,omitempty"`
}

func (c *Client) ListWorkspaces() ([]Workspace, error) {
	resp, err := c.do("GET", "/v1/workspaces", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	return out.Workspaces, c.decode(resp, &out)
}

// --- API Tokens ---

type APIToken struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

type CreateAPITokenResult struct {
	Token    APIToken `json:"token"`
	RawToken string   `json:"raw_token"`
}

func (c *Client) ListAPITokens(workspaceID string) ([]APIToken, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/api-tokens", workspaceID), nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		APITokens []APIToken `json:"api_tokens"`
	}
	return out.APITokens, c.decode(resp, &out)
}

func (c *Client) CreateAPIToken(workspaceID, name string) (*CreateAPITokenResult, error) {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/api-tokens", workspaceID), map[string]string{"name": name})
	if err != nil {
		return nil, err
	}
	var out CreateAPITokenResult
	return &out, c.decode(resp, &out)
}

func (c *Client) RevokeToken(workspaceID, tokenID string) error {
	resp, err := c.do("DELETE", fmt.Sprintf("/v1/workspaces/%s/api-tokens/%s", workspaceID, tokenID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// --- Workspaces (create) ---

func (c *Client) CreateWorkspace(name string) (*Workspace, error) {
	resp, err := c.do("POST", "/v1/workspaces", map[string]string{"name": name})
	if err != nil {
		return nil, err
	}
	var out Workspace
	return &out, c.decode(resp, &out)
}

// --- Servers ---

type Server struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
}

func (c *Client) ListServers(workspaceID string) ([]Server, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/servers", workspaceID), nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Servers []Server `json:"servers"`
	}
	return out.Servers, c.decode(resp, &out)
}

func (c *Client) CreateServer(workspaceID, name string) (*Server, error) {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/servers", workspaceID), map[string]string{"name": name})
	if err != nil {
		return nil, err
	}
	var out Server
	return &out, c.decode(resp, &out)
}

// --- API Sources ---

type APISource struct {
	ID                  string  `json:"id"`
	IngestStatus        string  `json:"ingest_status"`
	IngestFailureReason *string `json:"ingest_failure_reason,omitempty"`
}

func (c *Client) UploadSpec(workspaceID, serverID string, data []byte, filename string) (*APISource, error) {
	return c.UploadSpecRaw(workspaceID, serverID, data, c.baseURL+"/v1/workspaces/"+workspaceID+"/servers/"+serverID+"/api-sources/upload")
}

// UploadSpecRaw posts to a fully constructed URL (allows ?base_url= query param).
func (c *Client) UploadSpecRaw(workspaceID, serverID string, data []byte, fullURL string) (*APISource, error) {
	req, err := http.NewRequest("POST", fullURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("client: upload spec: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: upload spec: %w", err)
	}
	var out APISource
	return &out, c.decode(resp, &out)
}

func (c *Client) GetSources(workspaceID, serverID string) ([]APISource, error) {
	resp, err := c.do("GET", "/v1/workspaces/"+workspaceID+"/servers/"+serverID+"/api-sources", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		APISources []APISource `json:"api_sources"`
	}
	return out.APISources, c.decode(resp, &out)
}

// --- Deployments ---

type Deployment struct {
	ID       string `json:"id"`
	ServerID string `json:"server_id"`
	Status   string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func (c *Client) ListDeployments(workspaceID, serverID string) ([]Deployment, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/servers/%s/deployments", workspaceID, serverID), nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Deployments []Deployment `json:"deployments"`
	}
	return out.Deployments, c.decode(resp, &out)
}

type PublishResult struct {
	Deployment      Deployment       `json:"deployment"`
	MCPURL          string           `json:"mcp_url"`
	DeploymentToken string           `json:"deployment_token"`
	WorkspacePlan   string           `json:"workspace_plan"`
	Entitlements    PlanEntitlements `json:"entitlements"`
}

func (c *Client) Publish(workspaceID, serverID, slug, authConnectionID string) (*PublishResult, error) {
	body := map[string]string{"slug": slug}
	if authConnectionID != "" {
		body["auth_connection_id"] = authConnectionID
	}
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/servers/%s/deployments/publish", workspaceID, serverID), body)
	if err != nil {
		return nil, err
	}
	var out PublishResult
	return &out, c.decode(resp, &out)
}

// CheckSlugAvailability checks if a slug is available for a server.
func (c *Client) CheckSlugAvailability(workspaceID, serverID, slug string) (bool, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/servers/%s/slug-availability?slug=%s", workspaceID, serverID, url.QueryEscape(slug)), nil)
	if err != nil {
		return false, err
	}
	var out struct {
		Available bool `json:"available"`
	}
	return out.Available, c.decode(resp, &out)
}

func (c *Client) ActivateDeployment(workspaceID, serverID, deploymentID string) (*PublishResult, error) {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/servers/%s/deployments/%s/activate", workspaceID, serverID, deploymentID), nil)
	if err != nil {
		return nil, err
	}
	var out PublishResult
	return &out, c.decode(resp, &out)
}

// ReissueDeploymentToken revokes existing static deployment tokens and returns
// a freshly minted one. Used by `kraai servers reissue-token`.
func (c *Client) ReissueDeploymentToken(workspaceID, serverID, deploymentID string) (string, error) {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/servers/%s/deployments/%s/reissue-token", workspaceID, serverID, deploymentID), nil)
	if err != nil {
		return "", err
	}
	var out struct {
		DeploymentToken string `json:"deployment_token"`
	}
	if err := c.decode(resp, &out); err != nil {
		return "", err
	}
	return out.DeploymentToken, nil
}

func (c *Client) GetWorkspace(id string) (*Workspace, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s", id), nil)
	if err != nil {
		return nil, err
	}
	var out Workspace
	return &out, c.decode(resp, &out)
}

func (c *Client) ListPlans() ([]PlanEntitlements, error) {
	resp, err := c.do("GET", "/v1/plans", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Plans []PlanEntitlements `json:"plans"`
	}
	return out.Plans, c.decode(resp, &out)
}

// --- Usage ---

type WorkspaceUsage struct {
	WorkspaceID  string            `json:"workspace_id"`
	Plan         string            `json:"plan"`
	PlanLimit    int64             `json:"plan_limit"`
	TotalCount   int64             `json:"total_count"`
	PeriodStart  string            `json:"period_start"`
	PeriodEnd    string            `json:"period_end"`
	Entitlements PlanEntitlements  `json:"entitlements"`
	ByServer     []ServerUsageBrief `json:"by_server"`
}

type ServerUsageBrief struct {
	ServerID string `json:"server_id"`
	Count    int64  `json:"count"`
}

type ServerUsage struct {
	ServerID     string           `json:"server_id"`
	Plan         string           `json:"plan"`
	PlanLimit    int64            `json:"plan_limit"`
	Count        int64            `json:"count"`
	PeriodStart  string           `json:"period_start"`
	PeriodEnd    string           `json:"period_end"`
	Entitlements PlanEntitlements `json:"entitlements"`
}

func (c *Client) GetWorkspaceUsage(workspaceID string) (*WorkspaceUsage, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/usage", workspaceID), nil)
	if err != nil {
		return nil, err
	}
	var out WorkspaceUsage
	return &out, c.decode(resp, &out)
}

func (c *Client) GetServerUsage(workspaceID, serverID string) (*ServerUsage, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/servers/%s/usage", workspaceID, serverID), nil)
	if err != nil {
		return nil, err
	}
	var out ServerUsage
	return &out, c.decode(resp, &out)
}

// --- Logs ---

type RequestLog struct {
	ID           string  `json:"id"`
	WorkspaceID  string  `json:"workspace_id"`
	DeploymentID string  `json:"deployment_id"`
	ToolName     *string `json:"tool_name"`
	StatusCode   *int    `json:"status_code"`
	LatencyMs    *int    `json:"latency_ms"`
	CreatedAt    string  `json:"created_at"`
}

type ListLogsResult struct {
	Logs       []RequestLog `json:"logs"`
	NextCursor string       `json:"next_cursor"`
}

func (c *Client) ListLogs(workspaceID, serverID string, limit int, cursor string) (*ListLogsResult, error) {
	path := fmt.Sprintf("/v1/workspaces/%s/servers/%s/logs?limit=%d", workspaceID, serverID, limit)
	if cursor != "" {
		path += "&cursor=" + url.QueryEscape(cursor)
	}
	resp, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out ListLogsResult
	return &out, c.decode(resp, &out)
}

// --- Auth Connections ---

type AuthConnection struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	AuthKind  string          `json:"auth_kind"`
	Config    json.RawMessage `json:"config,omitempty"`
	CreatedAt string          `json:"created_at"`
}

type CreateAuthConnectionInput struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	InjectIn   string `json:"inject_in"`
	InjectName string `json:"inject_name"`
	Secret     string `json:"secret"`
}

func (c *Client) ListAuthConnections(workspaceID, serverID string) ([]AuthConnection, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/servers/%s/auth-connections", workspaceID, serverID), nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		AuthConnections []AuthConnection `json:"auth_connections"`
	}
	return out.AuthConnections, c.decode(resp, &out)
}

func (c *Client) CreateAuthConnection(workspaceID, serverID string, input CreateAuthConnectionInput) (*AuthConnection, error) {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/servers/%s/auth-connections", workspaceID, serverID), input)
	if err != nil {
		return nil, err
	}
	var out AuthConnection
	return &out, c.decode(resp, &out)
}

func (c *Client) DeleteAuthConnection(workspaceID, serverID, id string) error {
	resp, err := c.do("DELETE", fmt.Sprintf("/v1/workspaces/%s/servers/%s/auth-connections/%s", workspaceID, serverID, id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// --- Fetch Spec from URL ---

func (c *Client) FetchSpec(workspaceID, serverID, specURL string) (*APISource, error) {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/servers/%s/api-sources/fetch", workspaceID, serverID),
		map[string]string{"url": specURL})
	if err != nil {
		return nil, err
	}
	var out APISource
	return &out, c.decode(resp, &out)
}

// --- Workspace/Project Management ---

func (c *Client) RenameWorkspace(workspaceID, name string) error {
	resp, err := c.do("PATCH", fmt.Sprintf("/v1/workspaces/%s", workspaceID), map[string]string{"name": name})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

func (c *Client) RenameServer(workspaceID, serverID, name string) error {
	resp, err := c.do("PATCH", fmt.Sprintf("/v1/workspaces/%s/servers/%s", workspaceID, serverID), map[string]string{"name": name})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

func (c *Client) DeleteServer(workspaceID, serverID string) error {
	resp, err := c.do("DELETE", fmt.Sprintf("/v1/workspaces/%s/servers/%s", workspaceID, serverID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// --- Workflows ---

type WorkflowDefinition struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Definition  json.RawMessage `json:"definition"`
	CreatedAt   string          `json:"created_at"`
}

type WorkflowRun struct {
	ID            string  `json:"id"`
	DefinitionID  string  `json:"definition_id"`
	Status        string  `json:"status"`
	FailureReason *string `json:"failure_reason,omitempty"`
	StartedAt     *string `json:"started_at,omitempty"`
	CompletedAt   *string `json:"completed_at,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

type WorkflowStep struct {
	ID        string  `json:"id"`
	StepKey   string  `json:"step_key"`
	StepKind  string  `json:"step_kind"`
	State     string  `json:"state"`
	Attempt   int     `json:"attempt"`
	CreatedAt string  `json:"created_at"`
}

func (c *Client) ListWorkflowDefinitions(workspaceID, serverID string) ([]WorkflowDefinition, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/servers/%s/workflow-definitions", workspaceID, serverID), nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Definitions []WorkflowDefinition `json:"definitions"`
	}
	return out.Definitions, c.decode(resp, &out)
}

func (c *Client) CreateWorkflowDefinition(workspaceID, serverID, name, description string, definition json.RawMessage) (*WorkflowDefinition, error) {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/servers/%s/workflow-definitions", workspaceID, serverID), map[string]any{
		"name":        name,
		"description": description,
		"definition":  definition,
	})
	if err != nil {
		return nil, err
	}
	var out WorkflowDefinition
	return &out, c.decode(resp, &out)
}

func (c *Client) DeleteWorkflowDefinition(workspaceID, serverID, definitionID string) error {
	resp, err := c.do("DELETE", fmt.Sprintf("/v1/workspaces/%s/servers/%s/workflow-definitions/%s", workspaceID, serverID, definitionID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

func (c *Client) TriggerWorkflowRun(workspaceID, serverID, definitionID string) (*WorkflowRun, error) {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/servers/%s/workflow-definitions/%s/runs", workspaceID, serverID, definitionID), nil)
	if err != nil {
		return nil, err
	}
	var out WorkflowRun
	return &out, c.decode(resp, &out)
}

func (c *Client) ListWorkflowRuns(workspaceID, serverID, definitionID string) ([]WorkflowRun, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/servers/%s/workflow-definitions/%s/runs", workspaceID, serverID, definitionID), nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Runs []WorkflowRun `json:"runs"`
	}
	return out.Runs, c.decode(resp, &out)
}

func (c *Client) GetWorkflowRun(runID string) (*WorkflowRun, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workflow-runs/%s", runID), nil)
	if err != nil {
		return nil, err
	}
	var out WorkflowRun
	return &out, c.decode(resp, &out)
}

func (c *Client) GetWorkflowRunSteps(runID string) ([]WorkflowStep, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workflow-runs/%s/steps", runID), nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Steps []WorkflowStep `json:"steps"`
	}
	return out.Steps, c.decode(resp, &out)
}

func (c *Client) CancelWorkflowRun(runID string) error {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workflow-runs/%s/cancel", runID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// --- OPA Policies ---

type OPAPolicy struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

func (c *Client) ListPolicies(workspaceID string) ([]OPAPolicy, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/policies", workspaceID), nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Policies []OPAPolicy `json:"policies"`
	}
	return out.Policies, c.decode(resp, &out)
}

func (c *Client) CreatePolicy(workspaceID, name, regoSource string) (*OPAPolicy, error) {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/policies", workspaceID), map[string]string{
		"name":        name,
		"rego_source": regoSource,
	})
	if err != nil {
		return nil, err
	}
	var out OPAPolicy
	return &out, c.decode(resp, &out)
}

func (c *Client) DeletePolicy(workspaceID, policyID string) error {
	resp, err := c.do("DELETE", fmt.Sprintf("/v1/workspaces/%s/policies/%s", workspaceID, policyID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// --- Approvals ---

type ApprovalRequest struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	Action      string  `json:"action"`
	ResourceID  string  `json:"resource_id"`
	RequestedBy string  `json:"requested_by"`
	ReviewedBy  *string `json:"reviewed_by,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

func (c *Client) ListApprovals(workspaceID string) ([]ApprovalRequest, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/approvals", workspaceID), nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Approvals []ApprovalRequest `json:"approvals"`
	}
	return out.Approvals, c.decode(resp, &out)
}

func (c *Client) ListPendingApprovals(workspaceID string) ([]ApprovalRequest, error) {
	resp, err := c.do("GET", fmt.Sprintf("/v1/workspaces/%s/approvals/pending", workspaceID), nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Approvals []ApprovalRequest `json:"approvals"`
	}
	return out.Approvals, c.decode(resp, &out)
}

func (c *Client) ApproveRequest(workspaceID, approvalID string) error {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/approvals/%s/approve", workspaceID, approvalID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

func (c *Client) DenyRequest(workspaceID, approvalID string) error {
	resp, err := c.do("POST", fmt.Sprintf("/v1/workspaces/%s/approvals/%s/deny", workspaceID, approvalID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// --- MCP Client (direct calls to a live MCP endpoint) ---

// MCPClient calls a live MCP server endpoint using JSON-RPC.
type MCPClient struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

func NewMCPClient(endpoint, token string) *MCPClient {
	return &MCPClient{
		endpoint: endpoint,
		token:    token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				req.Header.Del("Authorization")
				if len(via) > 3 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

type mcpRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
}

type mcpRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type MCPTool struct {
	Name        string
	Description string
	Parameters  []MCPToolParam
}

type MCPToolParam struct {
	Name     string
	Type     string
	Required bool
}

type MCPServerInfo struct {
	Name            string
	Version         string
	ProtocolVersion string
}

func (mc *MCPClient) Initialize() (*MCPServerInfo, error) {
	body, err := json.Marshal(mcpRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	})
	if err != nil {
		return nil, fmt.Errorf("mcp: marshal: %w", err)
	}

	req, err := http.NewRequest("POST", mc.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mcp: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if mc.token != "" {
		req.Header.Set("Authorization", "Bearer "+mc.token)
	}

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mcp error %d: %s", resp.StatusCode, string(b))
	}

	var rpcResp mcpRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("mcp: decode: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcp: parse init: %w", err)
	}

	return &MCPServerInfo{
		Name:            result.ServerInfo.Name,
		Version:         result.ServerInfo.Version,
		ProtocolVersion: result.ProtocolVersion,
	}, nil
}

func (mc *MCPClient) ListTools() ([]MCPTool, error) {
	body, err := json.Marshal(mcpRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	})
	if err != nil {
		return nil, fmt.Errorf("mcp: marshal: %w", err)
	}

	req, err := http.NewRequest("POST", mc.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mcp: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if mc.token != "" {
		req.Header.Set("Authorization", "Bearer "+mc.token)
	}

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized — provide a valid --token")
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mcp error %d: %s", resp.StatusCode, string(b))
	}

	var rpcResp mcpRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("mcp: decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Parse the tools/list result.
	var result struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			InputSchema struct {
				Properties map[string]struct {
					Type string `json:"type"`
				} `json:"properties"`
				Required []string `json:"required"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcp: parse tools: %w", err)
	}

	tools := make([]MCPTool, 0, len(result.Tools))
	for _, t := range result.Tools {
		requiredSet := make(map[string]bool, len(t.InputSchema.Required))
		for _, r := range t.InputSchema.Required {
			requiredSet[r] = true
		}
		params := make([]MCPToolParam, 0, len(t.InputSchema.Properties))
		for name, prop := range t.InputSchema.Properties {
			params = append(params, MCPToolParam{
				Name:     name,
				Type:     prop.Type,
				Required: requiredSet[name],
			})
		}
		tools = append(tools, MCPTool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return tools, nil
}
