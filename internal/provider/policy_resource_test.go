// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAccPolicyResourcesLifecycle(t *testing.T) {
	t.Parallel()

	server := newPolicyAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.security.policyDeleted("policy-1") {
				return errors.New("policy-1 was not deleted")
			}
			if diff := server.organization.defaultsDiff(); diff != "" {
				return fmt.Errorf("organization policies were not restored to their server-defined defaults: %s", diff)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccPolicyConfig(server.URL, "baseline", "24h"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_security_policy.baseline", "id", "policy-1"),
					resource.TestCheckResourceAttr("ona_security_policy.baseline", "organization_id", "org-1"),
					resource.TestCheckResourceAttr("ona_security_policy.baseline", "name", "baseline"),
					resource.TestCheckResourceAttr("ona_security_policy.baseline", "spec.ports.default_effect", "allow"),
					resource.TestCheckResourceAttr("ona_security_policy.baseline", "spec.ports.rule.0.range_from", "22"),
					resource.TestCheckResourceAttr("ona_security_policy.baseline", "spec.files.default_actions.#", "2"),
					resource.TestCheckResourceAttr("data.ona_security_policies.all", "policies.#", "1"),
					resource.TestCheckResourceAttr("data.ona_security_policies.all", "policies.0.id", "policy-1"),
					resource.TestCheckResourceAttr("ona_organization_policies.test", "id", "org-1"),
					resource.TestCheckResourceAttr("ona_organization_policies.test", "security_policy_id", "policy-1"),
					resource.TestCheckResourceAttr("ona_organization_policies.test", "members_require_projects", "true"),
					resource.TestCheckResourceAttr("ona_organization_policies.test", "members_create_projects", "false"),
					resource.TestCheckResourceAttr("ona_organization_policies.test", "agent_policy.mcp_disabled", "true"),
					resource.TestCheckResourceAttr("ona_organization_policies.test", "agent_policy.allowed_agent_ids.#", "1"),
				),
			},
			{
				Config: testAccPolicyConfig(server.URL, "baseline", "24h"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_security_policy.baseline",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"spec.block_devices.%",
					"spec.block_devices.default_effect",
					"spec.data.%",
					"spec.data.default_effect",
					"spec.data.rule.#",
					"spec.data.rule.0.%",
					"spec.data.rule.0.destination.%",
					"spec.data.rule.0.destination.host",
					"spec.data.rule.0.effect",
					"spec.data.rule.0.source.%",
					"spec.data.rule.0.source.file",
					"spec.data.rule.0.source.selector",
					"spec.files.%",
					"spec.files.default_actions.#",
					"spec.files.default_actions.0",
					"spec.files.default_actions.1",
					"spec.files.default_effect",
					"spec.files.rule.#",
					"spec.files.rule.0.%",
					"spec.files.rule.0.actions.#",
					"spec.files.rule.0.actions.0",
					"spec.files.rule.0.effect",
					"spec.files.rule.0.path",
					"spec.ports.%",
					"spec.ports.default_effect",
					"spec.ports.rule.#",
					"spec.ports.rule.0.%",
					"spec.ports.rule.0.effect",
					"spec.ports.rule.0.range_from",
					"spec.ports.rule.0.range_to",
				},
			},
			{
				ResourceName:      "ona_organization_policies.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"archive_environments_after",
					"delete_archived_environments_after",
					"maximum_environment_lifetime",
					"maximum_environment_timeout",
				},
			},
			{
				Config: testAccPolicyConfig(server.URL, "baseline-updated", "48h"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_security_policy.baseline", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("ona_organization_policies.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_security_policy.baseline", "name", "baseline-updated"),
					resource.TestCheckResourceAttr("ona_organization_policies.test", "archive_environments_after", "48h"),
				),
			},
		},
	})
}

func TestAccPolicyResourcesRejectInvalidAgentPolicyAtPlan(t *testing.T) {
	t.Parallel()

	server := newPolicyAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccInvalidAgentPolicyConfig(server.URL, `conversation_sharing_policy = "workspace"`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Conversation Sharing Policy`),
			},
			{
				Config:      testAccInvalidAgentPolicyConfig(server.URL, `max_subagents_per_environment = 11`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Max Subagents`),
			},
		},
	})
}

