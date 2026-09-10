// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/gitpod-sdk-go/v1/v1connect"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"google.golang.org/protobuf/proto"
)

const githubAppIntegrationID = "01980ed3-a090-7b5b-a74c-9bf5d8cfe501"
const githubAppAddress = "ona_github_app_integration.test"

func TestAccGitHubAppLifecycle(t *testing.T) {
	t.Parallel()
	server, service := newGitHubAppAPIServer(t)
	step := func(version int, enabled bool) resource.TestStep {
		return resource.TestStep{Config: githubAppConfig(server.URL, "123", version, enabled), ConfigVariables: githubAppCredentials(version), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{githubAppSecretsCheck{}}}, ConfigStateChecks: []statecheck.StateCheck{githubAppSecretsCheck{}}}
	}
	create := step(1, true)
	create.Check = resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(githubAppAddress, "id", githubAppIntegrationID), resource.TestCheckResourceAttr(githubAppAddress, "app_slug", "owned-app"), resource.TestCheckNoResourceAttr(githubAppAddress, "credentials"), resource.TestCheckNoResourceAttr(githubAppAddress, "installation"))
	noop := step(1, true)
	noop.ConfigPlanChecks.PreApply = append(noop.ConfigPlanChecks.PreApply, plancheck.ExpectEmptyPlan())
	secretOnly := step(1, true)
	secretOnly.ConfigVariables = githubAppCredentials(2)
	secretOnly.ConfigPlanChecks.PreApply = append(secretOnly.ConfigPlanChecks.PreApply, plancheck.ExpectEmptyPlan())
	noBundle := step(1, true)
	noBundle.Config = strings.ReplaceAll(noBundle.Config, "credentials = var.github_app_credentials", "")
	noBundle.ConfigPlanChecks.PreApply = append(noBundle.ConfigPlanChecks.PreApply, plancheck.ExpectEmptyPlan())
	rotation := step(2, true)
	rotation.PreConfig = func() {
		service.mu.Lock()
		defer service.mu.Unlock()
		service.integrations[githubAppIntegrationID].ExternalInstallation = &v1.IntegrationExternalInstallation{Id: "42", AccountName: "my-org", AccountType: "Organization"}
	}
	rotation.ConfigPlanChecks.PreApply = append(rotation.ConfigPlanChecks.PreApply, plancheck.ExpectResourceAction(githubAppAddress, plancheck.ResourceActionUpdate))
	rotation.Check = resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(githubAppAddress, "id", githubAppIntegrationID), resource.TestCheckResourceAttr(githubAppAddress, "app_slug", "renamed-app"), resource.TestCheckResourceAttr(githubAppAddress, "installation.id", "42"))
	rotatedNoop := step(2, true)
	rotatedNoop.ConfigPlanChecks.PreApply = append(rotatedNoop.ConfigPlanChecks.PreApply, plancheck.ExpectEmptyPlan())
	disable := step(2, false)
	disable.Check = resource.TestCheckResourceAttr(githubAppAddress, "enabled", "false")
	enable := step(2, true)
	enable.Check = resource.TestCheckResourceAttr(githubAppAddress, "enabled", "true")
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		create, noop, secretOnly, noBundle, rotation, rotatedNoop, disable, enable,
		{ResourceName: githubAppAddress, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"credentials_version"}, ConfigVariables: githubAppCredentials(2)},
	}})
	type Expectation struct {
		Creates, Rotations, Deletes, Remaining, TokenCalls int
		AuthFields                                         []string
	}
	service.mu.Lock()
	got := Expectation{Creates: service.creates, Rotations: service.rotations, Deletes: service.deletes, Remaining: len(service.integrations), TokenCalls: service.tokenCalls, AuthFields: service.authFields}
	service.mu.Unlock()
	want := Expectation{Creates: 1, Rotations: 1, Deletes: 1, AuthFields: []string{"app_id", "client_secret", "private_key", "webhook_secret"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("API lifecycle mismatch (-want +got):\n%s", diff)
	}
}

