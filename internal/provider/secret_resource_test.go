// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	secretTestOrgID            = "01980ed3-a090-7b5b-a74c-9bf5d8cfe500"
	secretTestProjectID        = "01980ed3-a090-7b5b-a74c-9bf5d8cfe501"
	secretTestUserID           = "01980ed3-a090-7b5b-a74c-9bf5d8cfe502"
	secretTestServiceAccountID = "01980ed3-a090-7b5b-a74c-9bf5d8cfe503"
	secretTestCreatorID        = "01980ed3-a090-7b5b-a74c-9bf5d8cfe505"
)

func TestAccSecretResourceLifecycle(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newSecretAPIServer(t)
	t.Cleanup(server.Close)
	server.service.expectCreateValueV1()
	server.service.expectUpdateValue(secretTestValueV2)
	server.service.enablePagedOrganizationList()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.secretDeleted(secretTestSecretID(1)) {
				return errors.New("secret was not deleted")
			}
			if server.service.getSecretValueCalls() != 0 {
				return errors.New("GetSecretValue was called")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationSecretConfig(server.URL, "THIRD_PARTY_API_KEY", secretTestValueV1, "v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_secret.test", "id", secretTestSecretID(1)),
					resource.TestCheckResourceAttr("ona_secret.test", "scope", "organization"),
					resource.TestCheckNoResourceAttr("ona_secret.test", "organization_id"),
					resource.TestCheckNoResourceAttr("ona_secret.test", "updated_at"),
					resource.TestCheckResourceAttr("ona_secret.test", "name", "THIRD_PARTY_API_KEY"),
					resource.TestCheckResourceAttr("ona_secret.test", "environment_variable", "true"),
					resource.TestCheckResourceAttr("ona_secret.test", "value_version", "v1"),
					resource.TestCheckResourceAttr("ona_secret.test", "creator.id", secretTestCreatorID),
					resource.TestCheckResourceAttr("ona_secret.test", "creator.principal", "service_account"),
					resource.TestCheckNoResourceAttr("ona_secret.test", "value"),
					func(state *terraform.State) error {
						if !server.service.createValueMatched(secretTestSecretID(1)) {
							return errors.New("create did not receive the configured write-only value")
						}
						return nil
					},
				),
			},
			{
				Config: testAccOrganizationSecretConfig(server.URL, "THIRD_PARTY_API_KEY", secretTestValueV1, "v1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:            "ona_secret.test",
				ImportState:             true,
				ImportStateId:           "organization/" + secretTestSecretID(1),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value", "value_version"},
			},
			{
				Config: testAccOrganizationSecretConfig(server.URL, "THIRD_PARTY_API_KEY", secretTestValueV2, "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_secret.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_secret.test", "value_version", "v2"),
					resource.TestCheckNoResourceAttr("ona_secret.test", "value"),
					func(state *terraform.State) error {
						if !server.service.updateValueMatched(secretTestSecretID(1)) {
							return errors.New("update did not receive the configured write-only value")
						}
						return nil
					},
				),
			},
			{
				Config: testAccOrganizationSecretConfig(server.URL, "RENAMED_API_KEY", secretTestValueV2, "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_secret.test", plancheck.ResourceActionReplace),
					},
				},
			},
			{
				Config: testAccOrganizationSecretFileConfig(server.URL, secretTestValueV2, "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_secret.test", plancheck.ResourceActionReplace),
					},
				},
			},
			{
				Config: testAccOrganizationSecretCredentialProxyConfig(server.URL, secretTestValueV2, "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_secret.test", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})
}

