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
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	accessControlOrgID            = "11111111-1111-4111-8111-111111111111"
	accessControlGroupID          = "22222222-2222-4222-8222-222222222222"
	accessControlMembershipID     = "33333333-3333-4333-8333-333333333333"
	accessControlServiceAccountID = "44444444-4444-4444-8444-444444444444"
	accessControlAssignmentID     = "55555555-5555-4555-8555-555555555555"
	accessControlOtherServiceID   = "66666666-6666-4666-8666-666666666666"
	accessControlTeamID           = "77777777-7777-4777-8777-777777777777"
	accessControlCreatedAt        = "2026-01-02T03:04:05Z"
	accessControlUpdatedAt        = "2026-01-03T03:04:05Z"
)

func TestAccGroupResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.groupDeleted(accessControlGroupID) {
				return errors.New("group was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccGroupResourceConfig(server.URL, "Terraform Admins", "Initial description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_group.test", "id", accessControlGroupID),
					resource.TestCheckResourceAttr("ona_group.test", "name", "Terraform Admins"),
					resource.TestCheckResourceAttr("ona_group.test", "description", "Initial description"),
					resource.TestCheckResourceAttr("ona_group.test", "created_at", accessControlCreatedAt),
					resource.TestCheckNoResourceAttr("ona_group.test", "organization_id"),
					resource.TestCheckNoResourceAttr("ona_group.test", "system_managed"),
					resource.TestCheckNoResourceAttr("ona_group.test", "direct_share"),
					resource.TestCheckNoResourceAttr("ona_group.test", "updated_at"),
					resource.TestCheckNoResourceAttr("ona_group.test", "member_count"),
				),
			},
			{
				Config: testAccGroupResourceConfig(server.URL, "Terraform Admins", "Initial description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccGroupResourceConfig(server.URL, "Terraform Operators", "Updated description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_group.test", "name", "Terraform Operators"),
					resource.TestCheckResourceAttr("ona_group.test", "description", "Updated description"),
				),
			},
		},
	})
}