func TestAccGitHubAppImportNoop(t *testing.T) {
	t.Parallel()
	for _, structured := range []bool{false, true} {
		t.Run(fmt.Sprintf("structured_%t", structured), func(t *testing.T) {
			t.Parallel()
			server, service := newGitHubAppAPIServer(t)
			service.integrations[githubAppIntegrationID] = githubAppRemote(githubAppIntegrationID, true)
			importID := fmt.Sprintf("id = %q", githubAppIntegrationID)
			if structured {
				importID = fmt.Sprintf("identity = { integration_id = %q }", githubAppIntegrationID)
			}
			cfg := QueryProviderConfig(server.URL) + fmt.Sprintf("\nimport {\n to = %s\n %s\n}\nresource \"ona_github_app_integration\" \"test\" { app_id = \"123\" }\n", githubAppAddress, importID)
			resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: cfg, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckNoResourceAttr(githubAppAddress, "credentials"), resource.TestCheckNoResourceAttr(githubAppAddress, "credentials_version"))}, {Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}}}})
			type Expectation struct{ Creates, Rotations int }
			service.mu.Lock()
			got := Expectation{service.creates, service.rotations}
			service.mu.Unlock()
			if diff := cmp.Diff(Expectation{}, got); diff != "" {
				t.Errorf("import mutation mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAccGitHubAppImportRejectsSharedAndCustom(t *testing.T) {
	t.Parallel()
	for _, custom := range []bool{false, true} {
		t.Run(fmt.Sprintf("custom_%t", custom), func(t *testing.T) {
			t.Parallel()
			server, service := newGitHubAppAPIServer(t)
			remote := githubAppRemote(githubAppIntegrationID, custom)
			if custom {
				remote.IntegrationDefinitionId = ""
				remote.Host = "github.com"
			}
			service.integrations[githubAppIntegrationID] = remote
			cfg := QueryProviderConfig(server.URL) + fmt.Sprintf("\nimport {\n to = %s\n id = %q\n}\nresource \"ona_github_app_integration\" \"test\" { app_id = \"123\" }\n", githubAppAddress, githubAppIntegrationID)
			resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: cfg, ExpectError: regexp.MustCompile("Not an Organization-Owned GitHub App")}}})
		})
	}
}

func TestAccGitHubAppEmptyCredentialsImport(t *testing.T) {
	t.Parallel()
	tests := []struct{ Name, Credentials, Version string }{
		{Name: "generated_empty_object", Credentials: "{}"},
		{Name: "empty_object_null_version", Credentials: "{}", Version: "credentials_version = null"},
		{Name: "all_null_credentials", Credentials: "{ private_key = null, client_secret = null, webhook_secret = null }", Version: "credentials_version = null"},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			server, service := newGitHubAppAPIServer(t)
			service.integrations[githubAppIntegrationID] = githubAppRemote(githubAppIntegrationID, true)
			cfg := QueryProviderConfig(server.URL) + fmt.Sprintf(`
import {
  to = ona_github_app_integration.test
  identity = { integration_id = %q }
}
resource "ona_github_app_integration" "test" {
  app_id = "123"
  credentials = %s
  %s
}
`, githubAppIntegrationID, tc.Credentials, tc.Version)
			resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
				{Config: cfg, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckNoResourceAttr(githubAppAddress, "credentials"), resource.TestCheckNoResourceAttr(githubAppAddress, "credentials_version"))},
				{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			}})
			type Expectation struct{ Creates, Rotations int }
			service.mu.Lock()
			got := Expectation{service.creates, service.rotations}
			service.mu.Unlock()
			if diff := cmp.Diff(Expectation{}, got); diff != "" {
				t.Errorf("empty-object import mutation mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAccGitHubAppEmptyCredentialsMutationRejected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name, Credentials, Error string
		Rotate                   bool
	}{
		{Name: "create_empty", Credentials: "{}", Error: "Missing GitHub App Credentials"},
		{Name: "create_all_null", Credentials: "{ private_key = null, client_secret = null, webhook_secret = null }", Error: "Missing GitHub App Credentials"},
		{Name: "rotate_empty", Credentials: "{}", Error: "Missing GitHub App Credentials", Rotate: true},
		{Name: "rotate_all_null", Credentials: "{ private_key = null, client_secret = null, webhook_secret = null }", Error: "Missing GitHub App Credentials", Rotate: true},
		{Name: "create_partial", Credentials: `{ private_key = "private-only" }`, Error: "Incomplete GitHub App Credentials"},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			server, service := newGitHubAppAPIServer(t)
			var steps []resource.TestStep
			if tc.Rotate {
				steps = append(steps, resource.TestStep{Config: githubAppConfig(server.URL, "123", 1, true), ConfigVariables: githubAppCredentials(1)})
			}
			cfg := QueryProviderConfig(server.URL) + fmt.Sprintf(`
resource "ona_github_app_integration" "test" {
  app_id = "123"
  credentials = %s
  credentials_version = 2
}
`, tc.Credentials)
			steps = append(steps, resource.TestStep{Config: cfg, ExpectError: regexp.MustCompile(tc.Error)})
			resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: steps})
			type Expectation struct{ Creates, Updates int }
			want := Expectation{}
			if tc.Rotate {
				want.Creates = 1
			}
			service.mu.Lock()
			got := Expectation{service.creates, service.updates}
			service.mu.Unlock()
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("invalid credentials reached API (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAccGitHubAppIDRequiresReplacement(t *testing.T) {
	t.Parallel()
	server, _ := newGitHubAppAPIServer(t)
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: githubAppConfig(server.URL, "123", 1, true), ConfigVariables: githubAppCredentials(1)}, {Config: githubAppConfig(server.URL, "124", 2, true), ConfigVariables: githubAppCredentials(2), PlanOnly: true, ExpectNonEmptyPlan: true, ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectResourceAction(githubAppAddress, plancheck.ResourceActionDestroyBeforeCreate)}}}}})
}

