package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

type APIError struct {
	StatusCode int
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	if e.Code != "" || e.Message != "" {
		return fmt.Sprintf("ona api error %d %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("ona api error %d", e.StatusCode)
}

func IsNotFound(err error) bool {
	apiErr, ok := err.(*APIError)
	return ok && (apiErr.StatusCode == 404 || apiErr.Code == "not_found")
}

func apiErrorFromResponse(statusCode int, body []byte) error {
	var apiErr APIError
	_ = json.Unmarshal(body, &apiErr)
	apiErr.StatusCode = statusCode
	if apiErr.Message == "" {
		apiErr.Message = strings.TrimSpace(string(body))
	}
	return &apiErr
}

type PaginationRequest struct {
	PageSize int    `json:"pageSize,omitempty"`
	Token    string `json:"token,omitempty"`
}

type PaginationResponse struct {
	NextToken string `json:"nextToken,omitempty"`
}

func (p *PaginationResponse) GetNextToken() string {
	if p == nil {
		return ""
	}
	return p.NextToken
}

type GetAuthenticatedIdentityResponse struct {
	OrganizationID string `json:"organizationId,omitempty"`
}

type ListGroupsRequest struct {
	Pagination *PaginationRequest `json:"pagination,omitempty"`
}

type ListGroupsResponse struct {
	Groups     []*Group            `json:"groups,omitempty"`
	Pagination *PaginationResponse `json:"pagination,omitempty"`
}

type Group struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	OrganizationID string `json:"organizationId,omitempty"`
	SystemManaged  bool   `json:"systemManaged,omitempty"`
	DirectShare    bool   `json:"directShare,omitempty"`
}

func (g *Group) GetId() string {
	if g == nil {
		return ""
	}
	return g.ID
}

func (g *Group) GetName() string {
	if g == nil {
		return ""
	}
	return g.Name
}

func (g *Group) GetOrganizationId() string {
	if g == nil {
		return ""
	}
	return g.OrganizationID
}

func (g *Group) GetSystemManaged() bool {
	return g != nil && g.SystemManaged
}

func (g *Group) GetDirectShare() bool {
	return g != nil && g.DirectShare
}

type ListMembershipsRequest struct {
	GroupID    string             `json:"groupId,omitempty"`
	Pagination *PaginationRequest `json:"pagination,omitempty"`
}

type ListMembershipsResponse struct {
	Members    []*GroupMembership  `json:"members,omitempty"`
	Pagination *PaginationResponse `json:"pagination,omitempty"`
}

type GroupMembership struct {
	Subject *Subject `json:"subject,omitempty"`
}

func (m *GroupMembership) GetSubject() *Subject {
	if m == nil {
		return nil
	}
	return m.Subject
}

type Principal string

const (
	PrincipalUnspecified    Principal = "PRINCIPAL_UNSPECIFIED"
	PrincipalUser           Principal = "PRINCIPAL_USER"
	PrincipalServiceAccount Principal = "PRINCIPAL_SERVICE_ACCOUNT"
)

type Subject struct {
	ID        string    `json:"id,omitempty"`
	Principal Principal `json:"principal,omitempty"`
}

func (s *Subject) GetId() string {
	if s == nil {
		return ""
	}
	return s.ID
}

func (s *Subject) GetPrincipal() Principal {
	if s == nil {
		return PrincipalUnspecified
	}
	return s.Principal
}

type ListTeamsRequest struct {
	Pagination *PaginationRequest `json:"pagination,omitempty"`
}

type ListTeamsResponse struct {
	Teams      []*Team             `json:"teams,omitempty"`
	Pagination *PaginationResponse `json:"pagination,omitempty"`
}

type Team struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	OrganizationID string `json:"organizationId,omitempty"`
}

func (t *Team) GetId() string {
	if t == nil {
		return ""
	}
	return t.ID
}

func (t *Team) GetName() string {
	if t == nil {
		return ""
	}
	return t.Name
}

func (t *Team) GetOrganizationId() string {
	if t == nil {
		return ""
	}
	return t.OrganizationID
}

type ListSecurityPoliciesRequest struct {
	Pagination *PaginationRequest          `json:"pagination,omitempty"`
	Filter     *ListSecurityPoliciesFilter `json:"filter,omitempty"`
}

type ListSecurityPoliciesFilter struct {
	OrganizationID string `json:"organizationId,omitempty"`
}

type ListSecurityPoliciesResponse struct {
	SecurityPolicies []*SecurityPolicy   `json:"securityPolicies,omitempty"`
	Pagination       *PaginationResponse `json:"pagination,omitempty"`
}

