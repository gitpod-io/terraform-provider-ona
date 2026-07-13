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
	"sync"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"
)

func TestAccSCMIntegrationResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.scmDeleted("scm-1") {
				return errors.New("scm-1 was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccSCMIntegrationOAuthConfig(server.URL, "client-1", "secret-1", "v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_scm_integration.test", "id", "scm-1"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "runner_id", "runner-1"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "scm_id", "github"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "host", "github.com"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "auth_mode", "oauth"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "oauth_client_id", "client-1"),
					resource.TestCheckNoResourceAttr("ona_scm_integration.test", "oauth_client_secret"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "oauth_client_secret_version", "v1"),
				),
			},
			{
				Config: testAccSCMIntegrationOAuthConfig(server.URL, "client-1", "secret-1", "v1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:            "ona_scm_integration.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"oauth_client_secret", "oauth_client_secret_version"},
			},
			{
				Config: testAccSCMIntegrationOAuthConfig(server.URL, "client-2", "secret-2", "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_scm_integration.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_scm_integration.test", "oauth_client_id", "client-2"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "oauth_client_secret_version", "v2"),
					func(state *terraform.State) error {
						if !server.service.scmSecretUpdated("scm-1", "secret-2") {
							return errors.New("scm-1 secret was not rotated")
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAccSCMIntegrationResourcePAT(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSCMIntegrationPATConfig(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_scm_integration.test", "id", "scm-1"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "scm_id", "gitlab"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "auth_mode", "pat"),
				),
			},
			{
				Config: testAccSCMIntegrationPATConfig(server.URL),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccSCMIntegrationResourceOAuthRunnerPublicKeyMissing(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.setSCMCreateErr(connect.NewError(connect.CodeFailedPrecondition, errors.New("runner does not have a public key")))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccSCMIntegrationOAuthConfig(server.URL, "client-1", "secret-1", "v1"),
				ExpectError: regexp.MustCompile(`Runner Public Key Is Not Available[\s\S]*Deploy the runner first[\s\S]*rerun this Terraform configuration`),
			},
		},
	})
}

func TestAccSCMIntegrationResourceOAuthUpdateRunnerPublicKeyMissing(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSCMIntegrationOAuthConfig(server.URL, "client-1", "secret-1", "v1"),
				Check: func(state *terraform.State) error {
					server.service.setSCMUpdateErr(connect.NewError(connect.CodeFailedPrecondition, errors.New("runner does not have a public key")))
					return nil
				},
			},
			{
				Config:      testAccSCMIntegrationOAuthConfig(server.URL, "client-2", "secret-2", "v2"),
				ExpectError: regexp.MustCompile(`Runner Public Key Is Not Available[\s\S]*Deploy the runner first[\s\S]*rerun this Terraform configuration`),
			},
		},
	})
}

func TestAccSCMIntegrationResourceAzureDevOpsEntra(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSCMIntegrationAzureDevOpsEntraConfig(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_scm_integration.test", "id", "scm-1"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "scm_id", "azuredevops_entra"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "auth_mode", "oauth"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "issuer_url", "https://login.microsoftonline.com/tenant-id/v2.0"),
				),
			},
			{
				Config: testAccSCMIntegrationAzureDevOpsEntraConfig(server.URL),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccSCMIntegrationResourceAzureDevOpsEntraPAT(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSCMIntegrationAzureDevOpsEntraPATWithIssuerConfig(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_scm_integration.test", "id", "scm-1"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "scm_id", "azuredevops_entra"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "auth_mode", "pat"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "issuer_url", "https://login.microsoftonline.com/tenant-id/v2.0"),
					func(state *terraform.State) error {
						if server.service.scmCreateIssuerURLSent("scm-1") {
							return errors.New("issuer_url was sent to the API for an Azure DevOps Entra PAT integration")
						}
						return nil
					},
				),
			},
			{
				Config: testAccSCMIntegrationAzureDevOpsEntraPATWithIssuerConfig(server.URL),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccSCMIntegrationAzureDevOpsEntraPATConfig(server.URL),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_scm_integration.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ona_scm_integration.test", "issuer_url"),
					func(state *terraform.State) error {
						if server.service.scmUpdateIssuerURLSent("scm-1") {
							return errors.New("issuer_url was sent to the API for an Azure DevOps Entra PAT integration")
						}
						return nil
					},
				),
			},
			{
				Config: testAccSCMIntegrationAzureDevOpsEntraPATConfig(server.URL),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccSCMIntegrationResourceAzureDevOpsServer(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSCMIntegrationAzureDevOpsServerConfig(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_scm_integration.test", "id", "scm-1"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "scm_id", "azuredevops_server"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "auth_mode", "pat"),
					resource.TestCheckResourceAttr("ona_scm_integration.test", "virtual_directory", "/tfs"),
				),
			},
		},
	})
}