func TestAccPolicyResourcesOmittedSettingsRemainUnmanaged(t *testing.T) {
	t.Parallel()

	server := newPolicyAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMinimalPolicyConfig(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_organization_policies.test", "id", "org-1"),
					resource.TestCheckNoResourceAttr("ona_organization_policies.test", "allowed_editor_ids"),
					resource.TestCheckNoResourceAttr("ona_organization_policies.test", "editor_version_restriction"),
					resource.TestCheckNoResourceAttr("ona_organization_policies.test", "agent_policy"),
				),
			},
			{
				Config: testAccMinimalPolicyConfig(server.URL),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccPolicyConfig(host string, policyName string, archiveAfter string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_security_policy" "baseline" {
  organization_id = "org-1"
  name            = %[2]q

  spec {
    ports {
      default_effect = "allow"

      rule {
        range_from = 22
        range_to   = 22
        effect     = "block"
      }
    }

    executables {
      default_effect = "allow"

      rule {
        path   = "/usr/bin/nc"
        effect = "audit"
      }
    }

    files {
      default_effect  = "allow"
      default_actions = ["read", "write"]

      rule {
        path    = "/etc/shadow"
        actions = ["read"]
        effect  = "block"
      }
    }

    block_devices {
      default_effect = "block"
    }

    data {
      default_effect = "allow"

      rule {
        source {
          file     = "/workspace/secrets.env"
          selector = "10:20"
        }
        destination {
          host = "example.com"
        }
        effect = "block"
      }
    }
  }
}

data "ona_security_policies" "all" {
  organization_id = "org-1"

  depends_on = [ona_security_policy.baseline]
}

resource "ona_organization_policies" "test" {
  members_require_projects                = true
  members_create_projects                 = false
  allowed_editor_ids                      = ["editor-a"]
  default_editor_id                       = "editor-a"
  allow_local_runners                     = false
  maximum_running_environments_per_user   = 2
  maximum_environments_per_user           = 5
  default_environment_image               = "ubuntu:24.04"
  port_sharing_disabled                   = true
  delete_archived_environments_after      = "24h"
  maximum_environment_timeout             = "30m"
  maximum_environment_lifetime            = "720h"
  require_custom_domain_access            = false
  restrict_account_creation_to_scim       = true
  web_browser_disabled                    = true
  disable_from_scratch                    = true
  security_policy_id                      = ona_security_policy.baseline.id
  archive_environments_after              = %[3]q

  editor_version_restriction = [{
    editor_id        = "editor-a"
    allowed_versions = ["2026.1"]
  }]

  agent_policy = {
    mcp_disabled                  = true
    command_deny_list             = ["rm -rf /"]
    scm_tools_disabled            = true
    scm_tools_allowed_group_id    = "group-1"
    conversation_sharing_policy   = "organization"
    max_subagents_per_environment = 3
    allowed_agent_ids             = ["ona"]
  }
}
`, host, policyName, archiveAfter)
}

func testAccMinimalPolicyConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_organization_policies" "test" {
}
`, host)
}