func TestAccSecretResourceScopes(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	tests := []struct {
		Name           string
		Config         func(host string) string
		ImportID       string
		ResourceChecks []resource.TestCheckFunc
	}{
		{
			Name:     "project",
			Config:   testAccProjectSecretConfig,
			ImportID: "project/" + secretTestProjectID + "/" + secretTestSecretID(1),
			ResourceChecks: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("ona_secret.test", "scope", "project"),
				resource.TestCheckResourceAttr("ona_secret.test", "project_id", secretTestProjectID),
				resource.TestCheckResourceAttr("ona_secret.test", "file_path", "/workspace/.secret"),
			},
		},
		{
			Name:     "user_explicit",
			Config:   testAccUserSecretConfig,
			ImportID: "user/" + secretTestUserID + "/" + secretTestSecretID(1),
			ResourceChecks: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("ona_secret.test", "scope", "user"),
				resource.TestCheckResourceAttr("ona_secret.test", "user_id", secretTestUserID),
				resource.TestCheckResourceAttr("ona_secret.test", "api_only", "true"),
			},
		},
		{
			Name:     "service_account",
			Config:   testAccServiceAccountSecretConfig,
			ImportID: "service_account/" + secretTestServiceAccountID + "/" + secretTestSecretID(1),
			ResourceChecks: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("ona_secret.test", "scope", "service_account"),
				resource.TestCheckResourceAttr("ona_secret.test", "service_account_id", secretTestServiceAccountID),
				resource.TestCheckResourceAttr("ona_secret.test", "container_registry_basic_auth_host", "registry.example.com"),
				resource.TestCheckResourceAttr("ona_secret.test", "credential_proxy.0.header", "Authorization"),
			},
		},
		{
			Name:     "user_inferred",
			Config:   testAccInferredUserSecretConfig,
			ImportID: "user/" + secretTestUserID + "/" + secretTestSecretID(1),
			ResourceChecks: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("ona_secret.test", "scope", "user"),
				resource.TestCheckResourceAttr("ona_secret.test", "user_id", secretTestUserID),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
			server := newSecretAPIServer(t)
			t.Cleanup(server.Close)
			server.service.expectCreateValueV1()

			checks := []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("ona_secret.test", "id", secretTestSecretID(1)),
				resource.TestCheckResourceAttr("ona_secret.test", "name", "SCOPE_SECRET"),
				resource.TestCheckNoResourceAttr("ona_secret.test", "value"),
			}
			checks = append(checks, tc.ResourceChecks...)

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: tc.Config(server.URL),
						Check:  resource.ComposeAggregateTestCheckFunc(checks...),
					},
					{
						Config: tc.Config(server.URL),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{
								plancheck.ExpectEmptyPlan(),
							},
						},
					},
					{
						ResourceName:            "ona_secret.test",
						ImportState:             true,
						ImportStateId:           tc.ImportID,
						ImportStateVerify:       true,
						ImportStateVerifyIgnore: []string{"value", "value_version"},
					},
				},
			})
		})
	}
}

func TestAccSecretResourceRemoteMetadataDriftPlansReplacement(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newSecretAPIServer(t)
	t.Cleanup(server.Close)
	server.service.expectCreateValueV1()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             testAccOrganizationSecretConfig(server.URL, "THIRD_PARTY_API_KEY", secretTestValueV1, "v1"),
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_secret.test", "name", "THIRD_PARTY_API_KEY"),
					func(state *terraform.State) error {
						server.service.renameSecret(secretTestSecretID(1), "REMOTE_DRIFTED_API_KEY")
						return nil
					},
				),
			},
			{
				Config: testAccOrganizationSecretConfig(server.URL, "THIRD_PARTY_API_KEY", secretTestValueV1, "v1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_secret.test", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})
}

func TestAccSecretResourceCreateKeepsStateWhenListMisses(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	server := newSecretAPIServer(t)
	t.Cleanup(server.Close)
	server.service.expectCreateValueV1()
	server.service.omitSecretsFromList()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationSecretConfig(server.URL, "THIRD_PARTY_API_KEY", secretTestValueV1, "v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_secret.test", "id", secretTestSecretID(1)),
					resource.TestCheckResourceAttr("ona_secret.test", "name", "THIRD_PARTY_API_KEY"),
					resource.TestCheckNoResourceAttr("ona_secret.test", "value"),
				),
			},
		},
	})
}

type secretAPIServer struct {
	*httptest.Server
	service *fakeSecretService
}

func newSecretAPIServer(t *testing.T) *secretAPIServer {
	t.Helper()

	service := &fakeSecretService{
		secrets: map[string]*v1.Secret{},
		deleted: map[string]bool{},
	}
	secretPath, secretHandler := v1connect.NewSecretServiceHandler(service)
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(service)
	mux := http.NewServeMux()
	mux.Handle(secretPath, secretHandler)
	mux.Handle(identityPath, identityHandler)

	server := httptest.NewServer(http.StripPrefix("/api", mux))
	return &secretAPIServer{Server: server, service: service}
}

type fakeSecretService struct {
	v1connect.UnimplementedSecretServiceHandler
	v1connect.UnimplementedIdentityServiceHandler

	mu                   sync.Mutex
	secrets              map[string]*v1.Secret
	deleted              map[string]bool
	createValue          string
	updateValue          string
	createValueMatches   map[string]bool
	updateValueMatches   map[string]bool
	getValueCalls        int
	nextID               int
	pageOrganizationList bool
	omitListSecrets      bool
}

func (s *fakeSecretService) GetAuthenticatedIdentity(ctx context.Context, req *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{
		OrganizationId: secretTestOrgID,
		Subject: &v1.Subject{
			Id:        secretTestUserID,
			Principal: v1.Principal_PRINCIPAL_USER,
		},
	}), nil
}

