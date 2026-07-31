package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"connectrpc.com/connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	providerversion "github.com/gitpod-io/terraform-provider-ona/version"
)

const (
	DefaultHost = "https://app.gitpod.io"
)

var ErrMissingToken = errors.New("missing Ona token: set provider token or ONA_TOKEN")

type Config struct {
	Host      string
	Token     string
	UserAgent string
}

type Client struct {
	APIBaseURL string

	raw *rawClient

	mu             sync.Mutex
	organizationID string
}

func DefaultUserAgent() string {
	return providerversion.UserAgent()
}

func New(cfg Config) (*Client, error) {
	raw, apiBaseURL, err := newRawClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{
		APIBaseURL: apiBaseURL,
		raw:        raw,
	}, nil
}

func newRawClient(cfg Config) (*rawClient, string, error) {
	host := resolveHost(cfg.Host)
	token := resolveToken(cfg.Token)
	if token == "" {
		return nil, "", ErrMissingToken
	}

	apiBaseURL, err := APIBaseURL(host)
	if err != nil {
		return nil, "", err
	}

	userAgent := strings.TrimSpace(cfg.UserAgent)
	if userAgent == "" {
		userAgent = DefaultUserAgent()
	}
	return &rawClient{
		baseURL:    apiBaseURL,
		token:      token,
		userAgent:  userAgent,
		httpClient: http.DefaultClient,
	}, apiBaseURL, nil
}

func NewManagementPlane(cfg Config) (*managementclient.ManagementPlane, string, error) {
	host := resolveHost(cfg.Host)
	token := resolveToken(cfg.Token)
	if token == "" {
		return nil, "", ErrMissingToken
	}

	apiBaseURL, err := APIBaseURL(host)
	if err != nil {
		return nil, "", err
	}

	userAgent := strings.TrimSpace(cfg.UserAgent)
	if userAgent == "" {
		userAgent = DefaultUserAgent()
	}

	api, err := managementclient.New(apiBaseURL,
		managementclient.WithAccessToken(token),
		managementclient.WithUserAgent(userAgent),
	)
	if err != nil {
		return nil, "", fmt.Errorf("create Ona management client: %w", err)
	}
	return api, apiBaseURL, nil
}

func (c *Client) AuthenticatedOrganizationID(ctx context.Context) (string, error) {
	c.mu.Lock()
	cached := c.organizationID
	c.mu.Unlock()
	if cached != "" {
		return cached, nil
	}

	resp, err := c.GetAuthenticatedIdentity(ctx)
	if err != nil {
		return "", err
	}
	organizationID := resp.OrganizationID
	if organizationID == "" {
		return "", fmt.Errorf("authenticated identity did not include an organizationId")
	}

	c.mu.Lock()
	c.organizationID = organizationID
	c.mu.Unlock()
	return organizationID, nil
}

func (c *Client) GetAuthenticatedIdentity(ctx context.Context) (*GetAuthenticatedIdentityResponse, error) {
	var resp GetAuthenticatedIdentityResponse
	if err := c.post(ctx, "/gitpod.v1.IdentityService/GetAuthenticatedIdentity", struct{}{}, &resp); err != nil {
		return nil, fmt.Errorf("get authenticated identity: %w", err)
	}
	return &resp, nil
}

