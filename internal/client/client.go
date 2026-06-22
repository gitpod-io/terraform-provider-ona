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
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultHost = "https://app.gitpod.io"
	UserAgent   = "terraform-provider-ona/dev"
)

var ErrMissingToken = errors.New("missing Ona token: set provider token or ONA_TOKEN")

type Config struct {
	Host      string
	Token     string
	UserAgent string
}

type Client struct {
	APIBaseURL string

	httpClient *http.Client
	token      string
	userAgent  string

	mu             sync.Mutex
	organizationID string
}

func New(cfg Config) (*Client, error) {
	host := resolveHost(cfg.Host)
	token := resolveToken(cfg.Token)
	if token == "" {
		return nil, ErrMissingToken
	}

	apiBaseURL, err := APIBaseURL(host)
	if err != nil {
		return nil, err
	}

	userAgent := strings.TrimSpace(cfg.UserAgent)
	if userAgent == "" {
		userAgent = UserAgent
	}
	return &Client{
		APIBaseURL: apiBaseURL,
		httpClient: http.DefaultClient,
		token:      token,
		userAgent:  userAgent,
	}, nil
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
	data, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= 20; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.APIBaseURL, "/")+path, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.userAgent)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if len(bytes.TrimSpace(body)) == 0 {
				return nil
			}
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("decode response from %s: %w", path, err)
			}
			return nil
		}

		apiErr := apiErrorFromResponse(resp.StatusCode, body)
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 20 {
			lastErr = apiErr
			time.Sleep(retryDelay(resp.Header.Get("Retry-After"), attempt))
			continue
		}
		return apiErr
	}
	return lastErr
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds > 0 {
		delay := time.Duration(seconds) * time.Second
		if delay > 30*time.Second {
			return 30 * time.Second
		}
		return delay
	}
	delay := time.Duration(1<<minInt(attempt, 5)) * time.Second
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