func TestAccGitHubAppUnknownAppIDAndDefaultEnabled(t *testing.T) {
	t.Parallel()
	server, _ := newGitHubAppAPIServer(t)
	cfg := githubAppConfig(server.URL, "123", 1, true) + "\nresource \"terraform_data\" \"app\" { input = \"123\" }\n"
	cfg = strings.ReplaceAll(cfg, "app_id = \"123\"", "app_id = terraform_data.app.output")
	cfg = strings.ReplaceAll(cfg, "enabled = true", "")
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: cfg, ConfigVariables: githubAppCredentials(1), Check: resource.TestCheckResourceAttr(githubAppAddress, "enabled", "true")}, {Config: cfg, ConfigVariables: githubAppCredentials(1), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}}}})
}

func githubAppConfig(host, appID string, version int, enabled bool) string {
	return QueryProviderConfig(host) + fmt.Sprintf(`
variable "github_app_credentials" {
  type = object({ private_key = string, client_secret = string, webhook_secret = string })
  ephemeral = true
  sensitive = true
}
resource "ona_github_app_integration" "test" {
  app_id = %q
  enabled = %t
  credentials_version = %d
  credentials = var.github_app_credentials
}
`, appID, enabled, version)
}

func githubAppCredentials(version int) config.Variables {
	return config.Variables{"github_app_credentials": config.MapVariable(map[string]config.Variable{
		"private_key": config.StringVariable(fmt.Sprintf("private-must-not-appear-%d", version)), "client_secret": config.StringVariable(fmt.Sprintf("client-must-not-appear-%d", version)), "webhook_secret": config.StringVariable(fmt.Sprintf("webhook-must-not-appear-%d", version)),
	})}
}

type githubAppSecretsCheck struct{}

func (githubAppSecretsCheck) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	resp.Error = githubAppCheckSecrets(req.Plan)
}
func (githubAppSecretsCheck) CheckState(_ context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	resp.Error = githubAppCheckSecrets(req.State)
}
func githubAppCheckSecrets(value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode Terraform artifact: %w", err)
	}
	if strings.Contains(string(raw), "must-not-appear") {
		return fmt.Errorf("GitHub App credentials appeared in a Terraform artifact")
	}
	return nil
}

type githubAppService struct {
	v1connect.UnimplementedIntegrationServiceHandler
	mu                                      sync.Mutex
	integrations                            map[string]*v1.Integration
	creates, rotations, deletes, tokenCalls int
	updates                                 int
	authFields                              []string
}

func newGitHubAppAPIServer(t *testing.T) (*httptest.Server, *githubAppService) {
	t.Helper()
	s := &githubAppService{integrations: map[string]*v1.Integration{}}
	_, handler := v1connect.NewIntegrationServiceHandler(s)
	server := httptest.NewServer(http.StripPrefix("/api", handler))
	t.Cleanup(server.Close)
	return server, s
}

func githubAppRemote(id string, owned bool) *v1.Integration {
	app := githubSharedAppMetadata()
	if owned {
		app = &v1.IntegrationProprietaryAppConfig{AppId: "123", AppSlug: "owned-app", ClientId: "client-id"}
	}
	return &v1.Integration{Id: id, OrganizationId: "org-1", IntegrationDefinitionId: "github-app-definition", Enabled: true, Name: "GitHub", Auth: &v1.IntegrationAuthentication{RequiresAuth: true, ProprietaryApp: app}}
}

func githubSharedAppMetadata() *v1.IntegrationProprietaryAppConfig {
	return &v1.IntegrationProprietaryAppConfig{AppId: "456", AppSlug: "shared-app", ClientId: "shared-client-id"}
}