func (c *Client) ListGroups(ctx context.Context, req ListGroupsRequest) (*ListGroupsResponse, error) {
	var resp ListGroupsResponse
	if err := c.post(ctx, "/gitpod.v1.GroupService/ListGroups", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListMemberships(ctx context.Context, req ListMembershipsRequest) (*ListMembershipsResponse, error) {
	var resp ListMembershipsResponse
	if err := c.post(ctx, "/gitpod.v1.GroupService/ListMemberships", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListTeams(ctx context.Context, req ListTeamsRequest) (*ListTeamsResponse, error) {
	var resp ListTeamsResponse
	if err := c.post(ctx, "/gitpod.v1.TeamService/ListTeams", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListSecurityPolicies(ctx context.Context, req ListSecurityPoliciesRequest) (*ListSecurityPoliciesResponse, error) {
	var resp ListSecurityPoliciesResponse
	if err := c.post(ctx, "/gitpod.v1.SecurityService/ListSecurityPolicies", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListRunners(ctx context.Context, req ListRunnersRequest) (*ListRunnersResponse, error) {
	var resp ListRunnersResponse
	if err := c.post(ctx, "/gitpod.v1.RunnerService/ListRunners", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListEnvironmentClasses(ctx context.Context, req ListEnvironmentClassesRequest) (*ListEnvironmentClassesResponse, error) {
	var resp ListEnvironmentClassesResponse
	if err := c.post(ctx, "/gitpod.v1.RunnerConfigurationService/ListEnvironmentClasses", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetEnvironmentClass(ctx context.Context, req GetEnvironmentClassRequest) (*GetEnvironmentClassResponse, error) {
	var resp GetEnvironmentClassResponse
	if err := c.post(ctx, "/gitpod.v1.RunnerConfigurationService/GetEnvironmentClass", req, &resp); err != nil {
		if IsNotFound(err) {
			return nil, err
		}
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListProjects(ctx context.Context, req ListProjectsRequest) (*ListProjectsResponse, error) {
	var resp ListProjectsResponse
	if err := c.post(ctx, "/gitpod.v1.ProjectService/ListProjects", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListProjectEnvironmentClasses(ctx context.Context, req ListProjectEnvironmentClassesRequest) (*ListProjectEnvironmentClassesResponse, error) {
	var resp ListProjectEnvironmentClassesResponse
	if err := c.post(ctx, "/gitpod.v1.ProjectService/ListProjectEnvironmentClasses", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListServiceAccounts(ctx context.Context, req ListServiceAccountsRequest) (*ListServiceAccountsResponse, error) {
	var resp ListServiceAccountsResponse
	if err := c.post(ctx, "/gitpod.v1.ServiceAccountService/ListServiceAccounts", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListWorkflows(ctx context.Context, req ListWorkflowsRequest) (*ListWorkflowsResponse, error) {
	var resp ListWorkflowsResponse
	if err := c.post(ctx, "/gitpod.v1.WorkflowService/ListWorkflows", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetOrganizationPolicies(ctx context.Context, req GetOrganizationPoliciesRequest) (*GetOrganizationPoliciesResponse, error) {
	var resp GetOrganizationPoliciesResponse
	if err := c.post(ctx, "/gitpod.v1.OrganizationService/GetOrganizationPolicies", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) post(ctx context.Context, path string, in any, out any) error {
	return c.raw.Post(ctx, strings.TrimLeft(path, "/"), in, out)
}

func (c *Client) Post(ctx context.Context, path string, in any, out any) error {
	return c.post(ctx, path, in, out)
}

type rawClient struct {
	baseURL    string
	token      string
	userAgent  string
	httpClient *http.Client
}

func (c *rawClient) Post(ctx context.Context, path string, in any, out any) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(in); err != nil {
		return fmt.Errorf("encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+"/"+strings.TrimLeft(path, "/"), &body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return decodeAPIError(resp)
	}
	if out == nil {
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			return fmt.Errorf("drain response body: %w", err)
		}
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	return nil
}

func decodeAPIError(resp *http.Response) error {
	apiErr := &APIError{StatusCode: resp.StatusCode}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		apiErr.Message = err.Error()
		return apiErr
	}
	if len(body) == 0 {
		return apiErr
	}
	if err := json.Unmarshal(body, apiErr); err != nil {
		apiErr.Message = strings.TrimSpace(string(body))
		return apiErr
	}
	switch apiErr.Code {
	case "already_exists":
		return connect.NewError(connect.CodeAlreadyExists, apiErr)
	case "not_found":
		return connect.NewError(connect.CodeNotFound, apiErr)
	default:
		return apiErr
	}
}

func APIBaseURL(host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		host = DefaultHost
	}
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}

	parsed, err := url.Parse(host)
	if err != nil {
		return "", fmt.Errorf("parse Ona host: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported Ona host scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("ona host must include a hostname")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func resolveHost(configured string) string {
	if strings.TrimSpace(configured) != "" {
		return configured
	}
	if v := os.Getenv("ONA_HOST"); strings.TrimSpace(v) != "" {
		return v
	}
	return DefaultHost
}

func resolveToken(configured string) string {
	if strings.TrimSpace(configured) != "" {
		return configured
	}
	if v := os.Getenv("ONA_TOKEN"); strings.TrimSpace(v) != "" {
		return v
	}
	return ""
}