func TestAccSCMIntegrationResourceAzureValidation(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccSCMIntegrationAzureDevOpsEntraWithoutIssuerConfig(server.URL),
				ExpectError: regexp.MustCompile("Missing Azure DevOps Entra Issuer URL"),
			},
			{
				Config:      testAccSCMIntegrationAzureDevOpsServerOAuthConfig(server.URL),
				ExpectError: regexp.MustCompile("Azure DevOps Server SCM integrations currently require auth_mode=\"pat\""),
			},
			{
				Config:      testAccSCMIntegrationOAuthWithVirtualDirectoryConfig(server.URL),
				ExpectError: regexp.MustCompile("Unexpected Virtual Directory"),
			},
		},
	})
}

func TestAccEnvironmentClassResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.allEnvironmentClassesDisabled() {
				return errors.New("not all environment classes were disabled on destroy")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentClassConfig(server.URL, "Large", "100", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_environment_class.test", "id", "class-1"),
					resource.TestCheckResourceAttr("ona_environment_class.test", "runner_id", "runner-1"),
					resource.TestCheckResourceAttr("ona_environment_class.test", "display_name", "Large"),
					resource.TestCheckResourceAttr("ona_environment_class.test", "description", "High-memory class"),
					resource.TestCheckResourceAttr("ona_environment_class.test", "configuration.diskSizeGb", "100"),
					resource.TestCheckResourceAttr("ona_environment_class.test", "configuration.machineType", "m6i.2xlarge"),
					resource.TestCheckResourceAttr("ona_environment_class.test", "enabled", "true"),
				),
			},
			{
				Config: testAccEnvironmentClassConfig(server.URL, "Large", "100", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_environment_class.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccEnvironmentClassConfig(server.URL, "Large Updated", "100", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_environment_class.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_environment_class.test", "display_name", "Large Updated"),
					resource.TestCheckResourceAttr("ona_environment_class.test", "enabled", "false"),
				),
			},
			{
				Config: testAccEnvironmentClassConfig(server.URL, "Large Updated", "200", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_environment_class.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_environment_class.test", "id", "class-2"),
					resource.TestCheckResourceAttr("ona_environment_class.test", "configuration.diskSizeGb", "200"),
				),
			},
		},
	})
}

func TestAccEnvironmentClassResourceDefaultDescription(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.allEnvironmentClassesDisabled() {
				return errors.New("not all environment classes were disabled on destroy")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentClassWithoutDescriptionConfig(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_environment_class.test", "id", "class-1"),
					resource.TestCheckResourceAttr("ona_environment_class.test", "description", "Environment class managed by Terraform."),
				),
			},
			{
				Config: testAccEnvironmentClassWithoutDescriptionConfig(server.URL),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccRunnerTokenEphemeralResource(t *testing.T) {
	t.Parallel()

	server := newRunnerAPIServer(t, map[string]*v1.Runner{
		"runner-1": newTestRunner("runner-1", "Token Runner"),
	})
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerTokenEphemeralConfig(server.URL),
				Check: func(state *terraform.State) error {
					if !server.service.tokenCreated("exchange-token-runner-1") {
						return errors.New("runner token was not created")
					}
					return nil
				},
			},
		},
	})
}