type SecurityPolicy struct {
	ID       string                  `json:"id,omitempty"`
	Metadata *SecurityPolicyMetadata `json:"metadata,omitempty"`
}

func (p *SecurityPolicy) GetId() string {
	if p == nil {
		return ""
	}
	return p.ID
}

func (p *SecurityPolicy) GetMetadata() *SecurityPolicyMetadata {
	if p == nil {
		return nil
	}
	return p.Metadata
}

type SecurityPolicyMetadata struct {
	Name string `json:"name,omitempty"`
}

func (m *SecurityPolicyMetadata) GetName() string {
	if m == nil {
		return ""
	}
	return m.Name
}

type ListRunnersRequest struct {
	Pagination *PaginationRequest `json:"pagination,omitempty"`
}

type ListRunnersResponse struct {
	Runners    []*Runner           `json:"runners,omitempty"`
	Pagination *PaginationResponse `json:"pagination,omitempty"`
}

type Runner struct {
	RunnerID string `json:"runnerId,omitempty"`
	Name     string `json:"name,omitempty"`
}

func (r *Runner) GetRunnerId() string {
	if r == nil {
		return ""
	}
	return r.RunnerID
}

func (r *Runner) GetName() string {
	if r == nil {
		return ""
	}
	return r.Name
}

type ListEnvironmentClassesRequest struct {
	Pagination *PaginationRequest            `json:"pagination,omitempty"`
	Filter     *ListEnvironmentClassesFilter `json:"filter,omitempty"`
}

type ListEnvironmentClassesFilter struct {
	RunnerIDs []string `json:"runnerIds,omitempty"`
}

type ListEnvironmentClassesResponse struct {
	EnvironmentClasses []*EnvironmentClass `json:"environmentClasses,omitempty"`
	Pagination         *PaginationResponse `json:"pagination,omitempty"`
}

type GetEnvironmentClassRequest struct {
	EnvironmentClassID string `json:"environmentClassId,omitempty"`
}

type GetEnvironmentClassResponse struct {
	EnvironmentClass *EnvironmentClass `json:"environmentClass,omitempty"`
}

type EnvironmentClass struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	RunnerID    string `json:"runnerId,omitempty"`
}

func (c *EnvironmentClass) GetId() string {
	if c == nil {
		return ""
	}
	return c.ID
}

func (c *EnvironmentClass) GetDisplayName() string {
	if c == nil {
		return ""
	}
	return c.DisplayName
}

func (c *EnvironmentClass) GetRunnerId() string {
	if c == nil {
		return ""
	}
	return c.RunnerID
}

type ListProjectsRequest struct {
	Pagination *PaginationRequest `json:"pagination,omitempty"`
}

type ListProjectsResponse struct {
	Projects   []*Project          `json:"projects,omitempty"`
	Pagination *PaginationResponse `json:"pagination,omitempty"`
}

type Project struct {
	ID                    string                        `json:"id,omitempty"`
	Metadata              *ProjectMetadata              `json:"metadata,omitempty"`
	EnvironmentClasses    []*ProjectEnvironmentClass    `json:"environmentClasses,omitempty"`
	PrebuildConfiguration *ProjectPrebuildConfiguration `json:"prebuildConfiguration,omitempty"`
}

func (p *Project) GetId() string {
	if p == nil {
		return ""
	}
	return p.ID
}

func (p *Project) GetMetadata() *ProjectMetadata {
	if p == nil {
		return nil
	}
	return p.Metadata
}

func (p *Project) GetEnvironmentClasses() []*ProjectEnvironmentClass {
	if p == nil {
		return nil
	}
	return p.EnvironmentClasses
}

func (p *Project) GetPrebuildConfiguration() *ProjectPrebuildConfiguration {
	if p == nil {
		return nil
	}
	return p.PrebuildConfiguration
}

type ProjectMetadata struct {
	Name           string `json:"name,omitempty"`
	OrganizationID string `json:"organizationId,omitempty"`
}

func (m *ProjectMetadata) GetName() string {
	if m == nil {
		return ""
	}
	return m.Name
}

func (m *ProjectMetadata) GetOrganizationId() string {
	if m == nil {
		return ""
	}
	return m.OrganizationID
}

type ListProjectEnvironmentClassesRequest struct {
	ProjectID  string             `json:"projectId,omitempty"`
	Pagination *PaginationRequest `json:"pagination,omitempty"`
}

