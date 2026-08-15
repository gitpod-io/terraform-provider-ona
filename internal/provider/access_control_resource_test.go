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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/gitpod-sdk-go/v1/v1connect"
	"github.com/google/go-cmp/cmp"
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
	accessControlOrgID             = "11111111-1111-4111-8111-111111111111"
	accessControlGroupID           = "22222222-2222-4222-8222-222222222222"
	accessControlOtherGroupID      = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	accessControlMembershipID      = "33333333-3333-4333-8333-333333333333"
	accessControlServiceAccountID  = "44444444-4444-4444-8444-444444444444"
	accessControlAssignmentID      = "55555555-5555-4555-8555-555555555555"
	accessControlOtherServiceID    = "66666666-6666-4666-8666-666666666666"
	accessControlTeamID            = "77777777-7777-4777-8777-777777777777"
	accessControlUserID            = "88888888-8888-4888-8888-888888888888"
	accessControlOtherUserID       = "99999999-9999-4999-8999-999999999999"
	accessControlAutomationID      = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	accessControlOtherAutomationID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	accessControlDuplicateID       = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	accessControlProjectID         = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	accessControlOtherProjectID    = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	accessControlRunnerID          = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	accessControlOtherRunnerID     = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	accessControlReplacementID     = "12121212-1212-4212-8212-121212121212"
	accessControlOrgMembersGroupID = "13131313-1313-4313-8313-131313131313"
	accessControlCreatedAt         = "2026-01-02T03:04:05Z"
	accessControlUpdatedAt         = "2026-01-03T03:04:05Z"
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
				ResourceName:    "ona_team.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_team.test", plancheck.ResourceActionNoop),
					},
				},
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