func testAccSCMIntegrationOAuthConfig(host string, clientID string, secret string, secretVersion string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_scm_integration" "test" {
  runner_id                   = "runner-1"
  scm_id                      = "github"
  host                        = "github.com"
  auth_mode                   = "oauth"
  oauth_client_id             = %[2]q
  oauth_client_secret         = %[3]q
  oauth_client_secret_version = %[4]q
}
`, host, clientID, secret, secretVersion)
}

func testAccSCMIntegrationPATConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_scm_integration" "test" {
  runner_id = "runner-1"
  scm_id    = "gitlab"
  host      = "gitlab.com"
  auth_mode = "pat"
}
`, host)
}

func testAccSCMIntegrationAzureDevOpsEntraConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_scm_integration" "test" {
  runner_id                   = "runner-1"
  scm_id                      = "azuredevops_entra"
  host                        = "dev.azure.com"
  auth_mode                   = "oauth"
  oauth_client_id             = "client-1"
  oauth_client_secret         = "secret-1"
  oauth_client_secret_version = "v1"
  issuer_url                  = "https://login.microsoftonline.com/tenant-id/v2.0"
}
`, host)
}

func testAccSCMIntegrationAzureDevOpsEntraPATConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_scm_integration" "test" {
  runner_id = "runner-1"
  scm_id    = "azuredevops_entra"
  host      = "dev.azure.com"
  auth_mode = "pat"
}
`, host)
}

func testAccSCMIntegrationAzureDevOpsEntraPATWithIssuerConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_scm_integration" "test" {
  runner_id  = "runner-1"
  scm_id     = "azuredevops_entra"
  host       = "dev.azure.com"
  auth_mode  = "pat"
  issuer_url = "https://login.microsoftonline.com/tenant-id/v2.0"
}
`, host)
}

func testAccSCMIntegrationAzureDevOpsServerConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_scm_integration" "test" {
  runner_id         = "runner-1"
  scm_id            = "azuredevops_server"
  host              = "dev.azure.internal"
  auth_mode         = "pat"
  virtual_directory = "/tfs"
}
`, host)
}

func testAccSCMIntegrationAzureDevOpsEntraWithoutIssuerConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_scm_integration" "test" {
  runner_id                   = "runner-1"
  scm_id                      = "azuredevops_entra"
  host                        = "dev.azure.com"
  auth_mode                   = "oauth"
  oauth_client_id             = "client-1"
  oauth_client_secret         = "secret-1"
  oauth_client_secret_version = "v1"
}
`, host)
}

func testAccSCMIntegrationAzureDevOpsServerOAuthConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_scm_integration" "test" {
  runner_id                   = "runner-1"
  scm_id                      = "azuredevops_server"
  host                        = "dev.azure.internal"
  auth_mode                   = "oauth"
  oauth_client_id             = "client-1"
  oauth_client_secret         = "secret-1"
  oauth_client_secret_version = "v1"
  virtual_directory           = "/tfs"
}
`, host)
}

func testAccSCMIntegrationOAuthWithVirtualDirectoryConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_scm_integration" "test" {
  runner_id                   = "runner-1"
  scm_id                      = "github"
  host                        = "github.com"
  auth_mode                   = "oauth"
  oauth_client_id             = "client-1"
  oauth_client_secret         = "secret-1"
  oauth_client_secret_version = "v1"
  virtual_directory           = "/tfs"
}
`, host)
}

func testAccEnvironmentClassConfig(host string, displayName string, diskSize string, enabled bool) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_environment_class" "test" {
  runner_id    = "runner-1"
  display_name = %[2]q
  description  = "High-memory class"
  enabled      = %[4]t

  configuration = {
    machineType = "m6i.2xlarge"
    diskSizeGb  = %[3]q
  }
}
`, host, displayName, diskSize, enabled)
}

func testAccEnvironmentClassWithoutDescriptionConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_environment_class" "test" {
  runner_id    = "runner-1"
  display_name = "Default Description"

  configuration = {
    machineType = "m6i.2xlarge"
    diskSizeGb  = "100"
  }
}
`, host)
}

func testAccRunnerTokenEphemeralConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

ephemeral "ona_runner_token" "test" {
  runner_id = "runner-1"
}

provider "echo" {
  data = ephemeral.ona_runner_token.test
}

resource "echo" "test" {}
`, host)
}

type runnerConfigurationAPIServer struct {
	*httptest.Server
	service *fakeRunnerConfigurationService
}

func newRunnerConfigurationAPIServer(t *testing.T) *runnerConfigurationAPIServer {
	t.Helper()

	service := &fakeRunnerConfigurationService{
		scmIntegrations:    map[string]*v1.SCMIntegration{},
		scmCreateRequests:  map[string]*v1.CreateSCMIntegrationRequest{},
		scmUpdateRequests:  map[string][]*v1.UpdateSCMIntegrationRequest{},
		environmentClasses: map[string]*v1.EnvironmentClass{},
	}
	_, handler := v1connect.NewRunnerConfigurationServiceHandler(service)
	server := httptest.NewServer(http.StripPrefix("/api", handler))
	return &runnerConfigurationAPIServer{
		Server:  server,
		service: service,
	}
}

type fakeRunnerConfigurationService struct {
	v1connect.UnimplementedRunnerConfigurationServiceHandler

	mu                 sync.Mutex
	scmIntegrations    map[string]*v1.SCMIntegration
	scmCreateRequests  map[string]*v1.CreateSCMIntegrationRequest
	scmUpdateRequests  map[string][]*v1.UpdateSCMIntegrationRequest
	scmDeletes         []string
	scmSecretUpdates   map[string][]string
	scmCreateErr       error
	scmUpdateErr       error
	environmentClasses map[string]*v1.EnvironmentClass
}

func (s *fakeRunnerConfigurationService) CreateSCMIntegration(ctx context.Context, req *connect.Request[v1.CreateSCMIntegrationRequest]) (*connect.Response[v1.CreateSCMIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.scmCreateErr != nil {
		return nil, s.scmCreateErr
	}

	id := fmt.Sprintf("scm-%d", len(s.scmIntegrations)+1)
	integration := &v1.SCMIntegration{
		Id:               id,
		RunnerId:         req.Msg.GetRunnerId(),
		ScmId:            req.Msg.GetScmId(),
		Host:             req.Msg.GetHost(),
		Pat:              req.Msg.GetPat(),
		VirtualDirectory: optionalString(req.Msg.VirtualDirectory),
	}
	if req.Msg.OauthClientId != nil {
		integration.Oauth = &v1.SCMIntegrationOAuthConfig{
			ClientId:  req.Msg.GetOauthClientId(),
			IssuerUrl: req.Msg.GetIssuerUrl(),
		}
	}
	if req.Msg.OauthPlaintextClientSecret != nil {
		s.recordSCMSecretUpdate(id, req.Msg.GetOauthPlaintextClientSecret())
	}
	s.scmCreateRequests[id] = cloneCreateSCMIntegrationRequest(req.Msg)
	s.scmIntegrations[id] = integration
	return connect.NewResponse(&v1.CreateSCMIntegrationResponse{Id: id}), nil
}

func (s *fakeRunnerConfigurationService) GetSCMIntegration(ctx context.Context, req *connect.Request[v1.GetSCMIntegrationRequest]) (*connect.Response[v1.GetSCMIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	integration := s.scmIntegrations[req.Msg.GetId()]
	if integration == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("SCM integration not found"))
	}
	return connect.NewResponse(&v1.GetSCMIntegrationResponse{Integration: cloneSCMIntegration(integration)}), nil
}