func TestAccGroupResourceReadRemovesNotFound(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.groupDeleted(accessControlGroupID) {
				return errors.New("group was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccGroupResourceConfig(server.URL, "Terraform Admins", "Initial description"),
			},
			{
				PreConfig: func() {
					server.service.deleteGroup(accessControlGroupID)
				},
				Config: testAccGroupResourceConfig(server.URL, "Terraform Admins", "Initial description"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccGroupResourceImportStateSupportsLegacyAndIdentity(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.groupDeleted(accessControlGroupID) {
				return errors.New("group was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccGroupResourceConfig(server.URL, "Terraform Admins", "Initial description"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentity("ona_group.test", map[string]knownvalue.Check{"id": knownvalue.StringExact(accessControlGroupID)}),
					statecheck.ExpectIdentityValueMatchesState("ona_group.test", tfjsonpath.New("id")),
				},
			},
			{
				ResourceName:      "ona_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:    "ona_group.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group.test", plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

func TestAccTeamResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.teamDeleted() {
				return errors.New("team was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_team.test", "id", accessControlTeamID),
					resource.TestCheckResourceAttr("ona_team.test", "name", "Platform Engineering"),
					resource.TestCheckResourceAttr("ona_team.test", "created_at", accessControlCreatedAt),
					resource.TestCheckNoResourceAttr("ona_team.test", "organization_id"),
					resource.TestCheckNoResourceAttr("ona_team.test", "updated_at"),
					resource.TestCheckNoResourceAttr("ona_team.test", "member_count"),
					resource.TestCheckNoResourceAttr("ona_team.test", "creator_id"),
				),
			},
			{
				Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_team.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccTeamResourceConfig(server.URL, "Developer Productivity"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_team.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("ona_team.test", "name", "Developer Productivity"),
			},
			{
				Config: testAccTeamResourceConfig(server.URL, "Developer Productivity"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccTeamResourceReadRemovesNotFound(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.teamDeleted() {
				return errors.New("team was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
			},
			{
				PreConfig: func() {
					server.service.deleteTeam(accessControlTeamID)
				},
				Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_team.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccTeamResourceDeleteAcceptsNotFound(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.teamDeleted() {
				return errors.New("team was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
			},
			{
				PreConfig: func() {
					server.service.setNextTeamDeleteError(connect.NewError(connect.CodeNotFound, errors.New("team not found")))
				},
				Config: testAccProviderConfig(server.URL),
			},
		},
	})
}

func TestAccTeamResourceNameValidation(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.teamDeleted() {
				return errors.New("team was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config:      testAccTeamResourceConfig(server.URL, "ab"),
				ExpectError: regexp.MustCompile("string length must be between 3 and 80"),
			},
			{
				Config: testAccTeamResourceConfig(server.URL, "abc"),
				Check:  resource.TestCheckResourceAttr("ona_team.test", "name", "abc"),
			},
			{
				Config:      testAccTeamResourceConfig(server.URL, strings.Repeat("x", 81)),
				ExpectError: regexp.MustCompile("string length must be between 3 and 80"),
			},
			{
				Config: testAccTeamResourceConfig(server.URL, strings.Repeat("x", 80)),
				Check:  resource.TestCheckResourceAttr("ona_team.test", "name", strings.Repeat("x", 80)),
			},
		},
	})
}

func TestAccTeamResourceRejectsEmptyCreateResponse(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.setNextTeamCreateEmpty()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				ExpectError: regexp.MustCompile(`Unable to Create Ona Team[\s\S]*empty team`),
			},
		},
	})
}

func TestAccTeamResourceRejectsEmptyUpdateResponse(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.teamDeleted() {
				return errors.New("team was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
			},
			{
				PreConfig: func() {
					server.service.setNextTeamUpdateEmpty()
				},
				Config:      testAccTeamResourceConfig(server.URL, "Developer Productivity"),
				ExpectError: regexp.MustCompile(`Unable to Update Ona Team[\s\S]*empty team`),
			},
			{
				Config: testAccTeamResourceConfig(server.URL, "Developer Productivity"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccTeamResourceReadErrorPreservesState(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.teamDeleted() {
				return errors.New("team was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
			},
			{
				PreConfig: func() {
					server.service.setNextTeamGetError(errors.New("temporary team read failure"))
				},
				Config:      testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				ExpectError: regexp.MustCompile(`Unable to Read Ona Team[\s\S]*temporary team read failure`),
			},
			{
				Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccTeamResourceEmptyReadResponsePreservesState(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.teamDeleted() {
				return errors.New("team was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
			},
			{
				PreConfig: func() {
					server.service.setNextTeamGetEmpty()
				},
				Config:      testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				ExpectError: regexp.MustCompile(`Unable to Read Ona Team[\s\S]*empty team`),
			},
			{
				Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccTeamResourcePersistsIDBeforePostCreateReadError(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.setNextTeamGetError(errors.New("post-create read failure"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.teamDeleted() {
				return errors.New("team was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config:      testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				ExpectError: regexp.MustCompile(`Unable to Read Created Ona Team[\s\S]*post-create read failure`),
			},
			{
				Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_team.test", plancheck.ResourceActionReplace),
					},
				},
				Check: func(state *terraform.State) error {
					createCalls, deleteCalls := server.service.teamMutationCalls()
					if createCalls != 2 || deleteCalls != 1 {
						return fmt.Errorf("expected two team creates and one cleanup delete, got %d creates and %d deletes", createCalls, deleteCalls)
					}
					return nil
				},
			},
		},
	})
}

func TestAccTeamResourceAPIErrors(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		server := newAccessControlAPIServer(t)
		t.Cleanup(server.Close)
		server.service.setNextTeamCreateError(errors.New("create team denied"))

		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreCheck(t) },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config:      testAccTeamResourceConfig(server.URL, "Platform Engineering"),
					ExpectError: regexp.MustCompile(`Unable to Create Ona Team[\s\S]*create team denied`),
				},
			},
		})
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()

		server := newAccessControlAPIServer(t)
		t.Cleanup(server.Close)

		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreCheck(t) },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			CheckDestroy: func(state *terraform.State) error {
				if !server.service.teamDeleted() {
					return errors.New("team was not deleted")
				}
				return nil
			},
			Steps: []resource.TestStep{
				{
					Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				},
				{
					PreConfig: func() {
						server.service.setNextTeamUpdateError(errors.New("update team denied"))
					},
					Config:      testAccTeamResourceConfig(server.URL, "Developer Productivity"),
					ExpectError: regexp.MustCompile(`Unable to Update Ona Team[\s\S]*update team denied`),
				},
			},
		})
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := newAccessControlAPIServer(t)
		t.Cleanup(server.Close)

		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreCheck(t) },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			CheckDestroy: func(state *terraform.State) error {
				if !server.service.teamDeleted() {
					return errors.New("team was not deleted")
				}
				return nil
			},
			Steps: []resource.TestStep{
				{
					Config: testAccTeamResourceConfig(server.URL, "Platform Engineering"),
				},
				{
					PreConfig: func() {
						server.service.setNextTeamDeleteError(errors.New("delete team denied"))
					},
					Config:      testAccProviderConfig(server.URL),
					ExpectError: regexp.MustCompile(`Unable to Delete Ona Team[\s\S]*delete team denied`),
				},
				{
					Config: testAccProviderConfig(server.URL),
				},
			},
		})
	})
}

func TestAccGroupMembershipResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.membershipDeleted(accessControlMembershipID) {
				return errors.New("membership was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_group_membership.test", "id", accessControlMembershipID),
					resource.TestCheckResourceAttr("ona_group_membership.test", "group_id", accessControlGroupID),
					resource.TestCheckResourceAttr("ona_group_membership.test", "service_account_id", accessControlServiceAccountID),
					resource.TestCheckNoResourceAttr("ona_group_membership.test", "principal"),
					resource.TestCheckNoResourceAttr("ona_group_membership.test", "name"),
					resource.TestCheckNoResourceAttr("ona_group_membership.test", "avatar_url"),
				),
			},
			{
				Config: testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_group_membership.test",
				ImportState:       true,
				ImportStateId:     accessControlGroupID + "/" + accessControlServiceAccountID,
				ImportStateVerify: true,
			},
			{
				Config: testAccGroupMembershipResourceConfig(server.URL, accessControlOtherServiceID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group_membership.test", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})
}

func TestGroupMembershipImportStateEquivalence(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
			},
			{
				ResourceName:      "ona_group_membership.test",
				ImportState:       true,
				ImportStateId:     accessControlGroupID + "/" + accessControlServiceAccountID,
				ImportStateVerify: true,
				ImportStateCheck:  checkGroupMembershipImportState,
			},
			{
				ResourceName:    "ona_group_membership.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group_membership.test", plancheck.ResourceActionNoop),
						plancheck.ExpectKnownValue("ona_group_membership.test", tfjsonpath.New("group_id"), knownvalue.StringExact(accessControlGroupID)),
						plancheck.ExpectKnownValue("ona_group_membership.test", tfjsonpath.New("service_account_id"), knownvalue.StringExact(accessControlServiceAccountID)),
					},
				},
			},
		},
	})
}

func checkGroupMembershipImportState(states []*terraform.InstanceState) error {
	if len(states) != 1 {
		return fmt.Errorf("expected 1 imported state, got %d", len(states))
	}

	for attribute, expected := range map[string]string{
		"group_id":           accessControlGroupID,
		"service_account_id": accessControlServiceAccountID,
	} {
		if actual := states[0].Attributes[attribute]; actual != expected {
			return fmt.Errorf("expected imported %s %q, got %q", attribute, expected, actual)
		}
	}

	return nil
}

