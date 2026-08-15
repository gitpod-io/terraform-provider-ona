// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccGitAuthenticationResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}
	server.service.scmIntegrations["scm-2"] = &v1.SCMIntegration{Id: "scm-2", RunnerId: "runner-1", ScmId: "gitlab", Host: "gitlab.com", Pat: true}

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
						if request == nil || request.GetIntegrationId() != "" || request.GetRunnerId() != "runner-1" || request.GetHost() != "github.com" {
							return errors.New("create request did not use the selected SCM integration target")
						}
						if request.GetSource() != v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT || request.GetSubject().GetId() != "service-account-1" || request.GetSubject().GetPrincipal() != v1.Principal_PRINCIPAL_SERVICE_ACCOUNT {
							return errors.New("create request did not use a PAT-backed service-account subject")
						}
						if !server.service.hostAuthenticationPATUpdated("git-auth-1", "pat-1") {
							return errors.New("create request did not submit the PAT")
						}
						if !server.serviceAccountService.accessTokenRequested("service-account-1") {
							return errors.New("create did not authenticate as the target service account")
						}
						if got := server.service.hostAuthenticationCreateAuthorization("git-auth-1"); got != "Bearer access-token-service-account-1" {
							return fmt.Errorf("create authorization = %q, want target service account access token", got)
						}
						if got := server.service.hostAuthenticationGetCount("git-auth-1"); got != 0 {
							return fmt.Errorf("Create performed %d host authentication readbacks, want 0", got)
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
				PreConfig: func() {
					server.service.setHostAuthenticationPostUpdateMisses("git-auth-1", 1)
				},
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

func TestAccGitAuthenticationCreateRequiresPAT(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccGitAuthenticationImportedConfig(server.URL),
			ExpectError: regexp.MustCompile(`Missing Personal Access Token`),
		}},
	})

	type Expectation struct {
		CreateCalls          int
		AccessTokenRequested bool
	}
	want := Expectation{}
	got := Expectation{
		CreateCalls:          server.service.hostAuthenticationCreateCount(),
		AccessTokenRequested: server.serviceAccountService.accessTokenRequested("service-account-1"),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Git authentication creation without PAT mismatch (-want +got):\n%s", diff)
	}
}

func TestAccGitAuthenticationLifecycleUsesTargetServiceAccount(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}
	server.service.requireHostAuthenticationAuthorization("Bearer access-token-service-account-1")

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
				Check:  resource.TestCheckResourceAttr("ona_git_authentication.test", "id", "git-auth-1"),
			},
			{
				Config: testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-1", "v1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
			},
			{
				Config: testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-2", "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_git_authentication.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckResourceAttr("ona_git_authentication.test", "personal_access_token_version", "v2"),
			},
		},
	})
}

func TestAccGitAuthenticationRotationRetainsStateAfterReplicaMiss(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}

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
				Check:  resource.TestCheckResourceAttr("ona_git_authentication.test", "personal_access_token_version", "v1"),
			},
			{
				PreConfig: func() {
					server.service.setHostAuthenticationPostUpdateMisses("git-auth-1", 4)
				},
				Config:      testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-2", "v2"),
				ExpectError: regexp.MustCompile(`Terraform retained the\s+prior state`),
			},
			{
				Config: testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-2", "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_git_authentication.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_git_authentication.test", "id", "git-auth-1"),
					resource.TestCheckResourceAttr("ona_git_authentication.test", "personal_access_token_version", "v2"),
					func(state *terraform.State) error {
						if got := server.service.hostAuthenticationCreateCount(); got != 1 {
							return fmt.Errorf("created %d Git authentications after rotation readback failure, want 1", got)
						}
						return nil
					},
				),
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

