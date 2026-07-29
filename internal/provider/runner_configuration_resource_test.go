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
	"sync"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
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
					resource.TestCheckResourceAttr("ona_scm_integration.test", "kind", "github"),
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
				Config:          testAccSCMIntegrationImportedConfig(server.URL),
				ResourceName:    "ona_scm_integration.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected one imported SCM integration state, got %d", len(states))
					}
					if states[0].ID != "scm-1" || states[0].Attributes["runner_id"] != "runner-1" {
						return fmt.Errorf("structured identity imported unexpected SCM integration state: %#v", states[0].Attributes)
					}
					return nil
				},
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
					resource.TestCheckResourceAttr("ona_scm_integration.test", "kind", "gitlab"),
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
					resource.TestCheckResourceAttr("ona_scm_integration.test", "kind", "azuredevops_entra"),
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
					resource.TestCheckResourceAttr("ona_scm_integration.test", "kind", "azuredevops_entra"),
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
					resource.TestCheckResourceAttr("ona_scm_integration.test", "kind", "azuredevops_server"),
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

func TestAccRunnerLLMIntegrationResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.llmDeleted("llm-1") {
				return errors.New("llm-1 was not deleted")
			}
			if server.service.llmDeleteForced("llm-1") {
				return errors.New("llm-1 was force deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerLLMIntegrationConfig(server.URL, []string{"sonnet_3_7"}, "https://api.anthropic.com/v1", "api-key-1", "v1", 4000, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "id", "llm-1"),
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "runner_id", "runner-1"),
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "models.#", "1"),
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "endpoint", "https://api.anthropic.com/v1"),
					resource.TestCheckNoResourceAttr("ona_runner_llm_integration.test", "api_key"),
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "api_key_version", "v1"),
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "max_tokens", "4000"),
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "enabled", "true"),
					resource.TestCheckNoResourceAttr("ona_runner_llm_integration.test", "phase"),
					resource.TestCheckNoResourceAttr("ona_runner_llm_integration.test", "phase_reason"),
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "llm_provider", "anthropic"),
					func(state *terraform.State) error {
						if !server.service.llmAPIKeyUpdated("llm-1", "api-key-1") {
							return errors.New("llm-1 API key was not set")
						}
						return nil
					},
				),
			},
			{
				Config: testAccRunnerLLMIntegrationConfig(server.URL, []string{"sonnet_3_7"}, "https://api.anthropic.com/v1", "api-key-1", "v1", 4000, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:            "ona_runner_llm_integration.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"api_key", "api_key_version"},
			},
			{
				Config: testAccRunnerLLMIntegrationConfig(server.URL, []string{"sonnet_4", "sonnet_4_extended"}, "https://api.anthropic.com/v2", "api-key-2", "v2", 8000, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_runner_llm_integration.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "models.#", "2"),
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "endpoint", "https://api.anthropic.com/v2"),
					resource.TestCheckNoResourceAttr("ona_runner_llm_integration.test", "api_key"),
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "api_key_version", "v2"),
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "max_tokens", "8000"),
					resource.TestCheckResourceAttr("ona_runner_llm_integration.test", "enabled", "false"),
					resource.TestCheckNoResourceAttr("ona_runner_llm_integration.test", "phase"),
					resource.TestCheckNoResourceAttr("ona_runner_llm_integration.test", "phase_reason"),
					func(state *terraform.State) error {
						if !server.service.llmAPIKeyUpdated("llm-1", "api-key-2") {
							return errors.New("llm-1 API key was not rotated")
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAccRunnerLLMIntegrationResourceValidation(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccRunnerLLMIntegrationWithoutAPIKeyConfig(server.URL),
				ExpectError: regexp.MustCompile("Missing LLM API Key"),
			},
			{
				Config:      testAccRunnerLLMIntegrationInvalidModelConfig(server.URL),
				ExpectError: regexp.MustCompile("Invalid LLM Model"),
			},
			{
				Config:      testAccRunnerLLMIntegrationWhitespaceEndpointConfig(server.URL),
				ExpectError: regexp.MustCompile("Invalid LLM Endpoint"),
			},
		},
	})
}