func TestAccGroupMembershipResourceReadRemovesMissingMember(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.membershipDeleted(accessControlMembershipID) {
				return errors.New("membership was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
			},
			{
				PreConfig: func() {
					server.service.deleteMembership(accessControlMembershipID)
				},
				Config: testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group_membership.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccOrganizationRoleAssignmentResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted(accessControlAssignmentID) {
				return errors.New("role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationRoleAssignmentResourceConfig(server.URL, "runners_admin"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_organization_role_assignment.test", "id", accessControlAssignmentID),
					resource.TestCheckResourceAttr("ona_organization_role_assignment.test", "group_id", accessControlGroupID),
					resource.TestCheckNoResourceAttr("ona_organization_role_assignment.test", "organization_id"),
					resource.TestCheckResourceAttr("ona_organization_role_assignment.test", "role", "runners_admin"),
					resource.TestCheckNoResourceAttr("ona_organization_role_assignment.test", "resource_type"),
					resource.TestCheckNoResourceAttr("ona_organization_role_assignment.test", "resource_id"),
				),
			},
			{
				Config: testAccOrganizationRoleAssignmentResourceConfig(server.URL, "runners_admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:      "ona_organization_role_assignment.test",
				ImportState:       true,
				ImportStateId:     accessControlGroupID + "/runners_admin",
				ImportStateVerify: true,
			},
			{
				Config: testAccOrganizationRoleAssignmentResourceConfig(server.URL, "projects_admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_organization_role_assignment.test", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})
}

func TestOrganizationRoleAssignmentResourceImports(t *testing.T) {
	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		PreCheck:                 func() {},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationRoleAssignmentResourceConfig(server.URL, "runners_admin"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_organization_role_assignment.test", "id", accessControlAssignmentID),
					resource.TestCheckResourceAttr("ona_organization_role_assignment.test", "resource_type", "organization"),
					resource.TestCheckResourceAttr("ona_organization_role_assignment.test", "resource_id", accessControlOrgID),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentity("ona_organization_role_assignment.test", map[string]knownvalue.Check{
						"group_id":        knownvalue.StringExact(accessControlGroupID),
						"organization_id": knownvalue.StringExact(accessControlOrgID),
						"role":            knownvalue.StringExact("runners_admin"),
					}),
				},
			},
			{
				ResourceName:      "ona_organization_role_assignment.test",
				ImportState:       true,
				ImportStateId:     accessControlGroupID + "/" + accessControlOrgID + "/runners_admin",
				ImportStateVerify: true,
			},
			{
				ResourceName:    "ona_organization_role_assignment.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue("ona_organization_role_assignment.test", tfjsonpath.New("id"), knownvalue.StringExact(accessControlAssignmentID)),
						plancheck.ExpectKnownValue("ona_organization_role_assignment.test", tfjsonpath.New("group_id"), knownvalue.StringExact(accessControlGroupID)),
						plancheck.ExpectKnownValue("ona_organization_role_assignment.test", tfjsonpath.New("organization_id"), knownvalue.StringExact(accessControlOrgID)),
						plancheck.ExpectKnownValue("ona_organization_role_assignment.test", tfjsonpath.New("role"), knownvalue.StringExact("runners_admin")),
						plancheck.ExpectKnownValue("ona_organization_role_assignment.test", tfjsonpath.New("resource_type"), knownvalue.StringExact("organization")),
						plancheck.ExpectKnownValue("ona_organization_role_assignment.test", tfjsonpath.New("resource_id"), knownvalue.StringExact(accessControlOrgID)),
					},
				},
			},
		},
	})
}

func TestAccOrganizationRoleAssignmentResourceReadRemovesMissingAssignment(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted(accessControlAssignmentID) {
				return errors.New("role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationRoleAssignmentResourceConfig(server.URL, "runners_admin"),
			},
			{
				PreConfig: func() {
					server.service.deleteAssignment(accessControlAssignmentID)
				},
				Config: testAccOrganizationRoleAssignmentResourceConfig(server.URL, "runners_admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_organization_role_assignment.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func testAccGroupResourceConfig(host string, name string, description string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_group" "test" {
  name        = %[2]q
  description = %[3]q
}
`, host, name, description)
}

func testAccGroupMembershipResourceConfig(host string, serviceAccountID string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_group_membership" "test" {
  group_id           = %[2]q
  service_account_id = %[3]q
}
`, host, accessControlGroupID, serviceAccountID)
}

func testAccTeamResourceConfig(host string, name string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_team" "test" {
  name = %[2]q
}
`, host, name)
}

func testAccProviderConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}
`, host)
}