func (s *githubAppService) ListIntegrationDefinitions(_ context.Context, req *connect.Request[v1.ListIntegrationDefinitionsRequest]) (*connect.Response[v1.ListIntegrationDefinitionsResponse], error) {
	if req.Msg.GetPagination().GetToken() == "" {
		return connect.NewResponse(&v1.ListIntegrationDefinitionsResponse{Pagination: &v1.PaginationResponse{NextToken: "github"}, Definitions: []*v1.IntegrationDefinition{{Id: "other-definition", Host: "other.com"}}}), nil
	}
	return connect.NewResponse(&v1.ListIntegrationDefinitionsResponse{Definitions: []*v1.IntegrationDefinition{{Id: "github-app-definition", Name: "GitHub", Host: "github.com", Auth: &v1.IntegrationAuthentication{ProprietaryApp: githubSharedAppMetadata(), Oauth: &v1.IntegrationOAuthConfig{RedirectUrl: "https://canonical.example.com/auth/github/callback"}}}}}), nil
}

func (s *githubAppService) CreateIntegration(_ context.Context, req *connect.Request[v1.CreateIntegrationRequest]) (*connect.Response[v1.CreateIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creates++
	app := req.Msg.GetAuth().GetProprietaryApp()
	if !req.Msg.GetAuth().GetRequiresAuth() || app.GetAppId() == "" || app.GetPrivateKey() == "" || app.GetClientSecret() == "" || app.GetWebhookSecret() == "" || app.GetAppSlug() != "" || app.GetClientId() != "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("expected minimal App credential bundle"))
	}
	s.authFields = []string{"app_id", "client_secret", "private_key", "webhook_secret"}
	remote := githubAppRemote(githubAppIntegrationID, true)
	remote.Enabled = req.Msg.GetEnabled()
	remote.Auth.ProprietaryApp.AppId = app.GetAppId()
	s.integrations[remote.Id] = remote
	return connect.NewResponse(&v1.CreateIntegrationResponse{Integration: proto.CloneOf(remote)}), nil
}

func (s *githubAppService) GetIntegration(_ context.Context, req *connect.Request[v1.GetIntegrationRequest]) (*connect.Response[v1.GetIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	remote := s.integrations[req.Msg.GetId()]
	if remote == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("not found"))
	}
	return connect.NewResponse(&v1.GetIntegrationResponse{Integration: proto.CloneOf(remote)}), nil
}

func (s *githubAppService) UpdateIntegration(_ context.Context, req *connect.Request[v1.UpdateIntegrationRequest]) (*connect.Response[v1.UpdateIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates++
	remote := s.integrations[req.Msg.GetId()]
	if remote == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("not found"))
	}
	if req.Msg.Enabled != nil {
		remote.Enabled = req.Msg.GetEnabled()
	}
	if req.Msg.Auth != nil {
		s.rotations++
		app := req.Msg.GetAuth().GetProprietaryApp()
		if app.GetAppId() != remote.Auth.ProprietaryApp.AppId || app.GetPrivateKey() == "" || app.GetClientSecret() == "" || app.GetWebhookSecret() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("rotation requires the same App and complete credentials"))
		}
		remote.Auth.ProprietaryApp.AppSlug = "renamed-app"
	}
	return connect.NewResponse(&v1.UpdateIntegrationResponse{Integration: proto.CloneOf(remote)}), nil
}

func (s *githubAppService) DeleteIntegration(_ context.Context, req *connect.Request[v1.DeleteIntegrationRequest]) (*connect.Response[v1.DeleteIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes++
	delete(s.integrations, req.Msg.GetId())
	return connect.NewResponse(&v1.DeleteIntegrationResponse{}), nil
}

func (s *githubAppService) ListIntegrations(_ context.Context, req *connect.Request[v1.ListIntegrationsRequest]) (*connect.Response[v1.ListIntegrationsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.integrations))
	for id := range s.integrations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	index, _ := strconv.Atoi(req.Msg.GetPagination().GetToken())
	resp := &v1.ListIntegrationsResponse{}
	if index < len(ids) {
		resp.Integrations = []*v1.Integration{proto.CloneOf(s.integrations[ids[index]])}
		if index+1 < len(ids) {
			resp.Pagination = &v1.PaginationResponse{NextToken: strconv.Itoa(index + 1)}
		}
	}
	return connect.NewResponse(resp), nil
}

func (s *githubAppService) ValidateIntegration(_ context.Context, _ *connect.Request[v1.ValidateIntegrationRequest]) (*connect.Response[v1.ValidateIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokenCalls++
	return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("refresh must not validate GitHub installation"))
}