func TestAccRunnerLLMIntegrationResourcePublicKeyMissing(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.setLLMCreateErr(connect.NewError(connect.CodeFailedPrecondition, errors.New("runner does not have a public key")))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccRunnerLLMIntegrationConfig(server.URL, []string{"sonnet_3_7"}, "https://api.anthropic.com/v1", "api-key-1", "v1", 0, true),
				ExpectError: regexp.MustCompile(`Runner Public Key Is Not Available[\s\S]*Deploy the runner first[\s\S]*rerun this Terraform configuration`),
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
				ResourceName:    "ona_environment_class.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected one imported environment class state, got %d", len(states))
					}
					if states[0].ID != "class-1" || states[0].Attributes["runner_id"] != "runner-1" {
						return fmt.Errorf("structured identity imported unexpected environment class state: %#v", states[0].Attributes)
					}
					return nil
				},
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
  kind                        = "github"
  host                        = "github.com"
  auth_mode                   = "oauth"
  oauth_client_id             = %[2]q
  oauth_client_secret         = %[3]q
  oauth_client_secret_version = %[4]q
}
`, host, clientID, secret, secretVersion)
}

func testAccSCMIntegrationImportedConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_scm_integration" "test" {
  runner_id       = "runner-1"
  kind            = "github"
  host            = "github.com"
  auth_mode       = "oauth"
  oauth_client_id = "client-1"
}
`, host)
}

func testAccSCMIntegrationPATConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_scm_integration" "test" {
  runner_id = "runner-1"
  kind      = "gitlab"
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
  kind                        = "azuredevops_entra"
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
  kind      = "azuredevops_entra"
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
  kind       = "azuredevops_entra"
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
  kind              = "azuredevops_server"
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
  kind                        = "azuredevops_entra"
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
  kind                        = "azuredevops_server"
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
  kind                        = "github"
  host                        = "github.com"
  auth_mode                   = "oauth"
  oauth_client_id             = "client-1"
  oauth_client_secret         = "secret-1"
  oauth_client_secret_version = "v1"
  virtual_directory           = "/tfs"
}
`, host)
}

func testAccRunnerLLMIntegrationConfig(host string, models []string, endpoint string, apiKey string, apiKeyVersion string, maxTokens int, enabled bool) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner_llm_integration" "test" {
  runner_id       = "runner-1"
  models          = %[2]s
  endpoint        = %[3]q
  api_key         = %[4]q
  api_key_version = %[5]q
  max_tokens      = %[6]d
  enabled         = %[7]t
}
`, host, hclStringList(models), endpoint, apiKey, apiKeyVersion, maxTokens, enabled)
}

func testAccRunnerLLMIntegrationWithoutAPIKeyConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner_llm_integration" "test" {
  runner_id = "runner-1"
  models    = ["sonnet_3_7"]
  endpoint  = "https://api.anthropic.com/v1"
}
`, host)
}

func testAccRunnerLLMIntegrationInvalidModelConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner_llm_integration" "test" {
  runner_id = "runner-1"
  models    = ["not_a_model"]
  endpoint  = "https://api.anthropic.com/v1"
  api_key   = "api-key"
}
`, host)
}