func testAccOrganizationRoleAssignmentResourceConfig(host string, role string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_organization_role_assignment" "test" {
  group_id = %[2]q
  role     = %[3]q
}
`, host, accessControlGroupID, role)
}

type accessControlAPIServer struct {
	*httptest.Server
	service *fakeGroupService
}

func newAccessControlAPIServer(t *testing.T) *accessControlAPIServer {
	t.Helper()

	service := &fakeGroupService{
		groups:              map[string]*v1.Group{},
		deletedGroups:       map[string]bool{},
		teams:               map[string]*v1.Team{},
		deletedTeams:        map[string]bool{},
		memberships:         map[string]*v1.GroupMembership{},
		deletedMemberships:  map[string]bool{},
		assignments:         map[string]*v1.RoleAssignment{},
		deletedAssignments:  map[string]bool{},
		serviceAccountNames: map[string]string{accessControlServiceAccountID: "Terraform Service Account", accessControlOtherServiceID: "Other Service Account"},
	}
	mux := http.NewServeMux()
	groupPath, groupHandler := v1connect.NewGroupServiceHandler(service)
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(service)
	teamPath, teamHandler := v1connect.NewTeamServiceHandler(service)
	mux.Handle(groupPath, groupHandler)
	mux.Handle(identityPath, identityHandler)
	mux.Handle(teamPath, teamHandler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	return &accessControlAPIServer{
		Server:  server,
		service: service,
	}
}

type fakeGroupService struct {
	v1connect.UnimplementedGroupServiceHandler
	v1connect.UnimplementedIdentityServiceHandler
	v1connect.UnimplementedTeamServiceHandler

	mu                  sync.Mutex
	groups              map[string]*v1.Group
	deletedGroups       map[string]bool
	teams               map[string]*v1.Team
	deletedTeams        map[string]bool
	nextTeamCreateEmpty bool
	nextTeamCreateError error
	teamCreateCallCount int
	teamDeleteCallCount int
	nextTeamGetEmpty    bool
	nextTeamGetError    error
	nextTeamDeleteError error
	nextTeamUpdateEmpty bool
	nextTeamUpdateError error
	memberships         map[string]*v1.GroupMembership
	deletedMemberships  map[string]bool
	assignments         map[string]*v1.RoleAssignment
	deletedAssignments  map[string]bool
	serviceAccountNames map[string]string
}

func (s *fakeGroupService) CreateGroup(ctx context.Context, req *connect.Request[v1.CreateGroupRequest]) (*connect.Response[v1.CreateGroupResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	group := &v1.Group{
		Id:             accessControlGroupID,
		OrganizationId: req.Msg.GetOrganizationId(),
		Name:           req.Msg.GetName(),
		Description:    req.Msg.GetDescription(),
		CreatedAt:      timestampForTest(accessControlCreatedAt),
		UpdatedAt:      timestampForTest(accessControlCreatedAt),
	}
	s.groups[group.GetId()] = group
	s.deletedGroups[group.GetId()] = false
	return connect.NewResponse(&v1.CreateGroupResponse{Group: cloneGroup(group)}), nil
}

func (s *fakeGroupService) GetGroup(ctx context.Context, req *connect.Request[v1.GetGroupRequest]) (*connect.Response[v1.GetGroupResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	group := s.groups[req.Msg.GetId()]
	if group == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("group not found"))
	}
	return connect.NewResponse(&v1.GetGroupResponse{Group: cloneGroup(group)}), nil
}

func (s *fakeGroupService) UpdateGroup(ctx context.Context, req *connect.Request[v1.UpdateGroupRequest]) (*connect.Response[v1.UpdateGroupResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	group := s.groups[req.Msg.GetGroupId()]
	if group == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("group not found"))
	}
	group.Name = req.Msg.GetName()
	group.Description = req.Msg.GetDescription()
	group.UpdatedAt = timestampForTest(accessControlUpdatedAt)
	return connect.NewResponse(&v1.UpdateGroupResponse{Group: cloneGroup(group)}), nil
}

func (s *fakeGroupService) DeleteGroup(ctx context.Context, req *connect.Request[v1.DeleteGroupRequest]) (*connect.Response[v1.DeleteGroupResponse], error) {
	s.deleteGroup(req.Msg.GetGroupId())
	return connect.NewResponse(&v1.DeleteGroupResponse{}), nil
}

func (s *fakeGroupService) CreateTeam(ctx context.Context, req *connect.Request[v1.CreateTeamRequest]) (*connect.Response[v1.CreateTeamResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nextTeamCreateError != nil {
		err := s.nextTeamCreateError
		s.nextTeamCreateError = nil
		return nil, err
	}
	if s.nextTeamCreateEmpty {
		s.nextTeamCreateEmpty = false
		return connect.NewResponse(&v1.CreateTeamResponse{}), nil
	}
	s.teamCreateCallCount++

	team := &v1.Team{
		Id:             accessControlTeamID,
		OrganizationId: req.Msg.GetOrganizationId(),
		Name:           req.Msg.GetName(),
		CreatedAt:      timestampForTest(accessControlCreatedAt),
		UpdatedAt:      timestampForTest(accessControlCreatedAt),
		MemberCount:    7,
	}
	s.teams[team.GetId()] = team
	s.deletedTeams[team.GetId()] = false
	return connect.NewResponse(&v1.CreateTeamResponse{Team: cloneTeam(team)}), nil
}

func (s *fakeGroupService) GetTeam(ctx context.Context, req *connect.Request[v1.GetTeamRequest]) (*connect.Response[v1.GetTeamResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nextTeamGetError != nil {
		err := s.nextTeamGetError
		s.nextTeamGetError = nil
		return nil, err
	}
	if s.nextTeamGetEmpty {
		s.nextTeamGetEmpty = false
		return connect.NewResponse(&v1.GetTeamResponse{}), nil
	}

	team := s.teams[req.Msg.GetTeamId()]
	if team == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("team not found"))
	}
	return connect.NewResponse(&v1.GetTeamResponse{Team: cloneTeam(team)}), nil
}

func (s *fakeGroupService) UpdateTeam(ctx context.Context, req *connect.Request[v1.UpdateTeamRequest]) (*connect.Response[v1.UpdateTeamResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nextTeamUpdateError != nil {
		err := s.nextTeamUpdateError
		s.nextTeamUpdateError = nil
		return nil, err
	}

	team := s.teams[req.Msg.GetTeamId()]
	if team == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("team not found"))
	}
	team.Name = req.Msg.GetName()
	team.UpdatedAt = timestampForTest(accessControlUpdatedAt)
	if s.nextTeamUpdateEmpty {
		s.nextTeamUpdateEmpty = false
		return connect.NewResponse(&v1.UpdateTeamResponse{}), nil
	}
	return connect.NewResponse(&v1.UpdateTeamResponse{Team: cloneTeam(team)}), nil
}

func (s *fakeGroupService) DeleteTeam(ctx context.Context, req *connect.Request[v1.DeleteTeamRequest]) (*connect.Response[v1.DeleteTeamResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.teamDeleteCallCount++

	if s.nextTeamDeleteError != nil {
		err := s.nextTeamDeleteError
		s.nextTeamDeleteError = nil
		if connect.CodeOf(err) == connect.CodeNotFound {
			delete(s.teams, req.Msg.GetTeamId())
			s.deletedTeams[req.Msg.GetTeamId()] = true
		}
		return nil, err
	}
	delete(s.teams, req.Msg.GetTeamId())
	s.deletedTeams[req.Msg.GetTeamId()] = true
	return connect.NewResponse(&v1.DeleteTeamResponse{}), nil
}

func (s *fakeGroupService) CreateMembership(ctx context.Context, req *connect.Request[v1.CreateMembershipRequest]) (*connect.Response[v1.CreateMembershipResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	member := &v1.GroupMembership{
		Id:      accessControlMembershipID,
		GroupId: req.Msg.GetGroupId(),
		Subject: &v1.Subject{
			Id:        req.Msg.GetSubject().GetId(),
			Principal: req.Msg.GetSubject().GetPrincipal(),
		},
		Name: s.serviceAccountNames[req.Msg.GetSubject().GetId()],
	}
	s.memberships[memberKey(member.GetGroupId(), member.GetSubject().GetId())] = member
	s.deletedMemberships[member.GetId()] = false
	return connect.NewResponse(&v1.CreateMembershipResponse{Member: cloneMembership(member)}), nil
}

func (s *fakeGroupService) GetMembership(ctx context.Context, req *connect.Request[v1.GetMembershipRequest]) (*connect.Response[v1.GetMembershipResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	member := s.memberships[memberKey(req.Msg.GetGroupId(), req.Msg.GetSubject().GetId())]
	if member == nil {
		return connect.NewResponse(&v1.GetMembershipResponse{}), nil
	}
	return connect.NewResponse(&v1.GetMembershipResponse{Member: cloneMembership(member)}), nil
}

func (s *fakeGroupService) DeleteMembership(ctx context.Context, req *connect.Request[v1.DeleteMembershipRequest]) (*connect.Response[v1.DeleteMembershipResponse], error) {
	s.deleteMembership(req.Msg.GetMembershipId())
	return connect.NewResponse(&v1.DeleteMembershipResponse{}), nil
}

func (s *fakeGroupService) CreateRoleAssignment(ctx context.Context, req *connect.Request[v1.CreateRoleAssignmentRequest]) (*connect.Response[v1.CreateRoleAssignmentResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	assignment := &v1.RoleAssignment{
		Id:             accessControlAssignmentID,
		GroupId:        req.Msg.GetGroupId(),
		OrganizationId: req.Msg.GetResourceId(),
		ResourceId:     req.Msg.GetResourceId(),
		ResourceType:   req.Msg.GetResourceType(),
		ResourceRole:   req.Msg.GetResourceRole(),
	}
	s.assignments[assignmentKey(assignment.GetGroupId(), assignment.GetResourceId(), assignment.GetResourceRole())] = assignment
	s.deletedAssignments[assignment.GetId()] = false
	return connect.NewResponse(&v1.CreateRoleAssignmentResponse{Assignment: cloneAssignment(assignment)}), nil
}

func (s *fakeGroupService) ListRoleAssignments(ctx context.Context, req *connect.Request[v1.ListRoleAssignmentsRequest]) (*connect.Response[v1.ListRoleAssignmentsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var assignments []*v1.RoleAssignment
	for _, assignment := range s.assignments {
		if !matchesRoleAssignmentFilter(assignment, req.Msg.GetFilter()) {
			continue
		}
		assignments = append(assignments, cloneAssignment(assignment))
	}
	return connect.NewResponse(&v1.ListRoleAssignmentsResponse{Assignments: assignments}), nil
}

func (s *fakeGroupService) DeleteRoleAssignment(ctx context.Context, req *connect.Request[v1.DeleteRoleAssignmentRequest]) (*connect.Response[v1.DeleteRoleAssignmentResponse], error) {
	s.deleteAssignment(req.Msg.GetAssignmentId())
	return connect.NewResponse(&v1.DeleteRoleAssignmentResponse{}), nil
}

func (s *fakeGroupService) GetAuthenticatedIdentity(ctx context.Context, req *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{
		Subject: &v1.Subject{
			Id:        accessControlServiceAccountID,
			Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT,
		},
		OrganizationId: accessControlOrgID,
	}), nil
}

func (s *fakeGroupService) seedGroup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.groups[accessControlGroupID] = &v1.Group{
		Id:             accessControlGroupID,
		OrganizationId: accessControlOrgID,
		Name:           "Terraform Admins",
		Description:    "Seeded group",
		CreatedAt:      timestampForTest(accessControlCreatedAt),
		UpdatedAt:      timestampForTest(accessControlCreatedAt),
	}
}

func (s *fakeGroupService) deleteGroup(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.groups, id)
	s.deletedGroups[id] = true
}

func (s *fakeGroupService) groupDeleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletedGroups[id]
}

func (s *fakeGroupService) deleteTeam(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.teams, id)
	s.deletedTeams[id] = true
}

func (s *fakeGroupService) teamDeleted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletedTeams[accessControlTeamID]
}

func (s *fakeGroupService) teamMutationCalls() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.teamCreateCallCount, s.teamDeleteCallCount
}

func (s *fakeGroupService) setNextTeamDeleteError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextTeamDeleteError = err
}

func (s *fakeGroupService) setNextTeamCreateEmpty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextTeamCreateEmpty = true
}

func (s *fakeGroupService) setNextTeamCreateError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextTeamCreateError = err
}

func (s *fakeGroupService) setNextTeamGetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextTeamGetError = err
}

func (s *fakeGroupService) setNextTeamGetEmpty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextTeamGetEmpty = true
}

func (s *fakeGroupService) setNextTeamUpdateEmpty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextTeamUpdateEmpty = true
}

func (s *fakeGroupService) setNextTeamUpdateError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextTeamUpdateError = err
}

func (s *fakeGroupService) deleteMembership(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, member := range s.memberships {
		if member.GetId() == id {
			delete(s.memberships, key)
		}
	}
	s.deletedMemberships[id] = true
}

func (s *fakeGroupService) membershipDeleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletedMemberships[id]
}

func (s *fakeGroupService) deleteAssignment(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, assignment := range s.assignments {
		if assignment.GetId() == id {
			delete(s.assignments, key)
		}
	}
	s.deletedAssignments[id] = true
}

func (s *fakeGroupService) assignmentDeleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletedAssignments[id]
}

func memberKey(groupID string, serviceAccountID string) string {
	return groupID + "/" + serviceAccountID
}

func assignmentKey(groupID string, resourceID string, role v1.ResourceRole) string {
	return fmt.Sprintf("%s/%s/%d", groupID, resourceID, role)
}

func matchesRoleAssignmentFilter(assignment *v1.RoleAssignment, filter *v1.ListRoleAssignmentsRequest_Filter) bool {
	if filter == nil {
		return true
	}
	if filter.GetGroupId() != "" && assignment.GetGroupId() != filter.GetGroupId() {
		return false
	}
	if filter.GetResourceId() != "" && assignment.GetResourceId() != filter.GetResourceId() {
		return false
	}
	if len(filter.GetResourceTypes()) > 0 && !containsResourceType(filter.GetResourceTypes(), assignment.GetResourceType()) {
		return false
	}
	if len(filter.GetResourceRoles()) > 0 && !containsResourceRole(filter.GetResourceRoles(), assignment.GetResourceRole()) {
		return false
	}
	return true
}

func containsResourceType(values []v1.ResourceType, value v1.ResourceType) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func containsResourceRole(values []v1.ResourceRole, value v1.ResourceRole) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func cloneGroup(group *v1.Group) *v1.Group {
	return proto.CloneOf(group)
}

func cloneTeam(team *v1.Team) *v1.Team {
	return proto.CloneOf(team)
}

func cloneMembership(member *v1.GroupMembership) *v1.GroupMembership {
	return proto.CloneOf(member)
}

func cloneAssignment(assignment *v1.RoleAssignment) *v1.RoleAssignment {
	return proto.CloneOf(assignment)
}

func timestampForTest(value string) *timestamppb.Timestamp {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return timestamppb.New(parsed)
}
