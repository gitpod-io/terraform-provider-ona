// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccGitAuthenticationResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}
	server.service.scmIntegrations["scm-2"] = &v1.SCMIntegration{Id: "scm-2", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.hostAuthenticationDeleted("git-auth-1") {
				return errors.New("git-auth-1 was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-1", "v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_git_authentication.test", "id", "git-auth-1"),
					resource.TestCheckResourceAttr("ona_git_authentication.test", "service_account_id", "service-account-1"),
					resource.TestCheckResourceAttr("ona_git_authentication.test", "scm_integration_id", "scm-1"),
					resource.TestCheckResourceAttr("ona_git_authentication.test", "runner_id", "runner-1"),
					resource.TestCheckResourceAttr("ona_git_authentication.test", "host", "github.com"),
					resource.TestCheckNoResourceAttr("ona_git_authentication.test", "personal_access_token"),
					resource.TestCheckResourceAttr("ona_git_authentication.test", "personal_access_token_version", "v1"),
					func(state *terraform.State) error {
						request := server.service.hostAuthenticationCreateRequest("git-auth-1")
						if request == nil || request.GetIntegrationId() != "scm-1" || request.GetRunnerId() != "runner-1" || request.GetHost() != "github.com" {
							return errors.New("create request did not use the selected SCM integration")
						}
						if request.GetSource() != v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT || request.GetSubject().GetId() != "service-account-1" || request.GetSubject().GetPrincipal() != v1.Principal_PRINCIPAL_SERVICE_ACCOUNT {
							return errors.New("create request did not use a PAT-backed service-account subject")
						}
						if !server.service.hostAuthenticationPATUpdated("git-auth-1", "pat-1") {
							return errors.New("create request did not submit the PAT")
						}
						return nil
					},
				),
			},
			{
				Config: testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-1", "v1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
			},
			{
				ResourceName:            "ona_git_authentication.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"personal_access_token", "personal_access_token_version"},
			},
			{
				Config:          testAccGitAuthenticationImportedConfig(server.URL),
				ResourceName:    "ona_git_authentication.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 || states[0].ID != "git-auth-1" || states[0].Attributes["service_account_id"] != "service-account-1" {
						return fmt.Errorf("structured identity imported unexpected Git authentication state: %#v", states)
					}
					return nil
				},
			},
			{
				Config: testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-2", "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_git_authentication.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_git_authentication.test", "personal_access_token_version", "v2"),
					func(state *terraform.State) error {
						if !server.service.hostAuthenticationPATUpdated("git-auth-1", "pat-2") {
							return errors.New("PAT was not rotated")
						}
						return nil
					},
				),
			},
			{
				Config: testAccGitAuthenticationConfig(server.URL, "scm-2", "pat-3", "v3"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_git_authentication.test", plancheck.ResourceActionReplace),
				}},
			},
		},
	})
}

func TestAccGitAuthenticationRejectsSCMIntegrationWithoutPAT(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com"}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-1", "v1"),
			ExpectError: regexp.MustCompile("SCM Integration Does Not Support PAT Authentication"),
		}},
	})
	if got := server.service.hostAuthenticationCreateCount(); got != 0 {
		t.Fatalf("created %d Git authentications for a non-PAT SCM integration", got)
	}
}

func testAccGitAuthenticationConfig(host string, integrationID string, pat string, version string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_git_authentication" "test" {
  service_account_id = "service-account-1"
  scm_integration_id  = %[2]q

  personal_access_token         = %[3]q
  personal_access_token_version = %[4]q
}
`, host, integrationID, pat, version)
}

func testAccGitAuthenticationImportedConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_git_authentication" "test" {
  service_account_id = "service-account-1"
  scm_integration_id  = "scm-1"
}
`, host)
}