type ListProjectEnvironmentClassesResponse struct {
	ProjectEnvironmentClasses []*ProjectEnvironmentClass `json:"projectEnvironmentClasses,omitempty"`
	Pagination                *PaginationResponse        `json:"pagination,omitempty"`
}

type ProjectEnvironmentClass struct {
	EnvironmentClassID string `json:"environmentClassId,omitempty"`
	LocalRunner        bool   `json:"localRunner,omitempty"`
}

func (c *ProjectEnvironmentClass) GetEnvironmentClassId() string {
	if c == nil {
		return ""
	}
	return c.EnvironmentClassID
}

type ProjectPrebuildConfiguration struct {
	EnvironmentClassIDs []string `json:"environmentClassIds,omitempty"`
	Executor            *Subject `json:"executor,omitempty"`
}

func (c *ProjectPrebuildConfiguration) GetEnvironmentClassIds() []string {
	if c == nil {
		return nil
	}
	return c.EnvironmentClassIDs
}

func (c *ProjectPrebuildConfiguration) GetExecutor() *Subject {
	if c == nil {
		return nil
	}
	return c.Executor
}

type ListServiceAccountsRequest struct {
	Pagination *PaginationRequest         `json:"pagination,omitempty"`
	Filter     *ListServiceAccountsFilter `json:"filter,omitempty"`
}

type ListServiceAccountsFilter struct {
	IncludeSuspended bool `json:"includeSuspended,omitempty"`
}

type ListServiceAccountsResponse struct {
	ServiceAccounts []*ServiceAccount   `json:"serviceAccounts,omitempty"`
	Pagination      *PaginationResponse `json:"pagination,omitempty"`
}

type ServiceAccount struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	OrganizationID string `json:"organizationId,omitempty"`
}

func (a *ServiceAccount) GetId() string {
	if a == nil {
		return ""
	}
	return a.ID
}

func (a *ServiceAccount) GetName() string {
	if a == nil {
		return ""
	}
	return a.Name
}

func (a *ServiceAccount) GetOrganizationId() string {
	if a == nil {
		return ""
	}
	return a.OrganizationID
}

type ListWorkflowsRequest struct {
	Pagination *PaginationRequest `json:"pagination,omitempty"`
}

type ListWorkflowsResponse struct {
	Workflows  []*Workflow         `json:"workflows,omitempty"`
	Pagination *PaginationResponse `json:"pagination,omitempty"`
}

type Workflow struct {
	ID       string            `json:"id,omitempty"`
	Metadata *WorkflowMetadata `json:"metadata,omitempty"`
	Spec     *WorkflowSpec     `json:"spec,omitempty"`
}

func (w *Workflow) GetId() string {
	if w == nil {
		return ""
	}
	return w.ID
}

func (w *Workflow) GetMetadata() *WorkflowMetadata {
	if w == nil {
		return nil
	}
	return w.Metadata
}

func (w *Workflow) GetSpec() *WorkflowSpec {
	if w == nil {
		return nil
	}
	return w.Spec
}

type WorkflowMetadata struct {
	Name string `json:"name,omitempty"`
}

func (m *WorkflowMetadata) GetName() string {
	if m == nil {
		return ""
	}
	return m.Name
}

type WorkflowSpec struct {
	Report   any                `json:"report,omitempty"`
	Triggers []*WorkflowTrigger `json:"triggers,omitempty"`
}

func (s *WorkflowSpec) GetReport() any {
	if s == nil {
		return nil
	}
	return s.Report
}

func (s *WorkflowSpec) GetTriggers() []*WorkflowTrigger {
	if s == nil {
		return nil
	}
	return s.Triggers
}

type WorkflowTrigger struct {
	Context  *WorkflowTriggerContext `json:"context,omitempty"`
	Incident any                     `json:"incident,omitempty"`
}

func (t *WorkflowTrigger) GetContext() *WorkflowTriggerContext {
	if t == nil {
		return nil
	}
	return t.Context
}

func (t *WorkflowTrigger) GetIncident() any {
	if t == nil {
		return nil
	}
	return t.Incident
}

type WorkflowTriggerContext struct {
	Repositories any `json:"repositories,omitempty"`
}

func (c *WorkflowTriggerContext) GetRepositories() any {
	if c == nil {
		return nil
	}
	return c.Repositories
}

type GetOrganizationPoliciesRequest struct {
	OrganizationID string `json:"organizationId,omitempty"`
}

type GetOrganizationPoliciesResponse struct {
	Policies *OrganizationPolicies `json:"policies,omitempty"`
}

type OrganizationPolicies struct{}