func TestAccGroupMembershipResourceValidation(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupMembershipResourceConfigWithoutMember(server.URL),
				ExpectError: regexp.MustCompile(`Missing Attribute Configuration[\s\S]*Exactly one of these attributes must be configured`),
			},
			{
				Config:      testAccGroupMembershipResourceConfigWithBothMembers(server.URL),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination[\s\S]*Exactly one of these attributes must be configured`),
			},
		},
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
			if !server.service.membershipDeleted() {
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
					resource.TestCheckNoResourceAttr("ona_group_membership.test", "user_id"),
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
				ResourceName:      "ona_group_membership.test",
				ImportState:       true,
				ImportStateId:     accessControlGroupID + "/service_account/" + accessControlServiceAccountID,
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

func TestAccUserGroupMembershipResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.membershipDeleted() {
				return errors.New("membership was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipResourceConfig(server.URL, accessControlGroupID, accessControlUserID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_group_membership.test", "id", accessControlMembershipID),
					resource.TestCheckResourceAttr("ona_group_membership.test", "group_id", accessControlGroupID),
					resource.TestCheckResourceAttr("ona_group_membership.test", "user_id", accessControlUserID),
					resource.TestCheckNoResourceAttr("ona_group_membership.test", "service_account_id"),
				),
			},
			{
				Config: testAccUserGroupMembershipResourceConfig(server.URL, accessControlGroupID, accessControlUserID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "ona_group_membership.test",
				ImportState:       true,
				ImportStateId:     accessControlGroupID + "/user/" + accessControlUserID,
				ImportStateVerify: true,
			},
			{
				Config: testAccUserGroupMembershipResourceConfig(server.URL, accessControlGroupID, accessControlOtherUserID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group_membership.test", plancheck.ResourceActionReplace),
					},
				},
			},
			{
				Config: testAccUserGroupMembershipResourceConfig(server.URL, accessControlOtherGroupID, accessControlOtherUserID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group_membership.test", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})
}

func TestAccGroupMembershipResourceReplacesPrincipalType(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.membershipDeleted() {
				return errors.New("membership was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
			},
			{
				Config: testAccUserGroupMembershipResourceConfig(server.URL, accessControlGroupID, accessControlServiceAccountID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group_membership.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_group_membership.test", "user_id", accessControlServiceAccountID),
					resource.TestCheckNoResourceAttr("ona_group_membership.test", "service_account_id"),
				),
			},
			{
				Config: testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group_membership.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_group_membership.test", "service_account_id", accessControlServiceAccountID),
					resource.TestCheckNoResourceAttr("ona_group_membership.test", "user_id"),
				),
			},
		},
	})
}

func TestAccGroupMembershipResourcePersistsIDBeforeCreateResponseMappingError(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()
	server.service.setNextMembershipCreateResponsePrincipal(v1.Principal_PRINCIPAL_RUNNER)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.membershipDeleted() {
				return errors.New("membership was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
				ExpectError: regexp.MustCompile(`Unable to Create Ona Group Membership[\s\S]*unsupported membership principal "PRINCIPAL_RUNNER"`),
			},
			{
				Config: testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group_membership.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_group_membership.test", "id", accessControlMembershipID),
					func(state *terraform.State) error {
						createCalls, deleteCalls := server.service.membershipMutationCalls()
						if createCalls != 2 || deleteCalls != 1 {
							return fmt.Errorf("expected two membership creates and one cleanup delete, got %d creates and %d deletes", createCalls, deleteCalls)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAccGroupMembershipResourcePersistsIdentityAfterEmptyCreateResponse(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()
	server.service.setNextMembershipCreateResponseEmpty()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.membershipDeleted() {
				return errors.New("membership was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
				ExpectError: regexp.MustCompile(`Unable to Create Ona Group Membership[\s\S]*The Ona API returned an empty membership`),
			},
			{
				Config: testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group_membership.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_group_membership.test", "id", accessControlMembershipID),
					func(state *terraform.State) error {
						createCalls, deleteCalls := server.service.membershipMutationCalls()
						if createCalls != 2 || deleteCalls != 1 {
							return fmt.Errorf("expected two membership creates and one cleanup delete, got %d creates and %d deletes", createCalls, deleteCalls)
						}
						return nil
					},
				),
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
				ImportStateCheck:  checkGroupMembershipImportState("service_account_id", accessControlServiceAccountID),
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
						plancheck.ExpectKnownValue("ona_group_membership.test", tfjsonpath.New("user_id"), knownvalue.Null()),
					},
				},
			},
		},
	})
}

func TestUserGroupMembershipImportStateEquivalence(t *testing.T) {
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
				Config: testAccUserGroupMembershipResourceConfig(server.URL, accessControlGroupID, accessControlUserID),
			},
			{
				ResourceName:      "ona_group_membership.test",
				ImportState:       true,
				ImportStateId:     accessControlGroupID + "/user/" + accessControlUserID,
				ImportStateVerify: true,
				ImportStateCheck:  checkGroupMembershipImportState("user_id", accessControlUserID),
			},
			{
				ResourceName:    "ona_group_membership.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group_membership.test", plancheck.ResourceActionNoop),
						plancheck.ExpectKnownValue("ona_group_membership.test", tfjsonpath.New("group_id"), knownvalue.StringExact(accessControlGroupID)),
						plancheck.ExpectKnownValue("ona_group_membership.test", tfjsonpath.New("service_account_id"), knownvalue.Null()),
						plancheck.ExpectKnownValue("ona_group_membership.test", tfjsonpath.New("user_id"), knownvalue.StringExact(accessControlUserID)),
					},
				},
			},
		},
	})
}

func checkGroupMembershipImportState(memberAttribute, memberID string) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("expected 1 imported state, got %d", len(states))
		}

		for attribute, expected := range map[string]string{
			"group_id":      accessControlGroupID,
			memberAttribute: memberID,
		} {
			if actual := states[0].Attributes[attribute]; actual != expected {
				return fmt.Errorf("expected imported %s %q, got %q", attribute, expected, actual)
			}
		}
		otherAttribute := "user_id"
		if memberAttribute == otherAttribute {
			otherAttribute = "service_account_id"
		}
		if actual := states[0].Attributes[otherAttribute]; actual != "" {
			return fmt.Errorf("expected imported %s to be null, got %q", otherAttribute, actual)
		}

		return nil
	}
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
			if !server.service.membershipDeleted() {
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

func TestAccUserGroupMembershipResourceReadRemovesMissingMember(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.membershipDeleted() {
				return errors.New("membership was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupMembershipResourceConfig(server.URL, accessControlGroupID, accessControlUserID),
			},
			{
				PreConfig: func() {
					server.service.deleteMembership(accessControlMembershipID)
				},
				Config: testAccUserGroupMembershipResourceConfig(server.URL, accessControlGroupID, accessControlUserID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_group_membership.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccGroupMembershipResourceRejectsUnsupportedPrincipalOnRead(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
			},
			{
				PreConfig: func() {
					server.service.setMembershipPrincipal(accessControlMembershipID, v1.Principal_PRINCIPAL_RUNNER)
				},
				Config:      testAccGroupMembershipResourceConfig(server.URL, accessControlServiceAccountID),
				ExpectError: regexp.MustCompile(`Unable to Read Ona Group Membership[\s\S]*unsupported membership principal "PRINCIPAL_RUNNER"`),
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
			if !server.service.assignmentDeleted() {
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
				ImportStateId:     accessControlGroupID + "/runners_admin",
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
						plancheck.ExpectKnownValue("ona_organization_role_assignment.test", tfjsonpath.New("role"), knownvalue.StringExact("runners_admin")),
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
			if !server.service.assignmentDeleted() {
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

func TestAccAutomationRoleAssignmentResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted() {
				return errors.New("Automation role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_automation_role_assignment.test", "id", accessControlAssignmentID),
					resource.TestCheckResourceAttr("ona_automation_role_assignment.test", "automation_id", accessControlAutomationID),
					resource.TestCheckResourceAttr("ona_automation_role_assignment.test", "group_id", accessControlGroupID),
					resource.TestCheckResourceAttr("ona_automation_role_assignment.test", "role", "executor"),
					checkAutomationRoleAssignmentCreateRequest(server.service, automationRoleAssignmentRequestExpectation{
						Calls:        1,
						AutomationID: accessControlAutomationID,
						GroupID:      accessControlGroupID,
						ResourceType: v1.ResourceType_RESOURCE_TYPE_WORKFLOW,
						Role:         v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR,
					}),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentity("ona_automation_role_assignment.test", map[string]knownvalue.Check{
						"automation_id": knownvalue.StringExact(accessControlAutomationID),
						"group_id":      knownvalue.StringExact(accessControlGroupID),
						"role":          knownvalue.StringExact("executor"),
					}),
				},
			},
			{
				PreConfig: func() {
					server.service.seedAssignment(&v1.RoleAssignment{
						Id:           "00000000-0000-4000-8000-000000000000",
						GroupId:      accessControlOtherGroupID,
						ResourceId:   accessControlAutomationID,
						ResourceType: v1.ResourceType_RESOURCE_TYPE_WORKFLOW,
						ResourceRole: v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR,
					})
					server.service.seedAssignment(&v1.RoleAssignment{
						Id:           "11111111-0000-4000-8000-000000000000",
						GroupId:      accessControlGroupID,
						ResourceId:   accessControlAutomationID,
						ResourceType: v1.ResourceType_RESOURCE_TYPE_PROJECT,
						ResourceRole: v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR,
					})
					server.service.seedAssignment(&v1.RoleAssignment{
						Id:           "22222222-0000-4000-8000-000000000000",
						GroupId:      accessControlGroupID,
						ResourceId:   accessControlOtherAutomationID,
						ResourceType: v1.ResourceType_RESOURCE_TYPE_WORKFLOW,
						ResourceRole: v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR,
					})
					server.service.setRoleAssignmentListBehavior(1, true)
				},
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: checkAutomationRoleAssignmentListRequests(server.service),
			},
			{
				ResourceName:      "ona_automation_role_assignment.test",
				ImportState:       true,
				ImportStateId:     accessControlAutomationID + "/" + accessControlGroupID + "/executor",
				ImportStateVerify: true,
			},
			{
				ResourceName:    "ona_automation_role_assignment.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue("ona_automation_role_assignment.test", tfjsonpath.New("id"), knownvalue.StringExact(accessControlAssignmentID)),
						plancheck.ExpectKnownValue("ona_automation_role_assignment.test", tfjsonpath.New("automation_id"), knownvalue.StringExact(accessControlAutomationID)),
						plancheck.ExpectKnownValue("ona_automation_role_assignment.test", tfjsonpath.New("group_id"), knownvalue.StringExact(accessControlGroupID)),
						plancheck.ExpectKnownValue("ona_automation_role_assignment.test", tfjsonpath.New("role"), knownvalue.StringExact("executor")),
					},
				},
			},
			{
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_automation_role_assignment.test", plancheck.ResourceActionReplace),
					},
				},
			},
			{
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlOtherAutomationID, accessControlGroupID, "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_automation_role_assignment.test", plancheck.ResourceActionReplace),
					},
				},
			},
			{
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlOtherAutomationID, accessControlOtherGroupID, "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_automation_role_assignment.test", plancheck.ResourceActionReplace),
					},
				},
			},
		},
	})
}

func TestAccAutomationRoleAssignmentResourceDetectsDrift(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted() {
				return errors.New("Automation role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
			},
			{
				PreConfig: func() {
					server.service.deleteAssignment(accessControlAssignmentID)
				},
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_automation_role_assignment.test", plancheck.ResourceActionCreate),
					},
				},
			},
			{
				PreConfig: func() {
					server.service.replaceAssignmentRole(accessControlAssignmentID, v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_ADMIN)
				},
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_automation_role_assignment.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})

	type Expectation struct {
		CreateCalls int
	}
	expected := Expectation{CreateCalls: 3}
	createCalls, _ := server.service.roleAssignmentMutationCalls()
	got := Expectation{CreateCalls: createCalls}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("Automation role assignment drift calls mismatch (-want +got):\n%s", diff)
	}
}

func TestAccAutomationRoleAssignmentResourceCreateErrors(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		CreateCalls int
	}
	tests := []struct {
		Name        string
		Configure   func(*fakeGroupService)
		ExpectError *regexp.Regexp
	}{
		{
			Name: "enterprise_tier_error",
			Configure: func(service *fakeGroupService) {
				service.setNextRoleAssignmentCreateError(connect.NewError(connect.CodeFailedPrecondition, errors.New("sharing Automations with custom groups requires the Enterprise plan")))
			},
			ExpectError: regexp.MustCompile(`Unable to Create Ona Automation Role Assignment[\s\S]*required state[\s\S]*requires the Enterprise plan`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newAccessControlAPIServer(t)
			t.Cleanup(server.Close)
			server.service.seedGroup()
			tc.Configure(server.service)

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
						ExpectError: tc.ExpectError,
					},
				},
			})

			expected := Expectation{CreateCalls: 1}
			createCalls, _ := server.service.roleAssignmentMutationCalls()
			got := Expectation{CreateCalls: createCalls}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("Automation role assignment create calls mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAccAutomationRoleAssignmentResourceCreateRequiresImportForExistingAssignment(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()
	server.service.seedAssignment(&v1.RoleAssignment{
		Id:           accessControlAssignmentID,
		GroupId:      accessControlGroupID,
		ResourceId:   accessControlAutomationID,
		ResourceType: v1.ResourceType_RESOURCE_TYPE_WORKFLOW,
		ResourceRole: v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR,
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
				ExpectError: regexp.MustCompile(`already exists[\s\S]*ID "` + accessControlAssignmentID + `"[\s\S]*Import the existing[\s\S]*assignment`),
			},
		},
	})

	createCalls, _ := server.service.roleAssignmentMutationCalls()
	if diff := cmp.Diff(0, createCalls); diff != "" {
		t.Errorf("Automation role assignment create calls mismatch (-want +got):\n%s", diff)
	}
}

func TestAccAutomationRoleAssignmentResourceRejectsDuplicateMatches(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted() {
				return errors.New("Automation role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
			},
			{
				PreConfig: func() {
					server.service.seedAssignment(&v1.RoleAssignment{
						Id:           accessControlDuplicateID,
						GroupId:      accessControlGroupID,
						ResourceId:   accessControlAutomationID,
						ResourceType: v1.ResourceType_RESOURCE_TYPE_WORKFLOW,
						ResourceRole: v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR,
					})
					server.service.setRoleAssignmentListBehavior(1, false)
				},
				Config:      testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
				ExpectError: regexp.MustCompile(`multiple assignments match[\s\S]*` + accessControlAssignmentID + `[\s\S]*` + accessControlDuplicateID),
			},
			{
				PreConfig: func() {
					server.service.deleteAssignment(accessControlDuplicateID)
				},
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccAutomationRoleAssignmentResourceMalformedCreateRecoversState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name        string
		Configure   func(*fakeGroupService)
		ExpectError *regexp.Regexp
	}{
		{
			Name: "empty_assignment",
			Configure: func(service *fakeGroupService) {
				service.setNextRoleAssignmentCreateEmpty()
			},
			ExpectError: regexp.MustCompile(`returned an empty role assignment[\s\S]*recovered[\s\S]*ID "` + accessControlAssignmentID + `"`),
		},
		{
			Name: "assignment_without_id",
			Configure: func(service *fakeGroupService) {
				service.setNextRoleAssignmentCreateWithoutID()
			},
			ExpectError: regexp.MustCompile(`role assignment without an ID[\s\S]*recovered[\s\S]*ID "` + accessControlAssignmentID + `"`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newAccessControlAPIServer(t)
			t.Cleanup(server.Close)
			server.service.seedGroup()
			tc.Configure(server.service)

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy: func(state *terraform.State) error {
					if !server.service.assignmentDeleted() {
						return errors.New("Automation role assignment was not deleted")
					}
					return nil
				},
				Steps: []resource.TestStep{
					{
						Config:      testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
						ExpectError: tc.ExpectError,
					},
					{
						Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{
								plancheck.ExpectResourceAction("ona_automation_role_assignment.test", plancheck.ResourceActionReplace),
							},
						},
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("ona_automation_role_assignment.test", "id", accessControlAssignmentID),
							checkAutomationRoleAssignmentMutationCalls(server.service, 2, 1),
						),
					},
					{
						Config: testAccProviderConfig(server.URL),
					},
				},
			})

			createCalls, deleteCalls := server.service.roleAssignmentMutationCalls()
			expected := struct {
				CreateCalls int
				DeleteCalls int
			}{CreateCalls: 2, DeleteCalls: 2}
			got := struct {
				CreateCalls int
				DeleteCalls int
			}{CreateCalls: createCalls, DeleteCalls: deleteCalls}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("Automation role assignment recovery calls mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAccAutomationRoleAssignmentResourceMalformedCreatePreservesState(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()
	server.service.setNextRoleAssignmentCreateMismatched()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted() {
				return errors.New("Automation role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config:      testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
				ExpectError: regexp.MustCompile(`Unable to Create Ona Automation Role Assignment[\s\S]*does not match the requested`),
			},
			{
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "executor"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ona_automation_role_assignment.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_automation_role_assignment.test", "id", accessControlAssignmentID),
					checkAutomationRoleAssignmentCreateRequest(server.service, automationRoleAssignmentRequestExpectation{
						Calls:        2,
						AutomationID: accessControlAutomationID,
						GroupID:      accessControlGroupID,
						ResourceType: v1.ResourceType_RESOURCE_TYPE_WORKFLOW,
						Role:         v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR,
					}),
				),
			},
		},
	})
}

func TestAccAutomationRoleAssignmentResourceReadAndDeleteErrors(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted() {
				return errors.New("Automation role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "viewer"),
			},
			{
				PreConfig: func() {
					server.service.setNextRoleAssignmentListError(connect.NewError(connect.CodePermissionDenied, errors.New("read denied")))
				},
				Config:      testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "viewer"),
				ExpectError: regexp.MustCompile(`Unable to Read Ona Automation Role Assignment[\s\S]*does not have permission[\s\S]*read denied`),
			},
			{
				Config: testAccAutomationRoleAssignmentResourceConfig(server.URL, accessControlAutomationID, accessControlGroupID, "viewer"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				PreConfig: func() {
					server.service.setNextRoleAssignmentDeleteError(connect.NewError(connect.CodeUnavailable, errors.New("delete unavailable")))
				},
				Config:      testAccProviderConfig(server.URL),
				ExpectError: regexp.MustCompile(`Unable to Delete Ona Automation Role Assignment[\s\S]*temporarily unavailable[\s\S]*delete unavailable`),
			},
			{
				PreConfig: func() {
					server.service.setNextRoleAssignmentDeleteError(connect.NewError(connect.CodeNotFound, errors.New("assignment already deleted")))
				},
				Config: testAccProviderConfig(server.URL),
			},
		},
	})

	type Expectation struct {
		CreateCalls int
		DeleteCalls int
	}
	expected := Expectation{CreateCalls: 1, DeleteCalls: 2}
	createCalls, deleteCalls := server.service.roleAssignmentMutationCalls()
	got := Expectation{CreateCalls: createCalls, DeleteCalls: deleteCalls}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("Automation role assignment API error calls mismatch (-want +got):\n%s", diff)
	}
}

type automationRoleAssignmentRequestExpectation struct {
	Calls        int
	AutomationID string
	GroupID      string
	ResourceType v1.ResourceType
	Role         v1.ResourceRole
}

func checkAutomationRoleAssignmentCreateRequest(service *fakeGroupService, expected automationRoleAssignmentRequestExpectation) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		request, calls := service.latestRoleAssignmentCreateRequest()
		got := automationRoleAssignmentRequestExpectation{Calls: calls}
		if request != nil {
			got.AutomationID = request.GetResourceId()
			got.GroupID = request.GetGroupId()
			got.ResourceType = request.GetResourceType()
			got.Role = request.GetResourceRole()
		}
		if diff := cmp.Diff(expected, got); diff != "" {
			return fmt.Errorf("Automation role assignment create request mismatch (-want +got):\n%s", diff)
		}
		return nil
	}
}

func checkAutomationRoleAssignmentMutationCalls(service *fakeGroupService, expectedCreateCalls int, expectedDeleteCalls int) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		type Expectation struct {
			CreateCalls int
			DeleteCalls int
		}
		expected := Expectation{CreateCalls: expectedCreateCalls, DeleteCalls: expectedDeleteCalls}
		createCalls, deleteCalls := service.roleAssignmentMutationCalls()
		got := Expectation{CreateCalls: createCalls, DeleteCalls: deleteCalls}
		if diff := cmp.Diff(expected, got); diff != "" {
			return fmt.Errorf("Automation role assignment mutation calls mismatch (-want +got):\n%s", diff)
		}
		return nil
	}
}

func checkAutomationRoleAssignmentListRequests(service *fakeGroupService) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		type Expectation struct {
			SawContinuation bool
			InvalidRequests []int
		}
		expected := Expectation{SawContinuation: true}
		var got Expectation
		for index, request := range service.roleAssignmentListRequests() {
			filter := request.GetFilter()
			if request.GetPagination().GetToken() != "" {
				got.SawContinuation = true
			}
			if filter.GetGroupId() != accessControlGroupID ||
				filter.GetResourceId() != accessControlAutomationID ||
				!cmp.Equal(filter.GetResourceTypes(), []v1.ResourceType{v1.ResourceType_RESOURCE_TYPE_WORKFLOW}) ||
				!cmp.Equal(filter.GetResourceRoles(), []v1.ResourceRole{v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR}) {
				got.InvalidRequests = append(got.InvalidRequests, index)
			}
		}
		if diff := cmp.Diff(expected, got); diff != "" {
			return fmt.Errorf("Automation role assignment list requests mismatch (-want +got):\n%s", diff)
		}
		return nil
	}
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
	return testAccGroupMembershipResourceConfigWithMember(host, accessControlGroupID, "service_account_id", serviceAccountID)
}

func testAccUserGroupMembershipResourceConfig(host, groupID, userID string) string {
	return testAccGroupMembershipResourceConfigWithMember(host, groupID, "user_id", userID)
}

func testAccGroupMembershipResourceConfigWithMember(host, groupID, memberAttribute, memberID string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_group_membership" "test" {
  group_id           = %[2]q
  %[3]s = %[4]q
}
`, host, groupID, memberAttribute, memberID)
}