func (s *fakeSecretService) CreateSecret(ctx context.Context, req *connect.Request[v1.CreateSecretRequest]) (*connect.Response[v1.CreateSecretResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	id := secretTestSecretID(s.nextID)
	if s.createValueMatches == nil {
		s.createValueMatches = map[string]bool{}
	}
	s.createValueMatches[id] = req.Msg.GetValue() == s.createValue

	secret := &v1.Secret{
		Id:              id,
		Name:            req.Msg.GetName(),
		CreatedAt:       timestamppb.New(secretTestCreatedAt),
		UpdatedAt:       timestamppb.New(secretTestCreatedAt),
		Creator:         &v1.Subject{Id: secretTestCreatorID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT},
		Scope:           cloneSecretScope(req.Msg.GetScope()),
		CredentialProxy: cloneCredentialProxy(req.Msg.GetCredentialProxy()),
	}
	switch req.Msg.GetMount().(type) {
	case *v1.CreateSecretRequest_EnvironmentVariable:
		secret.Mount = &v1.Secret_EnvironmentVariable{EnvironmentVariable: req.Msg.GetEnvironmentVariable()}
	case *v1.CreateSecretRequest_FilePath:
		secret.Mount = &v1.Secret_FilePath{FilePath: req.Msg.GetFilePath()}
	case *v1.CreateSecretRequest_ContainerRegistryBasicAuthHost:
		secret.Mount = &v1.Secret_ContainerRegistryBasicAuthHost{ContainerRegistryBasicAuthHost: req.Msg.GetContainerRegistryBasicAuthHost()}
	case *v1.CreateSecretRequest_ApiOnly:
		secret.Mount = &v1.Secret_ApiOnly{ApiOnly: req.Msg.GetApiOnly()}
	}
	s.secrets[id] = secret
	return connect.NewResponse(&v1.CreateSecretResponse{Secret: cloneSecret(secret)}), nil
}

func (s *fakeSecretService) UpdateSecretValue(ctx context.Context, req *connect.Request[v1.UpdateSecretValueRequest]) (*connect.Response[v1.UpdateSecretValueResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	secret := s.secrets[req.Msg.GetSecretId()]
	if secret == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("secret not found"))
	}
	if s.updateValueMatches == nil {
		s.updateValueMatches = map[string]bool{}
	}
	s.updateValueMatches[req.Msg.GetSecretId()] = req.Msg.GetValue() == s.updateValue
	secret.UpdatedAt = timestamppb.New(secretTestUpdatedAt)
	return connect.NewResponse(&v1.UpdateSecretValueResponse{}), nil
}

func (s *fakeSecretService) ListSecrets(ctx context.Context, req *connect.Request[v1.ListSecretsRequest]) (*connect.Response[v1.ListSecretsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.omitListSecrets {
		return connect.NewResponse(&v1.ListSecretsResponse{
			Pagination: &v1.PaginationResponse{},
		}), nil
	}

	var secrets []*v1.Secret
	for _, secret := range s.secrets {
		if sameSecretScope(secret.GetScope(), req.Msg.GetFilter().GetScope()) {
			secrets = append(secrets, cloneSecret(secret))
		}
	}

	if s.pageOrganizationList && req.Msg.GetFilter().GetScope().GetOrganizationId() != "" {
		if req.Msg.GetPagination().GetToken() == "" {
			return connect.NewResponse(&v1.ListSecretsResponse{
				Pagination: &v1.PaginationResponse{NextToken: "next"},
				Secrets: []*v1.Secret{
					{
						Id:    secretTestSecretID(99),
						Name:  "DECOY_SECRET",
						Scope: cloneSecretScope(req.Msg.GetFilter().GetScope()),
						Mount: &v1.Secret_EnvironmentVariable{EnvironmentVariable: true},
					},
				},
			}), nil
		}
	}

	return connect.NewResponse(&v1.ListSecretsResponse{
		Pagination: &v1.PaginationResponse{},
		Secrets:    secrets,
	}), nil
}

func (s *fakeSecretService) DeleteSecret(ctx context.Context, req *connect.Request[v1.DeleteSecretRequest]) (*connect.Response[v1.DeleteSecretResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.secrets[req.Msg.GetSecretId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("secret not found"))
	}
	delete(s.secrets, req.Msg.GetSecretId())
	s.deleted[req.Msg.GetSecretId()] = true
	return connect.NewResponse(&v1.DeleteSecretResponse{}), nil
}

func (s *fakeSecretService) GetSecretValue(ctx context.Context, req *connect.Request[v1.GetSecretValueRequest]) (*connect.Response[v1.GetSecretValueResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.getValueCalls++
	return nil, connect.NewError(connect.CodePermissionDenied, errors.New("GetSecretValue must not be used by the Terraform provider"))
}

func (s *fakeSecretService) expectCreateValueV1() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.createValue = secretTestValueV1
}

func (s *fakeSecretService) expectUpdateValue(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.updateValue = value
}

func (s *fakeSecretService) enablePagedOrganizationList() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pageOrganizationList = true
}