func (s *fakeRunnerConfigurationService) UpdateSCMIntegration(ctx context.Context, req *connect.Request[v1.UpdateSCMIntegrationRequest]) (*connect.Response[v1.UpdateSCMIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.scmUpdateErr != nil {
		return nil, s.scmUpdateErr
	}

	integration := s.scmIntegrations[req.Msg.GetId()]
	if integration == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("SCM integration not found"))
	}
	s.scmUpdateRequests[req.Msg.GetId()] = append(s.scmUpdateRequests[req.Msg.GetId()], cloneUpdateSCMIntegrationRequest(req.Msg))
	if req.Msg.Pat != nil {
		integration.Pat = req.Msg.GetPat()
	}
	if req.Msg.OauthClientId != nil {
		if req.Msg.GetOauthClientId() == "" {
			integration.Oauth = nil
		} else {
			if integration.Oauth == nil {
				integration.Oauth = &v1.SCMIntegrationOAuthConfig{}
			}
			integration.Oauth.ClientId = req.Msg.GetOauthClientId()
		}
	}
	if req.Msg.IssuerUrl != nil {
		if integration.Oauth == nil {
			integration.Oauth = &v1.SCMIntegrationOAuthConfig{}
		}
		integration.Oauth.IssuerUrl = req.Msg.GetIssuerUrl()
	}
	if req.Msg.VirtualDirectory != nil {
		integration.VirtualDirectory = optionalString(req.Msg.VirtualDirectory)
	}
	if req.Msg.OauthPlaintextClientSecret != nil {
		s.recordSCMSecretUpdate(req.Msg.GetId(), req.Msg.GetOauthPlaintextClientSecret())
	}
	return connect.NewResponse(&v1.UpdateSCMIntegrationResponse{}), nil
}

func (s *fakeRunnerConfigurationService) DeleteSCMIntegration(ctx context.Context, req *connect.Request[v1.DeleteSCMIntegrationRequest]) (*connect.Response[v1.DeleteSCMIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.scmIntegrations[req.Msg.GetId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("SCM integration not found"))
	}
	delete(s.scmIntegrations, req.Msg.GetId())
	s.scmDeletes = append(s.scmDeletes, req.Msg.GetId())
	return connect.NewResponse(&v1.DeleteSCMIntegrationResponse{}), nil
}

func (s *fakeRunnerConfigurationService) CreateEnvironmentClass(ctx context.Context, req *connect.Request[v1.CreateEnvironmentClassRequest]) (*connect.Response[v1.CreateEnvironmentClassResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(req.Msg.GetDescription()) < 3 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("description must be at least 3 characters"))
	}

	id := fmt.Sprintf("class-%d", len(s.environmentClasses)+1)
	s.environmentClasses[id] = &v1.EnvironmentClass{
		Id:            id,
		RunnerId:      req.Msg.GetRunnerId(),
		DisplayName:   req.Msg.GetDisplayName(),
		Description:   req.Msg.GetDescription(),
		Configuration: cloneFieldValues(req.Msg.GetConfiguration()),
		Enabled:       true,
	}
	return connect.NewResponse(&v1.CreateEnvironmentClassResponse{Id: id}), nil
}

func (s *fakeRunnerConfigurationService) GetEnvironmentClass(ctx context.Context, req *connect.Request[v1.GetEnvironmentClassRequest]) (*connect.Response[v1.GetEnvironmentClassResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	class := s.environmentClasses[req.Msg.GetEnvironmentClassId()]
	if class == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("environment class not found"))
	}
	return connect.NewResponse(&v1.GetEnvironmentClassResponse{EnvironmentClass: cloneEnvironmentClass(class)}), nil
}