func TestAccGitAuthenticationCreateFailures(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		AccessTokenRequested bool
		CreateCalls          int
	}
	tests := []struct {
		Name          string
		Setup         func(*runnerConfigurationAPIServer)
		ExpectedError string
		Expected      Expectation
	}{
		{
			Name: "service_account_access_token_error",
			Setup: func(server *runnerConfigurationAPIServer) {
				server.serviceAccountService.setNextAccessTokenError(errors.New("mint access token denied"))
			},
			ExpectedError: `Unable to Authenticate as Ona Service Account[\s\S]*mint access token[\s\S]*denied`,
			Expected:      Expectation{AccessTokenRequested: true},
		},
		{
			Name: "create_api_error",
			Setup: func(server *runnerConfigurationAPIServer) {
				server.service.setNextHostAuthenticationCreateError(errors.New("create Git authentication denied"))
			},
			ExpectedError: `Unable to Create Ona Git Authentication[\s\S]*create Git authentication[\s\S]*denied`,
			Expected:      Expectation{AccessTokenRequested: true, CreateCalls: 1},
		},
		{
			Name: "empty_service_account_access_token",
			Setup: func(server *runnerConfigurationAPIServer) {
				server.serviceAccountService.returnEmptyNextAccessToken()
			},
			ExpectedError: `Unable to Authenticate as Ona Service Account[\s\S]*returned an empty[\s\S]*token`,
			Expected:      Expectation{AccessTokenRequested: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newRunnerConfigurationAPIServer(t)
			t.Cleanup(server.Close)
			server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}
			tc.Setup(server)

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-1", "v1"),
					ExpectError: regexp.MustCompile(tc.ExpectedError),
				}},
			})

			got := Expectation{
				AccessTokenRequested: server.serviceAccountService.accessTokenRequested("service-account-1"),
				CreateCalls:          server.service.hostAuthenticationCreateCount(),
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("Git authentication Create failure mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAccGitAuthenticationReadNotFoundPlansCreate(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}

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
			{Config: testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-1", "v1")},
			{
				Config:      testAccGitAuthenticationConfig(server.URL, "scm-1", "", "v2"),
				ExpectError: regexp.MustCompile(`Missing Personal Access Token`),
			},
			{
				PreConfig: func() { server.service.removeHostAuthentication("git-auth-1") },
				Config:    testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-1", "v1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_git_authentication.test", plancheck.ResourceActionCreate),
				}},
				Check: func(*terraform.State) error {
					if got := server.service.hostAuthenticationCreateCount(); got != 2 {
						return fmt.Errorf("CreateHostAuthenticationToken call count = %d, want 2", got)
					}
					return nil
				},
			},
		},
	})
}

func TestAccGitAuthenticationUpdateAPIFailure(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}

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
			{Config: testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-1", "v1")},
			{
				PreConfig: func() {
					server.service.setNextHostAuthenticationUpdateError(errors.New("update Git authentication denied"))
				},
				Config:      testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-2", "v2"),
				ExpectError: regexp.MustCompile(`Unable to Update Ona Git Authentication[\s\S]*update Git authentication[\s\S]*denied`),
			},
			{
				Config: testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-2", "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_git_authentication.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_git_authentication.test", "personal_access_token_version", "v2"),
					func(*terraform.State) error {
						if got := server.service.hostAuthenticationUpdateCallCount("git-auth-1"); got != 2 {
							return fmt.Errorf("UpdateHostAuthenticationToken call count = %d, want 2", got)
						}
						if !server.service.hostAuthenticationPATUpdated("git-auth-1", "pat-2") {
							return errors.New("PAT was not rotated after retrying the update")
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAccGitAuthenticationDeleteNotFound(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}
	server.service.setNextHostAuthenticationDeleteError(connect.NewError(connect.CodeNotFound, errors.New("Git authentication not found")))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if got := server.service.hostAuthenticationDeleteCallCount("git-auth-1"); got != 1 {
				return fmt.Errorf("DeleteHostAuthenticationToken call count = %d, want 1", got)
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-1", "v1"),
		}},
	})
}

func TestAccGitAuthenticationDeleteAPIFailure(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			if !server.service.hostAuthenticationDeleted("git-auth-1") {
				return errors.New("git-auth-1 was not deleted")
			}
			if got := server.service.hostAuthenticationDeleteCallCount("git-auth-1"); got != 2 {
				return fmt.Errorf("DeleteHostAuthenticationToken call count = %d, want 2", got)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: testAccGitAuthenticationConfig(server.URL, "scm-1", "pat-1", "v1")},
			{
				PreConfig: func() {
					server.service.setNextHostAuthenticationDeleteError(errors.New("delete Git authentication denied"))
				},
				Config:      testAccProviderConfig(server.URL),
				ExpectError: regexp.MustCompile(`Unable to Delete Ona Git Authentication[\s\S]*delete Git authentication[\s\S]*denied`),
			},
			{Config: testAccProviderConfig(server.URL)},
		},
	})
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