func testAccRunnerLLMIntegrationWhitespaceEndpointConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner_llm_integration" "test" {
  runner_id = "runner-1"
  models    = ["sonnet_3_7"]
  endpoint  = " https://api.anthropic.com/v1 "
  api_key   = "api-key"
}
`, host)
}

func hclStringList(values []string) string {
	result := "["
	for i, value := range values {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("%q", value)
	}
	return result + "]"
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
	service       *fakeRunnerConfigurationService
	runnerService *fakeRunnerService
}

func newRunnerConfigurationAPIServer(t *testing.T) *runnerConfigurationAPIServer {
	t.Helper()

	service := &fakeRunnerConfigurationService{
		hostAuthenticationTokens:           map[string]*v1.HostAuthenticationToken{},
		hostAuthenticationCreateRequests:   map[string]*v1.CreateHostAuthenticationTokenRequest{},
		hostAuthenticationUpdateRequests:   map[string][]*v1.UpdateHostAuthenticationTokenRequest{},
		hostAuthenticationGetCounts:        map[string]int{},
		hostAuthenticationGetMisses:        map[string]int{},
		hostAuthenticationPostUpdateMisses: map[string]int{},
		scmIntegrations:                    map[string]*v1.SCMIntegration{},
		scmCreateRequests:                  map[string]*v1.CreateSCMIntegrationRequest{},
		scmUpdateRequests:                  map[string][]*v1.UpdateSCMIntegrationRequest{},
		llmIntegrations:                    map[string]*v1.LLMIntegration{},
		llmCreateRequests:                  map[string]*v1.CreateLLMIntegrationRequest{},
		llmUpdateRequests:                  map[string][]*v1.UpdateLLMIntegrationRequest{},
		environmentClasses:                 map[string]*v1.EnvironmentClass{},
		environmentClassRunnerKinds:        map[string]v1.RunnerKind{},
		environmentClassRunnerProviders:    map[string]v1.RunnerProvider{},
	}
	runnerService := &fakeRunnerService{runners: map[string]*v1.Runner{}}

	runnerConfigurationPath, runnerConfigurationHandler := v1connect.NewRunnerConfigurationServiceHandler(service)
	runnerPath, runnerHandler := v1connect.NewRunnerServiceHandler(runnerService)
	mux := http.NewServeMux()
	mux.Handle("/api"+runnerConfigurationPath, http.StripPrefix("/api", runnerConfigurationHandler))
	mux.Handle("/api"+runnerPath, http.StripPrefix("/api", runnerHandler))
	server := httptest.NewServer(mux)
	return &runnerConfigurationAPIServer{
		Server:        server,
		service:       service,
		runnerService: runnerService,
	}
}

type fakeRunnerConfigurationService struct {
	v1connect.UnimplementedRunnerConfigurationServiceHandler

	mu                                 sync.Mutex
	hostAuthenticationTokens           map[string]*v1.HostAuthenticationToken
	hostAuthenticationCreateRequests   map[string]*v1.CreateHostAuthenticationTokenRequest
	hostAuthenticationUpdateRequests   map[string][]*v1.UpdateHostAuthenticationTokenRequest
	hostAuthenticationGetCounts        map[string]int
	hostAuthenticationGetMisses        map[string]int
	hostAuthenticationPostUpdateMisses map[string]int
	hostAuthenticationDeletes          []string
	hostAuthenticationPATUpdates       map[string][]string
	scmIntegrations                    map[string]*v1.SCMIntegration
	scmCreateRequests                  map[string]*v1.CreateSCMIntegrationRequest
	scmUpdateRequests                  map[string][]*v1.UpdateSCMIntegrationRequest
	scmDeletes                         []string
	scmSecretUpdates                   map[string][]string
	scmCreateErr                       error
	scmUpdateErr                       error
	llmIntegrations                    map[string]*v1.LLMIntegration
	llmCreateRequests                  map[string]*v1.CreateLLMIntegrationRequest
	llmUpdateRequests                  map[string][]*v1.UpdateLLMIntegrationRequest
	llmDeletes                         map[string]bool
	llmDeleteForce                     map[string]bool
	llmAPIKeyUpdates                   map[string][]string
	llmCreateErr                       error
	environmentClasses                 map[string]*v1.EnvironmentClass
	lastEnvironmentClassListKinds      []v1.RunnerKind
	environmentClassRunnerKinds        map[string]v1.RunnerKind
	environmentClassRunnerProviders    map[string]v1.RunnerProvider
}

func (s *fakeRunnerConfigurationService) CreateHostAuthenticationToken(ctx context.Context, req *connect.Request[v1.CreateHostAuthenticationTokenRequest]) (*connect.Response[v1.CreateHostAuthenticationTokenResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.hostAuthenticationTokens {
		if existing.GetRunnerId() == req.Msg.GetRunnerId() && existing.GetHost() == req.Msg.GetHost() && existing.GetSubject().GetId() == req.Msg.GetSubject().GetId() {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("host authentication token already exists"))
		}
	}
	id := fmt.Sprintf("git-auth-%d", len(s.hostAuthenticationTokens)+1)
	token := &v1.HostAuthenticationToken{
		Id:            id,
		RunnerId:      req.Msg.GetRunnerId(),
		Host:          req.Msg.GetHost(),
		Source:        req.Msg.GetSource(),
		ExpiresAt:     req.Msg.GetExpiresAt(),
		IntegrationId: req.Msg.GetIntegrationId(),
		Scopes:        append([]string(nil), req.Msg.GetScopes()...),
		Subject:       cloneRunnerConfigurationSubject(req.Msg.GetSubject()),
	}
	s.hostAuthenticationTokens[id] = token
	s.hostAuthenticationCreateRequests[id] = cloneCreateHostAuthenticationTokenRequest(req.Msg)
	s.recordHostAuthenticationPATUpdate(id, req.Msg.GetToken())
	return connect.NewResponse(&v1.CreateHostAuthenticationTokenResponse{Token: cloneHostAuthenticationToken(token)}), nil
}

func (s *fakeRunnerConfigurationService) GetHostAuthenticationToken(ctx context.Context, req *connect.Request[v1.GetHostAuthenticationTokenRequest]) (*connect.Response[v1.GetHostAuthenticationTokenResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hostAuthenticationGetCounts[req.Msg.GetId()]++
	if s.hostAuthenticationGetMisses[req.Msg.GetId()] > 0 {
		s.hostAuthenticationGetMisses[req.Msg.GetId()]--
		return nil, connect.NewError(connect.CodeNotFound, errors.New("host authentication token is not visible on the read replica"))
	}
	token := s.hostAuthenticationTokens[req.Msg.GetId()]
	if token == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("host authentication token not found"))
	}
	return connect.NewResponse(&v1.GetHostAuthenticationTokenResponse{Token: cloneHostAuthenticationToken(token)}), nil
}

func (s *fakeRunnerConfigurationService) ListHostAuthenticationTokens(ctx context.Context, req *connect.Request[v1.ListHostAuthenticationTokensRequest]) (*connect.Response[v1.ListHostAuthenticationTokensResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var tokens []*v1.HostAuthenticationToken
	for _, token := range s.hostAuthenticationTokens {
		if runnerID := req.Msg.GetFilter().GetRunnerId(); runnerID != "" && token.GetRunnerId() != runnerID {
			continue
		}
		if subjectID := req.Msg.GetFilter().GetSubjectId(); subjectID != "" && token.GetSubject().GetId() != subjectID {
			continue
		}
		tokens = append(tokens, cloneHostAuthenticationToken(token))
	}
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].GetId() < tokens[j].GetId() })
	return connect.NewResponse(&v1.ListHostAuthenticationTokensResponse{Tokens: tokens, Pagination: &v1.PaginationResponse{}}), nil
}

func (s *fakeRunnerConfigurationService) UpdateHostAuthenticationToken(ctx context.Context, req *connect.Request[v1.UpdateHostAuthenticationTokenRequest]) (*connect.Response[v1.UpdateHostAuthenticationTokenResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hostAuthenticationTokens[req.Msg.GetId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("host authentication token not found"))
	}
	s.hostAuthenticationUpdateRequests[req.Msg.GetId()] = append(s.hostAuthenticationUpdateRequests[req.Msg.GetId()], cloneUpdateHostAuthenticationTokenRequest(req.Msg))
	if req.Msg.Token != nil {
		s.recordHostAuthenticationPATUpdate(req.Msg.GetId(), req.Msg.GetToken())
	}
	if misses := s.hostAuthenticationPostUpdateMisses[req.Msg.GetId()]; misses > 0 {
		s.hostAuthenticationGetMisses[req.Msg.GetId()] = misses
		delete(s.hostAuthenticationPostUpdateMisses, req.Msg.GetId())
	}
	return connect.NewResponse(&v1.UpdateHostAuthenticationTokenResponse{}), nil
}

func (s *fakeRunnerConfigurationService) DeleteHostAuthenticationToken(ctx context.Context, req *connect.Request[v1.DeleteHostAuthenticationTokenRequest]) (*connect.Response[v1.DeleteHostAuthenticationTokenResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hostAuthenticationTokens[req.Msg.GetId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("host authentication token not found"))
	}
	delete(s.hostAuthenticationTokens, req.Msg.GetId())
	s.hostAuthenticationDeletes = append(s.hostAuthenticationDeletes, req.Msg.GetId())
	return connect.NewResponse(&v1.DeleteHostAuthenticationTokenResponse{}), nil
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

func (s *fakeRunnerConfigurationService) ListSCMIntegrations(ctx context.Context, req *connect.Request[v1.ListSCMIntegrationsRequest]) (*connect.Response[v1.ListSCMIntegrationsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runnerIDs := map[string]struct{}{}
	for _, id := range req.Msg.GetFilter().GetRunnerIds() {
		runnerIDs[id] = struct{}{}
	}

	var integrations []*v1.SCMIntegration
	for _, integration := range s.scmIntegrations {
		if len(runnerIDs) > 0 {
			if _, ok := runnerIDs[integration.GetRunnerId()]; !ok {
				continue
			}
		}
		integrations = append(integrations, cloneSCMIntegration(integration))
	}
	sort.Slice(integrations, func(i, j int) bool {
		return integrations[i].GetId() < integrations[j].GetId()
	})
	return connect.NewResponse(&v1.ListSCMIntegrationsResponse{Integrations: integrations}), nil
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

func (s *fakeRunnerConfigurationService) CreateLLMIntegration(ctx context.Context, req *connect.Request[v1.CreateLLMIntegrationRequest]) (*connect.Response[v1.CreateLLMIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.llmCreateErr != nil {
		return nil, s.llmCreateErr
	}

	id := fmt.Sprintf("llm-%d", len(s.llmIntegrations)+1)
	integration := &v1.LLMIntegration{
		Id:        id,
		RunnerId:  req.Msg.GetRunnerId(),
		Models:    append([]v1.SupportedModel{}, req.Msg.GetModels()...),
		Endpoint:  req.Msg.GetEndpoint(),
		MaxTokens: req.Msg.GetMaxTokens(),
		Phase:     v1.LLMIntegrationPhase_LLM_INTEGRATION_PHASE_AVAILABLE,
		Provider:  llmProviderForTestModels(req.Msg.GetModels()),
	}
	if req.Msg.GetApiKey() != "" {
		s.recordLLMAPIKeyUpdate(id, req.Msg.GetApiKey())
	}
	s.llmCreateRequests[id] = cloneCreateLLMIntegrationRequest(req.Msg)
	s.llmIntegrations[id] = integration
	return connect.NewResponse(&v1.CreateLLMIntegrationResponse{Id: id}), nil
}

func (s *fakeRunnerConfigurationService) GetLLMIntegration(ctx context.Context, req *connect.Request[v1.GetLLMIntegrationRequest]) (*connect.Response[v1.GetLLMIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	integration := s.llmIntegrations[req.Msg.GetId()]
	if integration == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("LLM integration not found"))
	}
	return connect.NewResponse(&v1.GetLLMIntegrationResponse{Integration: cloneLLMIntegration(integration)}), nil
}

func (s *fakeRunnerConfigurationService) UpdateLLMIntegration(ctx context.Context, req *connect.Request[v1.UpdateLLMIntegrationRequest]) (*connect.Response[v1.UpdateLLMIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	integration := s.llmIntegrations[req.Msg.GetId()]
	if integration == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("LLM integration not found"))
	}
	s.llmUpdateRequests[req.Msg.GetId()] = append(s.llmUpdateRequests[req.Msg.GetId()], cloneUpdateLLMIntegrationRequest(req.Msg))
	if req.Msg.Endpoint != nil {
		integration.Endpoint = req.Msg.GetEndpoint()
	}
	if len(req.Msg.GetModels()) > 0 {
		integration.Models = append([]v1.SupportedModel{}, req.Msg.GetModels()...)
		integration.Provider = llmProviderForTestModels(req.Msg.GetModels())
	}
	if req.Msg.ApiKey != nil {
		s.recordLLMAPIKeyUpdate(req.Msg.GetId(), req.Msg.GetApiKey())
	}
	if req.Msg.MaxTokens != nil {
		integration.MaxTokens = req.Msg.GetMaxTokens()
	}
	if req.Msg.Phase != nil {
		integration.Phase = req.Msg.GetPhase()
	}
	return connect.NewResponse(&v1.UpdateLLMIntegrationResponse{}), nil
}

func (s *fakeRunnerConfigurationService) DeleteLLMIntegration(ctx context.Context, req *connect.Request[v1.DeleteLLMIntegrationRequest]) (*connect.Response[v1.DeleteLLMIntegrationResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.llmIntegrations[req.Msg.GetId()] == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("LLM integration not found"))
	}
	delete(s.llmIntegrations, req.Msg.GetId())
	if s.llmDeletes == nil {
		s.llmDeletes = map[string]bool{}
	}
	if s.llmDeleteForce == nil {
		s.llmDeleteForce = map[string]bool{}
	}
	s.llmDeletes[req.Msg.GetId()] = true
	s.llmDeleteForce[req.Msg.GetId()] = req.Msg.GetForce()
	return connect.NewResponse(&v1.DeleteLLMIntegrationResponse{}), nil
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

func (s *fakeRunnerConfigurationService) ListEnvironmentClasses(ctx context.Context, req *connect.Request[v1.ListEnvironmentClassesRequest]) (*connect.Response[v1.ListEnvironmentClassesResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastEnvironmentClassListKinds = append([]v1.RunnerKind(nil), req.Msg.GetFilter().GetRunnerKinds()...)

	runnerIDs := map[string]struct{}{}
	for _, id := range req.Msg.GetFilter().GetRunnerIds() {
		runnerIDs[id] = struct{}{}
	}
	runnerProviders := map[v1.RunnerProvider]struct{}{}
	for _, provider := range req.Msg.GetFilter().GetRunnerProviders() {
		runnerProviders[provider] = struct{}{}
	}
	runnerKinds := map[v1.RunnerKind]struct{}{}
	for _, kind := range req.Msg.GetFilter().GetRunnerKinds() {
		runnerKinds[kind] = struct{}{}
	}

	var classes []*v1.EnvironmentClass
	for _, class := range s.environmentClasses {
		if len(runnerIDs) > 0 {
			if _, ok := runnerIDs[class.GetRunnerId()]; !ok {
				continue
			}
		}
		if provider, ok := s.environmentClassRunnerProviders[class.GetRunnerId()]; ok && len(runnerProviders) > 0 {
			if _, ok := runnerProviders[provider]; !ok {
				continue
			}
		}
		if kind, ok := s.environmentClassRunnerKinds[class.GetRunnerId()]; ok && len(runnerKinds) > 0 {
			if _, ok := runnerKinds[kind]; !ok {
				continue
			}
		}
		if enabled := req.Msg.GetFilter().Enabled; enabled != nil && class.GetEnabled() != *enabled {
			continue
		}
		classes = append(classes, cloneEnvironmentClass(class))
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i].GetId() < classes[j].GetId() })
	return connect.NewResponse(&v1.ListEnvironmentClassesResponse{EnvironmentClasses: classes}), nil
}

func (s *fakeRunnerConfigurationService) environmentClassListRunnerKinds() []v1.RunnerKind {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]v1.RunnerKind(nil), s.lastEnvironmentClassListKinds...)
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

func (s *fakeRunnerConfigurationService) hostAuthenticationDeleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, deleted := range s.hostAuthenticationDeletes {
		if deleted == id {
			return true
		}
	}
	return false
}

func (s *fakeRunnerConfigurationService) hostAuthenticationPATUpdated(id string, pat string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, updated := range s.hostAuthenticationPATUpdates[id] {
		if updated == pat {
			return true
		}
	}
	return false
}

func (s *fakeRunnerConfigurationService) hostAuthenticationCreateRequest(id string) *v1.CreateHostAuthenticationTokenRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return cloneCreateHostAuthenticationTokenRequest(s.hostAuthenticationCreateRequests[id])
}

func (s *fakeRunnerConfigurationService) hostAuthenticationCreateCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.hostAuthenticationCreateRequests)
}

func (s *fakeRunnerConfigurationService) hostAuthenticationGetCount(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.hostAuthenticationGetCounts[id]
}

func (s *fakeRunnerConfigurationService) setHostAuthenticationPostUpdateMisses(id string, misses int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hostAuthenticationPostUpdateMisses[id] = misses
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

func (s *fakeRunnerConfigurationService) llmDeleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.llmDeletes[id]
}

func (s *fakeRunnerConfigurationService) llmDeleteForced(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.llmDeleteForce[id]
}

func (s *fakeRunnerConfigurationService) llmAPIKeyUpdated(id string, apiKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, updated := range s.llmAPIKeyUpdates[id] {
		if updated == apiKey {
			return true
		}
	}
	return false
}

func (s *fakeRunnerConfigurationService) scmSecretUpdateCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	for _, updates := range s.scmSecretUpdates {
		count += len(updates)
	}
	return count
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

func (s *fakeRunnerConfigurationService) setLLMCreateErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.llmCreateErr = err
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

func (s *fakeRunnerConfigurationService) recordHostAuthenticationPATUpdate(id string, pat string) {
	if s.hostAuthenticationPATUpdates == nil {
		s.hostAuthenticationPATUpdates = map[string][]string{}
	}
	s.hostAuthenticationPATUpdates[id] = append(s.hostAuthenticationPATUpdates[id], pat)
}

func (s *fakeRunnerConfigurationService) recordLLMAPIKeyUpdate(id string, apiKey string) {
	if s.llmAPIKeyUpdates == nil {
		s.llmAPIKeyUpdates = map[string][]string{}
	}
	s.llmAPIKeyUpdates[id] = append(s.llmAPIKeyUpdates[id], apiKey)
}

func cloneSCMIntegration(integration *v1.SCMIntegration) *v1.SCMIntegration {
	cloned, ok := proto.Clone(integration).(*v1.SCMIntegration)
	if !ok {
		return nil
	}
	return cloned
}

func cloneHostAuthenticationToken(token *v1.HostAuthenticationToken) *v1.HostAuthenticationToken {
	if token == nil {
		return nil
	}
	cloned, ok := proto.Clone(token).(*v1.HostAuthenticationToken)
	if !ok {
		return nil
	}
	return cloned
}

func cloneRunnerConfigurationSubject(subject *v1.Subject) *v1.Subject {
	if subject == nil {
		return nil
	}
	cloned, ok := proto.Clone(subject).(*v1.Subject)
	if !ok {
		return nil
	}
	return cloned
}

func cloneCreateHostAuthenticationTokenRequest(request *v1.CreateHostAuthenticationTokenRequest) *v1.CreateHostAuthenticationTokenRequest {
	if request == nil {
		return nil
	}
	cloned, ok := proto.Clone(request).(*v1.CreateHostAuthenticationTokenRequest)
	if !ok {
		return nil
	}
	return cloned
}

func cloneUpdateHostAuthenticationTokenRequest(request *v1.UpdateHostAuthenticationTokenRequest) *v1.UpdateHostAuthenticationTokenRequest {
	if request == nil {
		return nil
	}
	cloned, ok := proto.Clone(request).(*v1.UpdateHostAuthenticationTokenRequest)
	if !ok {
		return nil
	}
	return cloned
}

func cloneLLMIntegration(integration *v1.LLMIntegration) *v1.LLMIntegration {
	cloned, ok := proto.Clone(integration).(*v1.LLMIntegration)
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

func cloneCreateLLMIntegrationRequest(request *v1.CreateLLMIntegrationRequest) *v1.CreateLLMIntegrationRequest {
	cloned, ok := proto.Clone(request).(*v1.CreateLLMIntegrationRequest)
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

func cloneUpdateLLMIntegrationRequest(request *v1.UpdateLLMIntegrationRequest) *v1.UpdateLLMIntegrationRequest {
	cloned, ok := proto.Clone(request).(*v1.UpdateLLMIntegrationRequest)
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

func llmProviderForTestModels(models []v1.SupportedModel) v1.LLMProvider {
	for _, model := range models {
		switch model {
		case v1.SupportedModel_SUPPORTED_MODEL_OPENAI_4O,
			v1.SupportedModel_SUPPORTED_MODEL_OPENAI_4O_MINI,
			v1.SupportedModel_SUPPORTED_MODEL_OPENAI_O1,
			v1.SupportedModel_SUPPORTED_MODEL_OPENAI_O1_MINI,
			v1.SupportedModel_SUPPORTED_MODEL_OPENAI_AUTO:
			return v1.LLMProvider_LLM_PROVIDER_OPENAI
		}
	}
	if len(models) > 0 {
		return v1.LLMProvider_LLM_PROVIDER_ANTHROPIC
	}
	return v1.LLMProvider_LLM_PROVIDER_UNSPECIFIED
}