func testAccInvalidAgentPolicyConfig(host string, agentPolicyBody string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_organization_policies" "test" {
  agent_policy = {
    %[2]s
  }
}
`, host, agentPolicyBody)
}

type policyAPIServer struct {
	*httptest.Server
	security     *fakeSecurityService
	organization *fakeOrganizationService
}

func newPolicyAPIServer(t *testing.T) *policyAPIServer {
	t.Helper()

	securityService := &fakeSecurityService{
		policies: map[string]*v1.SecurityPolicy{},
		now:      time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
	}
	defaults := newTestOrganizationPolicies("org-1")
	organizationService := &fakeOrganizationService{
		policies: cloneOrganizationPolicies(defaults),
		defaults: cloneOrganizationPolicies(defaults),
	}

	securityPath, securityHandler := v1connect.NewSecurityServiceHandler(securityService)
	organizationPath, organizationHandler := v1connect.NewOrganizationServiceHandler(organizationService)
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(organizationService)
	mux := http.NewServeMux()
	mux.Handle(securityPath, securityHandler)
	mux.Handle(organizationPath, organizationHandler)
	mux.Handle(identityPath, identityHandler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))

	return &policyAPIServer{
		Server:       server,
		security:     securityService,
		organization: organizationService,
	}
}

type fakeSecurityService struct {
	v1connect.UnimplementedSecurityServiceHandler

	mu       sync.Mutex
	policies map[string]*v1.SecurityPolicy
	deleted  []string
	now      time.Time
}

func (s *fakeSecurityService) CreateSecurityPolicy(ctx context.Context, req *connect.Request[v1.CreateSecurityPolicyRequest]) (*connect.Response[v1.CreateSecurityPolicyResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("policy-%d", len(s.policies)+1)
	policy := &v1.SecurityPolicy{
		Id:             id,
		OrganizationId: req.Msg.GetOrganizationId(),
		Metadata:       cloneSecurityPolicyMetadata(req.Msg.GetMetadata()),
		Spec:           cloneSecurityPolicySpec(req.Msg.GetSpec()),
		CreatedAt:      timestamppb.New(s.now),
		UpdatedAt:      timestamppb.New(s.now),
	}
	s.policies[id] = policy
	return connect.NewResponse(&v1.CreateSecurityPolicyResponse{SecurityPolicy: cloneSecurityPolicy(policy)}), nil
}

func (s *fakeSecurityService) GetSecurityPolicy(ctx context.Context, req *connect.Request[v1.GetSecurityPolicyRequest]) (*connect.Response[v1.GetSecurityPolicyResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	policy := s.policies[req.Msg.GetSecurityPolicyId()]
	if policy == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("security policy not found"))
	}
	return connect.NewResponse(&v1.GetSecurityPolicyResponse{SecurityPolicy: cloneSecurityPolicy(policy)}), nil
}

func (s *fakeSecurityService) ListSecurityPolicies(ctx context.Context, req *connect.Request[v1.ListSecurityPoliciesRequest]) (*connect.Response[v1.ListSecurityPoliciesResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filter := req.Msg.GetFilter()
	idFilter := map[string]struct{}{}
	for _, id := range filter.GetSecurityPolicyIds() {
		idFilter[id] = struct{}{}
	}

	var policies []*v1.SecurityPolicy
	for _, policy := range s.policies {
		if filter.GetOrganizationId() != "" && policy.GetOrganizationId() != filter.GetOrganizationId() {
			continue
		}
		if len(idFilter) > 0 {
			if _, ok := idFilter[policy.GetId()]; !ok {
				continue
			}
		}
		if filter.GetSearch() != "" && !strings.Contains(policy.GetMetadata().GetName(), filter.GetSearch()) {
			continue
		}
		policies = append(policies, cloneSecurityPolicy(policy))
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].GetId() < policies[j].GetId()
	})
	return connect.NewResponse(&v1.ListSecurityPoliciesResponse{SecurityPolicies: policies}), nil
}

func (s *fakeSecurityService) UpdateSecurityPolicy(ctx context.Context, req *connect.Request[v1.UpdateSecurityPolicyRequest]) (*connect.Response[v1.UpdateSecurityPolicyResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	policy := s.policies[req.Msg.GetSecurityPolicyId()]
	if policy == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("security policy not found"))
	}
	if req.Msg.GetMetadata() != nil {
		policy.Metadata = cloneSecurityPolicyMetadata(req.Msg.GetMetadata())
	}
	if req.Msg.GetSpec() != nil {
		policy.Spec = cloneSecurityPolicySpec(req.Msg.GetSpec())
	}
	policy.UpdatedAt = timestamppb.New(s.now.Add(time.Hour))
	return connect.NewResponse(&v1.UpdateSecurityPolicyResponse{SecurityPolicy: cloneSecurityPolicy(policy)}), nil
}

func (s *fakeSecurityService) DeleteSecurityPolicy(ctx context.Context, req *connect.Request[v1.DeleteSecurityPolicyRequest]) (*connect.Response[v1.DeleteSecurityPolicyResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.policies[req.Msg.GetSecurityPolicyId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("security policy not found"))
	}
	delete(s.policies, req.Msg.GetSecurityPolicyId())
	s.deleted = append(s.deleted, req.Msg.GetSecurityPolicyId())
	return connect.NewResponse(&v1.DeleteSecurityPolicyResponse{}), nil
}

func (s *fakeSecurityService) policyDeleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, deleted := range s.deleted {
		if deleted == id {
			return true
		}
	}
	return false
}

type fakeOrganizationService struct {
	v1connect.UnimplementedOrganizationServiceHandler

	mu       sync.Mutex
	policies *v1.OrganizationPolicies
	defaults *v1.OrganizationPolicies
}

func (s *fakeOrganizationService) GetAuthenticatedIdentity(ctx context.Context, req *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{
		OrganizationId: s.policies.GetOrganizationId(),
		Subject: &v1.Subject{
			Id:        "user-1",
			Principal: v1.Principal_PRINCIPAL_USER,
		},
	}), nil
}

func (s *fakeOrganizationService) GetIDToken(ctx context.Context, req *connect.Request[v1.GetIDTokenRequest]) (*connect.Response[v1.GetIDTokenResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("GetIDToken is not implemented"))
}

func (s *fakeOrganizationService) ExchangeToken(ctx context.Context, req *connect.Request[v1.ExchangeTokenRequest]) (*connect.Response[v1.ExchangeTokenResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("ExchangeToken is not implemented"))
}

func (s *fakeOrganizationService) GetOrganizationPolicies(ctx context.Context, req *connect.Request[v1.GetOrganizationPoliciesRequest]) (*connect.Response[v1.GetOrganizationPoliciesResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.policies.GetOrganizationId() {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("organization policies not found"))
	}
	return connect.NewResponse(&v1.GetOrganizationPoliciesResponse{Policies: cloneOrganizationPolicies(s.policies)}), nil
}

func (s *fakeOrganizationService) UpdateOrganizationPolicies(ctx context.Context, req *connect.Request[v1.UpdateOrganizationPoliciesRequest]) (*connect.Response[v1.UpdateOrganizationPoliciesResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.policies.GetOrganizationId() {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("organization policies not found"))
	}
	if req.Msg.MaximumEnvironmentTimeout != nil {
		s.policies.MaximumEnvironmentTimeout = req.Msg.MaximumEnvironmentTimeout
	}
	if req.Msg.MembersRequireProjects != nil {
		s.policies.MembersRequireProjects = req.Msg.GetMembersRequireProjects()
	}
	if req.Msg.MembersCreateProjects != nil {
		s.policies.MembersCreateProjects = req.Msg.GetMembersCreateProjects()
	}
	if len(req.Msg.AllowedEditorIds) > 0 {
		s.policies.AllowedEditorIds = append([]string(nil), req.Msg.AllowedEditorIds...)
	}
	if req.Msg.DefaultEditorId != nil {
		s.policies.DefaultEditorId = req.Msg.GetDefaultEditorId()
	}
	if req.Msg.AllowLocalRunners != nil {
		s.policies.AllowLocalRunners = req.Msg.GetAllowLocalRunners()
	}
	if req.Msg.MaximumRunningEnvironmentsPerUser != nil {
		s.policies.MaximumRunningEnvironmentsPerUser = req.Msg.GetMaximumRunningEnvironmentsPerUser()
	}
	if req.Msg.MaximumEnvironmentsPerUser != nil {
		s.policies.MaximumEnvironmentsPerUser = req.Msg.GetMaximumEnvironmentsPerUser()
	}
	if req.Msg.DefaultEnvironmentImage != nil {
		s.policies.DefaultEnvironmentImage = req.Msg.GetDefaultEnvironmentImage()
	}
	if req.Msg.PortSharingDisabled != nil {
		s.policies.PortSharingDisabled = req.Msg.GetPortSharingDisabled()
	}
	if req.Msg.DeleteArchivedEnvironmentsAfter != nil {
		s.policies.DeleteArchivedEnvironmentsAfter = req.Msg.DeleteArchivedEnvironmentsAfter
	}
	if req.Msg.MaximumEnvironmentLifetime != nil {
		s.policies.MaximumEnvironmentLifetime = req.Msg.MaximumEnvironmentLifetime
	}
	if req.Msg.RequireCustomDomainAccess != nil {
		s.policies.RequireCustomDomainAccess = req.Msg.GetRequireCustomDomainAccess()
	}
	if req.Msg.EditorVersionRestrictions != nil {
		s.policies.EditorVersionRestrictions = cloneEditorVersionRestrictions(req.Msg.EditorVersionRestrictions)
	}
	if req.Msg.RestrictAccountCreationToScim != nil {
		s.policies.RestrictAccountCreationToScim = req.Msg.GetRestrictAccountCreationToScim()
	}
	if req.Msg.WebBrowserDisabled != nil {
		s.policies.WebBrowserDisabled = req.Msg.GetWebBrowserDisabled()
	}
	if req.Msg.DisableFromScratch != nil {
		s.policies.DisableFromScratch = req.Msg.GetDisableFromScratch()
	}
	if req.Msg.SecurityPolicyId != nil {
		s.policies.SecurityPolicyId = req.Msg.GetSecurityPolicyId()
	}
	if req.Msg.ArchiveEnvironmentsAfter != nil {
		s.policies.ArchiveEnvironmentsAfter = req.Msg.ArchiveEnvironmentsAfter
	}
	if req.Msg.AgentPolicy != nil {
		applyAgentPolicyUpdate(s.policies.AgentPolicy, req.Msg.AgentPolicy)
	}
	return connect.NewResponse(&v1.UpdateOrganizationPoliciesResponse{}), nil
}

func (s *fakeOrganizationService) defaultsDiff() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if proto.Equal(s.defaults, s.policies) {
		return ""
	}
	defaults, _ := protojson.Marshal(s.defaults)
	actual, _ := protojson.Marshal(s.policies)
	return fmt.Sprintf("want %s, got %s", defaults, actual)
}

func newTestOrganizationPolicies(organizationID string) *v1.OrganizationPolicies {
	return &v1.OrganizationPolicies{
		OrganizationId:                    organizationID,
		MaximumEnvironmentTimeout:         durationpb.New(30 * time.Minute),
		MembersRequireProjects:            false,
		MembersCreateProjects:             true,
		AllowedEditorIds:                  []string{"editor-default"},
		DefaultEditorId:                   "editor-default",
		AllowLocalRunners:                 false,
		MaximumRunningEnvironmentsPerUser: 1,
		MaximumEnvironmentsPerUser:        3,
		DefaultEnvironmentImage:           "ubuntu:22.04",
		DeleteArchivedEnvironmentsAfter:   durationpb.New(24 * time.Hour),
		MaximumEnvironmentLifetime:        durationpb.New(720 * time.Hour),
		EditorVersionRestrictions: map[string]*v1.EditorVersionPolicy{
			"editor-default": {AllowedVersions: []string{"stable"}},
		},
		AgentPolicy: &v1.AgentPolicy{
			CommandDenyList: []string{"server-default-command"},
		},
		ArchiveEnvironmentsAfter: durationpb.New(24 * time.Hour),
	}
}

func applyAgentPolicyUpdate(policy *v1.AgentPolicy, update *v1.UpdateOrganizationPoliciesRequest_UpdateAgentPolicy) {
	if update.McpDisabled != nil {
		policy.McpDisabled = update.GetMcpDisabled()
	}
	if update.CommandDenyList != nil {
		policy.CommandDenyList = append([]string(nil), update.CommandDenyList...)
	}
	if update.ScmToolsDisabled != nil {
		policy.ScmToolsDisabled = update.GetScmToolsDisabled()
	}
	if update.ScmToolsAllowedGroupId != nil {
		policy.ScmToolsAllowedGroupId = update.GetScmToolsAllowedGroupId()
	}
	if update.ConversationSharingPolicy != nil {
		policy.ConversationSharingPolicy = update.GetConversationSharingPolicy()
	}
	if update.MaxSubagentsPerEnvironment != nil {
		policy.MaxSubagentsPerEnvironment = update.GetMaxSubagentsPerEnvironment()
	}
	policy.AllowedAgentIds = append([]string(nil), update.AllowedAgentIds...)
	policy.AllowedCodexModels = append([]v1.CodexOpenAIModel(nil), update.AllowedCodexModels...) //nolint:staticcheck // Existing Terraform schema still maps the legacy allowlist.
	policy.AllowedCodexReasoningEfforts = append([]v1.CodexReasoningEffort(nil), update.AllowedCodexReasoningEfforts...)
	policy.AllowedCodexServiceTiers = append([]v1.CodexServiceTier(nil), update.AllowedCodexServiceTiers...)
}

func cloneSecurityPolicy(policy *v1.SecurityPolicy) *v1.SecurityPolicy {
	cloned, ok := proto.Clone(policy).(*v1.SecurityPolicy)
	if !ok {
		return nil
	}
	return cloned
}

func cloneSecurityPolicyMetadata(metadata *v1.SecurityPolicy_Metadata) *v1.SecurityPolicy_Metadata {
	cloned, ok := proto.Clone(metadata).(*v1.SecurityPolicy_Metadata)
	if !ok {
		return nil
	}
	return cloned
}

func cloneSecurityPolicySpec(spec *v1.SecurityPolicy_Spec) *v1.SecurityPolicy_Spec {
	cloned, ok := proto.Clone(spec).(*v1.SecurityPolicy_Spec)
	if !ok {
		return nil
	}
	return cloned
}

func cloneOrganizationPolicies(policies *v1.OrganizationPolicies) *v1.OrganizationPolicies {
	cloned, ok := proto.Clone(policies).(*v1.OrganizationPolicies)
	if !ok {
		return nil
	}
	return cloned
}

func cloneEditorVersionRestrictions(restrictions map[string]*v1.EditorVersionPolicy) map[string]*v1.EditorVersionPolicy {
	result := make(map[string]*v1.EditorVersionPolicy, len(restrictions))
	for editorID, restriction := range restrictions {
		cloned, ok := proto.Clone(restriction).(*v1.EditorVersionPolicy)
		if !ok {
			continue
		}
		result[editorID] = cloned
	}
	return result
}