func (s *fakeRunnerConfigurationService) UpdateEnvironmentClass(ctx context.Context, req *connect.Request[v1.UpdateEnvironmentClassRequest]) (*connect.Response[v1.UpdateEnvironmentClassResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	class := s.environmentClasses[req.Msg.GetEnvironmentClassId()]
	if class == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("environment class not found"))
	}
	if req.Msg.DisplayName != nil {
		class.DisplayName = req.Msg.GetDisplayName()
	}
	if req.Msg.Description != nil {
		class.Description = req.Msg.GetDescription()
	}
	if req.Msg.Enabled != nil {
		class.Enabled = req.Msg.GetEnabled()
	}
	return connect.NewResponse(&v1.UpdateEnvironmentClassResponse{}), nil
}

func (s *fakeRunnerConfigurationService) scmDeleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, deleted := range s.scmDeletes {
		if deleted == id {
			return true
		}
	}
	return false
}

func (s *fakeRunnerConfigurationService) scmSecretUpdated(id string, secret string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, updated := range s.scmSecretUpdates[id] {
		if updated == secret {
			return true
		}
	}
	return false
}

func (s *fakeRunnerConfigurationService) scmCreateIssuerURLSent(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	request := s.scmCreateRequests[id]
	return request != nil && request.IssuerUrl != nil
}

func (s *fakeRunnerConfigurationService) scmUpdateIssuerURLSent(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, request := range s.scmUpdateRequests[id] {
		if request != nil && request.IssuerUrl != nil {
			return true
		}
	}
	return false
}

func (s *fakeRunnerConfigurationService) setSCMCreateErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scmCreateErr = err
}

func (s *fakeRunnerConfigurationService) setSCMUpdateErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scmUpdateErr = err
}

func (s *fakeRunnerConfigurationService) allEnvironmentClassesDisabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.environmentClasses) == 0 {
		return false
	}
	for _, class := range s.environmentClasses {
		if class.GetEnabled() {
			return false
		}
	}
	return true
}

func (s *fakeRunnerConfigurationService) recordSCMSecretUpdate(id string, secret string) {
	if s.scmSecretUpdates == nil {
		s.scmSecretUpdates = map[string][]string{}
	}
	s.scmSecretUpdates[id] = append(s.scmSecretUpdates[id], secret)
}

func cloneSCMIntegration(integration *v1.SCMIntegration) *v1.SCMIntegration {
	cloned, ok := proto.Clone(integration).(*v1.SCMIntegration)
	if !ok {
		return nil
	}
	return cloned
}

func cloneCreateSCMIntegrationRequest(request *v1.CreateSCMIntegrationRequest) *v1.CreateSCMIntegrationRequest {
	cloned, ok := proto.Clone(request).(*v1.CreateSCMIntegrationRequest)
	if !ok {
		return nil
	}
	return cloned
}

func cloneUpdateSCMIntegrationRequest(request *v1.UpdateSCMIntegrationRequest) *v1.UpdateSCMIntegrationRequest {
	cloned, ok := proto.Clone(request).(*v1.UpdateSCMIntegrationRequest)
	if !ok {
		return nil
	}
	return cloned
}

func cloneEnvironmentClass(class *v1.EnvironmentClass) *v1.EnvironmentClass {
	cloned, ok := proto.Clone(class).(*v1.EnvironmentClass)
	if !ok {
		return nil
	}
	return cloned
}

func cloneFieldValues(values []*v1.FieldValue) []*v1.FieldValue {
	result := make([]*v1.FieldValue, 0, len(values))
	for _, value := range values {
		result = append(result, &v1.FieldValue{
			Key:   value.GetKey(),
			Value: value.GetValue(),
		})
	}
	return result
}

func optionalString(value *string) *string {
	if value == nil {
		return nil
	}
	return testStringPtr(*value)
}

func testStringPtr(value string) *string {
	return &value
}