func testAccGroupMembershipResourceConfigWithoutMember(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_group_membership" "test" {
  group_id = %[2]q
}
`, host, accessControlGroupID)
}

func testAccGroupMembershipResourceConfigWithBothMembers(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_group_membership" "test" {
  group_id           = %[2]q
  service_account_id = %[3]q
  user_id            = %[4]q
}
`, host, accessControlGroupID, accessControlServiceAccountID, accessControlUserID)
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

func testAccAutomationRoleAssignmentResourceConfig(host string, automationID string, groupID string, role string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_automation_role_assignment" "test" {
  automation_id = %[2]q
  group_id      = %[3]q
  role          = %[4]q
}
`, host, automationID, groupID, role)
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
	teamListRequests    []*v1.ListTeamsRequest
	teamListPageLimit   int32
	nextTeamListError   error
	nextTeamDeleteError error
	nextTeamUpdateEmpty bool
	nextTeamUpdateError error
	memberships         map[string]*v1.GroupMembership
	deletedMemberships  map[string]bool
	membershipTest      struct {
		nextCreatePrincipal *v1.Principal
		nextCreateEmpty     bool
		createCalls         int
		deleteCalls         int
	}
	membershipListCalls []*v1.ListMembershipsRequest
	membershipPageLimit int32
	assignments         map[string]*v1.RoleAssignment
	deletedAssignments  map[string]bool
	roleAssignmentTest  struct {
		nextCreateError      error
		nextCreateEmpty      bool
		nextCreateWithoutID  bool
		nextCreateMismatched bool
		nextListError        error
		nextDeleteError      error
		createRequests       []*v1.CreateRoleAssignmentRequest
		listRequests         []*v1.ListRoleAssignmentsRequest
		deleteRequests       []*v1.DeleteRoleAssignmentRequest
		pageLimit            int32
		ignoreFilters        bool
		repeatNextToken      bool
	}
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

func (s *fakeGroupService) ListTeams(ctx context.Context, req *connect.Request[v1.ListTeamsRequest]) (*connect.Response[v1.ListTeamsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.teamListRequests = append(s.teamListRequests, proto.CloneOf(req.Msg))
	if s.nextTeamListError != nil {
		err := s.nextTeamListError
		s.nextTeamListError = nil
		return nil, err
	}

	var teams []*v1.Team
	for _, team := range s.teams {
		teams = append(teams, cloneTeam(team))
	}
	sort.Slice(teams, func(i, j int) bool { return teams[i].GetId() < teams[j].GetId() })

	start, _ := strconv.Atoi(req.Msg.GetPagination().GetToken())
	if start > len(teams) {
		start = len(teams)
	}
	pageSize := int(req.Msg.GetPagination().GetPageSize())
	if pageSize <= 0 {
		pageSize = len(teams)
	}
	if s.teamListPageLimit > 0 && int(s.teamListPageLimit) < pageSize {
		pageSize = int(s.teamListPageLimit)
	}
	end := start + pageSize
	if end > len(teams) {
		end = len(teams)
	}
	var nextToken string
	if end < len(teams) {
		nextToken = strconv.Itoa(end)
	}

	return connect.NewResponse(&v1.ListTeamsResponse{
		Teams:      teams[start:end],
		Pagination: &v1.PaginationResponse{NextToken: nextToken},
	}), nil
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
	s.membershipTest.createCalls++

	member := &v1.GroupMembership{
		Id:      accessControlMembershipID,
		GroupId: req.Msg.GetGroupId(),
		Subject: &v1.Subject{
			Id:        req.Msg.GetSubject().GetId(),
			Principal: req.Msg.GetSubject().GetPrincipal(),
		},
		Name: s.serviceAccountNames[req.Msg.GetSubject().GetId()],
	}
	s.memberships[memberKey(member.GetGroupId(), member.GetSubject().GetPrincipal(), member.GetSubject().GetId())] = member
	s.deletedMemberships[member.GetId()] = false
	if s.membershipTest.nextCreateEmpty {
		s.membershipTest.nextCreateEmpty = false
		return connect.NewResponse(&v1.CreateMembershipResponse{}), nil
	}
	responseMember := cloneMembership(member)
	if s.membershipTest.nextCreatePrincipal != nil {
		responseMember.Subject.Principal = *s.membershipTest.nextCreatePrincipal
		s.membershipTest.nextCreatePrincipal = nil
	}
	return connect.NewResponse(&v1.CreateMembershipResponse{Member: responseMember}), nil
}

func (s *fakeGroupService) GetMembership(ctx context.Context, req *connect.Request[v1.GetMembershipRequest]) (*connect.Response[v1.GetMembershipResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	member := s.memberships[memberKey(req.Msg.GetGroupId(), req.Msg.GetSubject().GetPrincipal(), req.Msg.GetSubject().GetId())]
	if member == nil {
		return connect.NewResponse(&v1.GetMembershipResponse{}), nil
	}
	return connect.NewResponse(&v1.GetMembershipResponse{Member: cloneMembership(member)}), nil
}

func (s *fakeGroupService) DeleteMembership(ctx context.Context, req *connect.Request[v1.DeleteMembershipRequest]) (*connect.Response[v1.DeleteMembershipResponse], error) {
	s.mu.Lock()
	s.membershipTest.deleteCalls++
	s.mu.Unlock()
	s.deleteMembership(req.Msg.GetMembershipId())
	return connect.NewResponse(&v1.DeleteMembershipResponse{}), nil
}

func (s *fakeGroupService) CreateRoleAssignment(ctx context.Context, req *connect.Request[v1.CreateRoleAssignmentRequest]) (*connect.Response[v1.CreateRoleAssignmentResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roleAssignmentTest.createRequests = append(s.roleAssignmentTest.createRequests, proto.CloneOf(req.Msg))
	if s.roleAssignmentTest.nextCreateError != nil {
		err := s.roleAssignmentTest.nextCreateError
		s.roleAssignmentTest.nextCreateError = nil
		return nil, err
	}

	assignment := &v1.RoleAssignment{
		Id:             accessControlAssignmentID,
		GroupId:        req.Msg.GetGroupId(),
		OrganizationId: req.Msg.GetResourceId(),
		ResourceId:     req.Msg.GetResourceId(),
		ResourceType:   req.Msg.GetResourceType(),
		ResourceRole:   req.Msg.GetResourceRole(),
	}
	s.assignments[assignment.GetId()] = assignment
	s.deletedAssignments[assignment.GetId()] = false
	if s.roleAssignmentTest.nextCreateEmpty {
		s.roleAssignmentTest.nextCreateEmpty = false
		return connect.NewResponse(&v1.CreateRoleAssignmentResponse{}), nil
	}
	responseAssignment := cloneAssignment(assignment)
	if s.roleAssignmentTest.nextCreateWithoutID {
		s.roleAssignmentTest.nextCreateWithoutID = false
		responseAssignment.Id = ""
	}
	if s.roleAssignmentTest.nextCreateMismatched {
		s.roleAssignmentTest.nextCreateMismatched = false
		if req.Msg.GetResourceType() == v1.ResourceType_RESOURCE_TYPE_RUNNER {
			responseAssignment.ResourceId = accessControlOtherRunnerID
		} else if req.Msg.GetResourceType() == v1.ResourceType_RESOURCE_TYPE_PROJECT {
			responseAssignment.ResourceId = accessControlOtherProjectID
		} else {
			responseAssignment.ResourceId = accessControlOtherAutomationID
		}
	}
	return connect.NewResponse(&v1.CreateRoleAssignmentResponse{Assignment: responseAssignment}), nil
}

func (s *fakeGroupService) ListRoleAssignments(ctx context.Context, req *connect.Request[v1.ListRoleAssignmentsRequest]) (*connect.Response[v1.ListRoleAssignmentsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roleAssignmentTest.listRequests = append(s.roleAssignmentTest.listRequests, proto.CloneOf(req.Msg))
	if s.roleAssignmentTest.nextListError != nil {
		err := s.roleAssignmentTest.nextListError
		s.roleAssignmentTest.nextListError = nil
		return nil, err
	}

	var assignments []*v1.RoleAssignment
	for _, assignment := range s.assignments {
		if !s.roleAssignmentTest.ignoreFilters && !matchesRoleAssignmentFilter(assignment, req.Msg.GetFilter()) {
			continue
		}
		assignments = append(assignments, cloneAssignment(assignment))
	}
	sort.Slice(assignments, func(i, j int) bool {
		return assignments[i].GetId() < assignments[j].GetId()
	})

	start, _ := strconv.Atoi(req.Msg.GetPagination().GetToken())
	if start > len(assignments) {
		start = len(assignments)
	}
	pageSize := int(req.Msg.GetPagination().GetPageSize())
	if pageSize <= 0 {
		pageSize = len(assignments)
	}
	if s.roleAssignmentTest.pageLimit > 0 && int(s.roleAssignmentTest.pageLimit) < pageSize {
		pageSize = int(s.roleAssignmentTest.pageLimit)
	}
	end := start + pageSize
	if end > len(assignments) {
		end = len(assignments)
	}
	var nextToken string
	if end < len(assignments) {
		nextToken = strconv.Itoa(end)
	}
	if s.roleAssignmentTest.repeatNextToken && req.Msg.GetPagination().GetToken() != "" && nextToken != "" {
		nextToken = req.Msg.GetPagination().GetToken()
	}
	return connect.NewResponse(&v1.ListRoleAssignmentsResponse{
		Assignments: assignments[start:end],
		Pagination:  &v1.PaginationResponse{NextToken: nextToken},
	}), nil
}

func (s *fakeGroupService) DeleteRoleAssignment(ctx context.Context, req *connect.Request[v1.DeleteRoleAssignmentRequest]) (*connect.Response[v1.DeleteRoleAssignmentResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roleAssignmentTest.deleteRequests = append(s.roleAssignmentTest.deleteRequests, proto.CloneOf(req.Msg))
	if s.roleAssignmentTest.nextDeleteError != nil {
		err := s.roleAssignmentTest.nextDeleteError
		s.roleAssignmentTest.nextDeleteError = nil
		if connect.CodeOf(err) == connect.CodeNotFound {
			s.deleteAssignmentLocked(req.Msg.GetAssignmentId())
		}
		return nil, err
	}
	s.deleteAssignmentLocked(req.Msg.GetAssignmentId())
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

func (s *fakeGroupService) setNextMembershipCreateResponsePrincipal(principal v1.Principal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.membershipTest.nextCreatePrincipal = &principal
}

func (s *fakeGroupService) setNextMembershipCreateResponseEmpty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.membershipTest.nextCreateEmpty = true
}

func (s *fakeGroupService) membershipMutationCalls() (createCalls, deleteCalls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.membershipTest.createCalls, s.membershipTest.deleteCalls
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

func (s *fakeGroupService) setMembershipPrincipal(id string, principal v1.Principal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, member := range s.memberships {
		if member.GetId() == id {
			member.Subject.Principal = principal
		}
	}
}

func (s *fakeGroupService) membershipDeleted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletedMemberships[accessControlMembershipID]
}

func (s *fakeGroupService) deleteAssignment(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteAssignmentLocked(id)
}

func (s *fakeGroupService) deleteAssignmentLocked(id string) {
	delete(s.assignments, id)
	s.deletedAssignments[id] = true
}

func (s *fakeGroupService) seedAssignment(assignment *v1.RoleAssignment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assignments[assignment.GetId()] = cloneAssignment(assignment)
	s.deletedAssignments[assignment.GetId()] = false
}

func (s *fakeGroupService) replaceAssignmentRole(id string, role v1.ResourceRole) {
	s.mu.Lock()
	defer s.mu.Unlock()
	assignment := s.assignments[id]
	if assignment != nil {
		assignment.ResourceRole = role
	}
}

func (s *fakeGroupService) replaceAssignment(id, replacementID string, role v1.ResourceRole) {
	s.mu.Lock()
	defer s.mu.Unlock()
	assignment := s.assignments[id]
	if assignment == nil {
		return
	}
	delete(s.assignments, id)
	s.deletedAssignments[id] = true
	replacement := cloneAssignment(assignment)
	replacement.Id = replacementID
	replacement.ResourceRole = role
	s.assignments[replacementID] = replacement
	s.deletedAssignments[replacementID] = false
}

func (s *fakeGroupService) setRoleAssignmentListBehavior(pageLimit int32, ignoreFilters bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roleAssignmentTest.pageLimit = pageLimit
	s.roleAssignmentTest.ignoreFilters = ignoreFilters
}

func (s *fakeGroupService) setRoleAssignmentRepeatedToken(repeat bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roleAssignmentTest.repeatNextToken = repeat
}

func (s *fakeGroupService) setNextRoleAssignmentCreateError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roleAssignmentTest.nextCreateError = err
}

func (s *fakeGroupService) setNextRoleAssignmentCreateEmpty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roleAssignmentTest.nextCreateEmpty = true
}

func (s *fakeGroupService) setNextRoleAssignmentCreateWithoutID() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roleAssignmentTest.nextCreateWithoutID = true
}

func (s *fakeGroupService) setNextRoleAssignmentCreateMismatched() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roleAssignmentTest.nextCreateMismatched = true
}

func (s *fakeGroupService) setNextRoleAssignmentListError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roleAssignmentTest.nextListError = err
}

func (s *fakeGroupService) setNextRoleAssignmentDeleteError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roleAssignmentTest.nextDeleteError = err
}

func (s *fakeGroupService) latestRoleAssignmentCreateRequest() (*v1.CreateRoleAssignmentRequest, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	requests := s.roleAssignmentTest.createRequests
	if len(requests) == 0 {
		return nil, 0
	}
	return proto.CloneOf(requests[len(requests)-1]), len(requests)
}

func (s *fakeGroupService) roleAssignmentListRequests() []*v1.ListRoleAssignmentsRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	requests := make([]*v1.ListRoleAssignmentsRequest, 0, len(s.roleAssignmentTest.listRequests))
	for _, request := range s.roleAssignmentTest.listRequests {
		requests = append(requests, proto.CloneOf(request))
	}
	return requests
}

func (s *fakeGroupService) roleAssignmentMutationCalls() (createCalls, deleteCalls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.roleAssignmentTest.createRequests), len(s.roleAssignmentTest.deleteRequests)
}

func (s *fakeGroupService) assignmentDeleted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletedAssignments[accessControlAssignmentID]
}

func (s *fakeGroupService) roleAssignmentDeleted(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletedAssignments[id]
}

func memberKey(groupID string, principal v1.Principal, subjectID string) string {
	return fmt.Sprintf("%s/%d/%s", groupID, principal, subjectID)
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