func (s *fakeSecretService) omitSecretsFromList() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.omitListSecrets = true
}

func (s *fakeSecretService) renameSecret(id string, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if secret := s.secrets[id]; secret != nil {
		secret.Name = name
	}
}

func (s *fakeSecretService) createValueMatched(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.createValueMatches[id]
}

func (s *fakeSecretService) updateValueMatched(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.updateValueMatches[id]
}

func (s *fakeSecretService) secretDeleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.deleted[id]
}

func (s *fakeSecretService) getSecretValueCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getValueCalls
}

func testAccOrganizationSecretConfig(host string, name string, value string, valueVersion string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_secret" "test" {
  scope = "organization"
  name  = %[2]q

  value         = %[3]q
  value_version = %[4]q

  environment_variable = true
}
`, host, name, value, valueVersion)
}

func testAccOrganizationSecretFileConfig(host string, value string, valueVersion string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_secret" "test" {
  scope = "organization"
  name  = "THIRD_PARTY_API_KEY"

  value         = %[2]q
  value_version = %[3]q

  file_path = "/workspace/.third-party-api-key"
}
`, host, value, valueVersion)
}

func testAccOrganizationSecretCredentialProxyConfig(host string, value string, valueVersion string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_secret" "test" {
  scope = "organization"
  name  = "THIRD_PARTY_API_KEY"

  value         = %[2]q
  value_version = %[3]q

  environment_variable = true

  credential_proxy {
    target_hosts = ["github.com"]
    header       = "Authorization"
  }
}
`, host, value, valueVersion)
}

func testAccProjectSecretConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_secret" "test" {
  scope      = "project"
  project_id = %[2]q
  name       = "SCOPE_SECRET"

  value         = %[3]q
  value_version = "v1"

  file_path = "/workspace/.secret"
}
`, host, secretTestProjectID, secretTestValueV1)
}

func testAccUserSecretConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_secret" "test" {
  scope   = "user"
  user_id = %[2]q
  name    = "SCOPE_SECRET"

  value         = %[3]q
  value_version = "v1"

  api_only = true
}
`, host, secretTestUserID, secretTestValueV1)
}

func testAccInferredUserSecretConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_secret" "test" {
  scope = "user"
  name  = "SCOPE_SECRET"

  value         = %[2]q
  value_version = "v1"

  api_only = true
}
`, host, secretTestValueV1)
}

func testAccServiceAccountSecretConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_secret" "test" {
  scope              = "service_account"
  service_account_id = %[2]q
  name               = "SCOPE_SECRET"

  value         = %[3]q
  value_version = "v1"

  container_registry_basic_auth_host = "registry.example.com"

  credential_proxy {
    target_hosts = ["registry.example.com"]
    header       = "Authorization"
  }
}
`, host, secretTestServiceAccountID, secretTestValueV1)
}

func sameSecretScope(a *v1.SecretScope, b *v1.SecretScope) bool {
	switch {
	case a.GetOrganizationId() != "" || b.GetOrganizationId() != "":
		return a.GetOrganizationId() == b.GetOrganizationId()
	case a.GetProjectId() != "" || b.GetProjectId() != "":
		return a.GetProjectId() == b.GetProjectId()
	case a.GetUserId() != "" || b.GetUserId() != "":
		return a.GetUserId() == b.GetUserId()
	case a.GetServiceAccountId() != "" || b.GetServiceAccountId() != "":
		return a.GetServiceAccountId() == b.GetServiceAccountId()
	default:
		return false
	}
}

func cloneSecret(secret *v1.Secret) *v1.Secret {
	if secret == nil {
		return nil
	}
	result, ok := proto.Clone(secret).(*v1.Secret)
	if !ok {
		return nil
	}
	return result
}

func cloneCredentialProxy(proxy *v1.Secret_CredentialProxy) *v1.Secret_CredentialProxy {
	if proxy == nil {
		return nil
	}
	result, ok := proto.Clone(proxy).(*v1.Secret_CredentialProxy)
	if !ok {
		return nil
	}
	return result
}

func cloneSecretScope(scope *v1.SecretScope) *v1.SecretScope {
	if scope == nil {
		return nil
	}
	result, ok := proto.Clone(scope).(*v1.SecretScope)
	if !ok {
		return nil
	}
	return result
}

func secretTestSecretID(n int) string {
	return fmt.Sprintf("01980ed3-a090-7b5b-a74c-9bf5d8cfe5%02d", n)
}

var (
	secretTestCreatedAt = timestamppb.Now().AsTime()
	secretTestUpdatedAt = secretTestCreatedAt.Add(1)
)

const (
	secretTestValueV1 = "test-secret-value-v1"
	secretTestValueV2 = "test-secret-value-v2"
)
