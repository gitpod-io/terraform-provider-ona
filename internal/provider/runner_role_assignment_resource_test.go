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
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccRunnerRoleAssignmentResourceLifecycle(t *testing.T) {
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
				return errors.New("runner role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner_role_assignment.test", "id", accessControlAssignmentID),
					resource.TestCheckResourceAttr("ona_runner_role_assignment.test", "runner_id", accessControlRunnerID),
					resource.TestCheckResourceAttr("ona_runner_role_assignment.test", "group_id", accessControlGroupID),
					resource.TestCheckResourceAttr("ona_runner_role_assignment.test", "role", "admin"),
					checkRunnerRoleAssignmentCreateRequest(server.service, runnerRoleAssignmentRequestExpectation{
						Calls:        1,
						RunnerID:     accessControlRunnerID,
						GroupID:      accessControlGroupID,
						ResourceType: v1.ResourceType_RESOURCE_TYPE_RUNNER,
						Role:         v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN,
					}),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentity("ona_runner_role_assignment.test", map[string]knownvalue.Check{
						"runner_id": knownvalue.StringExact(accessControlRunnerID),
						"group_id":  knownvalue.StringExact(accessControlGroupID),
					}),
				},
			},
			{
				PreConfig: func() {
					server.service.seedAssignment(&v1.RoleAssignment{
						Id:           "00000000-0000-4000-8000-000000000000",
						GroupId:      accessControlOtherGroupID,
						ResourceId:   accessControlRunnerID,
						ResourceType: v1.ResourceType_RESOURCE_TYPE_RUNNER,
						ResourceRole: v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN,
					})
					server.service.seedAssignment(&v1.RoleAssignment{
						Id:           "11111111-0000-4000-8000-000000000000",
						GroupId:      accessControlGroupID,
						ResourceId:   accessControlRunnerID,
						ResourceType: v1.ResourceType_RESOURCE_TYPE_WORKFLOW,
						ResourceRole: v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN,
					})
					server.service.seedAssignment(&v1.RoleAssignment{
						Id:           "22222222-0000-4000-8000-000000000000",
						GroupId:      accessControlGroupID,
						ResourceId:   accessControlAutomationID,
						ResourceType: v1.ResourceType_RESOURCE_TYPE_RUNNER,
						ResourceRole: v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN,
					})
					server.service.setRoleAssignmentListBehavior(1, true)
				},
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
				Check: checkRunnerRoleAssignmentListRequests(server.service),
			},
			{
				ResourceName:      "ona_runner_role_assignment.test",
				ImportState:       true,
				ImportStateId:     accessControlRunnerID + "/" + accessControlGroupID,
				ImportStateVerify: true,
			},
			{
				ResourceName:    "ona_runner_role_assignment.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportPlanChecks: resource.ImportPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectKnownValue("ona_runner_role_assignment.test", tfjsonpath.New("id"), knownvalue.StringExact(accessControlAssignmentID)),
					plancheck.ExpectKnownValue("ona_runner_role_assignment.test", tfjsonpath.New("runner_id"), knownvalue.StringExact(accessControlRunnerID)),
					plancheck.ExpectKnownValue("ona_runner_role_assignment.test", tfjsonpath.New("group_id"), knownvalue.StringExact(accessControlGroupID)),
					plancheck.ExpectKnownValue("ona_runner_role_assignment.test", tfjsonpath.New("role"), knownvalue.StringExact("admin")),
				}},
			},
			{
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "user"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_runner_role_assignment.test", plancheck.ResourceActionReplace),
				}},
			},
			{
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlOtherRunnerID, accessControlGroupID, "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_runner_role_assignment.test", plancheck.ResourceActionReplace),
				}},
			},
			{
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlOtherRunnerID, accessControlOtherGroupID, "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_runner_role_assignment.test", plancheck.ResourceActionReplace),
				}},
			},
		},
	})
}

func TestAccRunnerRoleAssignmentResourceDetectsDeletionAndRoleReplacement(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted() || !server.service.roleAssignmentDeleted(accessControlReplacementID) {
				return errors.New("runner role assignments were not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
			},
			{
				PreConfig: func() {
					server.service.deleteAssignment(accessControlAssignmentID)
				},
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_runner_role_assignment.test", plancheck.ResourceActionCreate),
				}},
			},
			{
				PreConfig: func() {
					server.service.replaceAssignment(accessControlAssignmentID, accessControlReplacementID, v1.ResourceRole_RESOURCE_ROLE_RUNNER_USER)
				},
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_runner_role_assignment.test", plancheck.ResourceActionReplace),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner_role_assignment.test", "id", accessControlAssignmentID),
					resource.TestCheckResourceAttr("ona_runner_role_assignment.test", "role", "admin"),
				),
			},
		},
	})

	type Expectation struct {
		CreateCalls int
		DeleteCalls int
	}
	expected := Expectation{CreateCalls: 3, DeleteCalls: 2}
	createCalls, deleteCalls := server.service.roleAssignmentMutationCalls()
	got := Expectation{CreateCalls: createCalls, DeleteCalls: deleteCalls}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("runner role assignment drift calls mismatch (-want +got):\n%s", diff)
	}
}

func TestAccRunnerRoleAssignmentResourceCreateErrors(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		CreateCalls int
	}
	tests := []struct {
		Name        string
		APIError    error
		ExpectError *regexp.Regexp
	}{
		{
			Name:        "enterprise_tier",
			APIError:    connect.NewError(connect.CodeFailedPrecondition, errors.New("sharing Runners with custom groups requires the Enterprise plan")),
			ExpectError: regexp.MustCompile(`Unable to Create Ona Runner Role Assignment[\s\S]*required state[\s\S]*requires[\s\S]*Enterprise plan`),
		},
		{
			Name:        "grant_permission",
			APIError:    connect.NewError(connect.CodePermissionDenied, errors.New("runner grant denied")),
			ExpectError: regexp.MustCompile(`Unable to Create Ona Runner Role Assignment[\s\S]*does not have permission[\s\S]*runner grant denied`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newAccessControlAPIServer(t)
			t.Cleanup(server.Close)
			server.service.setNextRoleAssignmentCreateError(tc.APIError)

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
					ExpectError: tc.ExpectError,
				}},
			})

			expected := Expectation{CreateCalls: 1}
			createCalls, _ := server.service.roleAssignmentMutationCalls()
			got := Expectation{CreateCalls: createCalls}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("runner role assignment create calls mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAccRunnerRoleAssignmentResourceRejectsExistingAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name        string
		Assignment  func() *v1.RoleAssignment
		ExpectError *regexp.Regexp
	}{
		{
			Name: "direct_assignment_requires_import",
			Assignment: func() *v1.RoleAssignment {
				return runnerRoleAssignment(accessControlAssignmentID, accessControlRunnerID, accessControlGroupID, v1.ResourceRole_RESOURCE_ROLE_RUNNER_USER)
			},
			ExpectError: regexp.MustCompile(`already exists[\s\S]*ID[\s\S]*` + accessControlAssignmentID + `[\s\S]*Import it with[\s\S]*` + accessControlRunnerID + `/` + accessControlGroupID),
		},
		{
			Name: "derived_assignment_cannot_be_owned",
			Assignment: func() *v1.RoleAssignment {
				assignment := runnerRoleAssignment(accessControlAssignmentID, accessControlRunnerID, accessControlGroupID, v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN)
				derived := v1.ResourceRole_RESOURCE_ROLE_ORG_RUNNERS_ADMIN
				assignment.DerivedFromOrgRole = &derived
				return assignment
			},
			ExpectError: regexp.MustCompile(`derived from an organization role[\s\S]*RESOURCE_ROLE_ORG_RUNNERS_ADMIN[\s\S]*Manage the organization role`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newAccessControlAPIServer(t)
			t.Cleanup(server.Close)
			server.service.seedAssignment(tc.Assignment())

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
					ExpectError: tc.ExpectError,
				}},
			})

			createCalls, _ := server.service.roleAssignmentMutationCalls()
			if diff := cmp.Diff(0, createCalls); diff != "" {
				t.Errorf("runner role assignment create calls mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAccRunnerRoleAssignmentResourceRejectsDuplicateDirectAssignments(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted() {
				return errors.New("runner role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
			},
			{
				PreConfig: func() {
					server.service.seedAssignment(runnerRoleAssignment(accessControlDuplicateID, accessControlRunnerID, accessControlGroupID, v1.ResourceRole_RESOURCE_ROLE_RUNNER_USER))
					server.service.setRoleAssignmentListBehavior(1, false)
				},
				Config:      testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
				ExpectError: regexp.MustCompile(`Multiple direct runner role assignments[\s\S]*` + accessControlAssignmentID + `[\s\S]*` + accessControlDuplicateID),
			},
			{
				PreConfig: func() {
					server.service.deleteAssignment(accessControlDuplicateID)
				},
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
			},
		},
	})
}

func TestAccRunnerRoleAssignmentResourceRejectsRepeatedPaginationToken(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted() {
				return errors.New("runner role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
			},
			{
				PreConfig: func() {
					server.service.seedAssignment(runnerRoleAssignment("00000000-0000-4000-8000-000000000000", accessControlAutomationID, accessControlOtherGroupID, v1.ResourceRole_RESOURCE_ROLE_RUNNER_USER))
					server.service.seedAssignment(runnerRoleAssignment("11111111-0000-4000-8000-000000000000", accessControlOtherRunnerID, accessControlOtherGroupID, v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN))
					server.service.setRoleAssignmentListBehavior(1, true)
					server.service.setRoleAssignmentRepeatedToken(true)
				},
				Config:      testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
				ExpectError: regexp.MustCompile(`repeated pagination token`),
			},
			{
				PreConfig: func() {
					server.service.setRoleAssignmentRepeatedToken(false)
				},
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
			},
		},
	})
}

func TestAccRunnerRoleAssignmentResourceMalformedCreateRecoversState(t *testing.T) {
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
			ExpectError: regexp.MustCompile(`returned an empty role assignment[\s\S]*recovered[\s\S]*assignment[\s\S]*ID[\s\S]*` + accessControlAssignmentID),
		},
		{
			Name: "assignment_without_id",
			Configure: func(service *fakeGroupService) {
				service.setNextRoleAssignmentCreateWithoutID()
			},
			ExpectError: regexp.MustCompile(`role assignment without an ID[\s\S]*recovered[\s\S]*assignment[\s\S]*ID[\s\S]*` + accessControlAssignmentID),
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newAccessControlAPIServer(t)
			t.Cleanup(server.Close)
			tc.Configure(server.service)

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				CheckDestroy: func(state *terraform.State) error {
					if !server.service.assignmentDeleted() {
						return errors.New("runner role assignment was not deleted")
					}
					return nil
				},
				Steps: []resource.TestStep{
					{
						Config:      testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
						ExpectError: tc.ExpectError,
					},
					{
						Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
						ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("ona_runner_role_assignment.test", plancheck.ResourceActionReplace),
						}},
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("ona_runner_role_assignment.test", "id", accessControlAssignmentID),
							checkRunnerRoleAssignmentMutationCalls(server.service, 2, 1),
						),
					},
					{Config: testAccProviderConfig(server.URL)},
				},
			})

			type Expectation struct {
				CreateCalls int
				DeleteCalls int
			}
			expected := Expectation{CreateCalls: 2, DeleteCalls: 2}
			createCalls, deleteCalls := server.service.roleAssignmentMutationCalls()
			got := Expectation{CreateCalls: createCalls, DeleteCalls: deleteCalls}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("runner role assignment recovery calls mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAccRunnerRoleAssignmentResourceMalformedCreatePreservesState(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.setNextRoleAssignmentCreateMismatched()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted() {
				return errors.New("runner role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config:      testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
				ExpectError: regexp.MustCompile(`Unable to Create Ona Runner Role Assignment[\s\S]*does not match the requested`),
			},
			{
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "admin"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_runner_role_assignment.test", plancheck.ResourceActionReplace),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner_role_assignment.test", "id", accessControlAssignmentID),
					checkRunnerRoleAssignmentMutationCalls(server.service, 2, 1),
				),
			},
		},
	})
}

func TestAccRunnerRoleAssignmentResourceReadAndDeleteErrors(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted() {
				return errors.New("runner role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "user"),
			},
			{
				PreConfig: func() {
					server.service.setNextRoleAssignmentListError(connect.NewError(connect.CodePermissionDenied, errors.New("read denied")))
				},
				Config:      testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "user"),
				ExpectError: regexp.MustCompile(`Unable to Read Ona Runner Role Assignment[\s\S]*does not have permission[\s\S]*read denied`),
			},
			{
				Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlGroupID, "user"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
			},
			{
				PreConfig: func() {
					server.service.setNextRoleAssignmentDeleteError(connect.NewError(connect.CodeUnavailable, errors.New("delete unavailable")))
				},
				Config:      testAccProviderConfig(server.URL),
				ExpectError: regexp.MustCompile(`Unable to Delete Ona Runner Role Assignment[\s\S]*temporarily unavailable[\s\S]*delete unavailable`),
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
		t.Errorf("runner role assignment API error calls mismatch (-want +got):\n%s", diff)
	}
}

func TestAccRunnerRoleAssignmentResourceSupportsGeneralAccessAndOrganizationAdmin(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedAssignment(&v1.RoleAssignment{
		Id:           accessControlDuplicateID,
		GroupId:      accessControlOrgMembersGroupID,
		ResourceId:   accessControlOrgID,
		ResourceType: v1.ResourceType_RESOURCE_TYPE_ORGANIZATION,
		ResourceRole: v1.ResourceRole_RESOURCE_ROLE_ORG_RUNNERS_ADMIN,
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if !server.service.assignmentDeleted() {
				return errors.New("runner role assignment was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: testAccRunnerRoleAssignmentResourceConfig(server.URL, accessControlRunnerID, accessControlOrgMembersGroupID, "user"),
			Check: checkRunnerRoleAssignmentCreateRequest(server.service, runnerRoleAssignmentRequestExpectation{
				Calls:        1,
				RunnerID:     accessControlRunnerID,
				GroupID:      accessControlOrgMembersGroupID,
				ResourceType: v1.ResourceType_RESOURCE_TYPE_RUNNER,
				Role:         v1.ResourceRole_RESOURCE_ROLE_RUNNER_USER,
			}),
		}},
	})
}

type runnerRoleAssignmentRequestExpectation struct {
	Calls        int
	RunnerID     string
	GroupID      string
	ResourceType v1.ResourceType
	Role         v1.ResourceRole
}

func checkRunnerRoleAssignmentCreateRequest(service *fakeGroupService, expected runnerRoleAssignmentRequestExpectation) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		request, calls := service.latestRoleAssignmentCreateRequest()
		got := runnerRoleAssignmentRequestExpectation{Calls: calls}
		if request != nil {
			got.RunnerID = request.GetResourceId()
			got.GroupID = request.GetGroupId()
			got.ResourceType = request.GetResourceType()
			got.Role = request.GetResourceRole()
		}
		if diff := cmp.Diff(expected, got); diff != "" {
			return fmt.Errorf("runner role assignment create request mismatch (-want +got):\n%s", diff)
		}
		return nil
	}
}

func checkRunnerRoleAssignmentMutationCalls(service *fakeGroupService, expectedCreateCalls, expectedDeleteCalls int) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		type Expectation struct {
			CreateCalls int
			DeleteCalls int
		}
		expected := Expectation{CreateCalls: expectedCreateCalls, DeleteCalls: expectedDeleteCalls}
		createCalls, deleteCalls := service.roleAssignmentMutationCalls()
		got := Expectation{CreateCalls: createCalls, DeleteCalls: deleteCalls}
		if diff := cmp.Diff(expected, got); diff != "" {
			return fmt.Errorf("runner role assignment mutation calls mismatch (-want +got):\n%s", diff)
		}
		return nil
	}
}

func checkRunnerRoleAssignmentListRequests(service *fakeGroupService) resource.TestCheckFunc {
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
				filter.GetResourceId() != accessControlRunnerID ||
				!cmp.Equal(filter.GetResourceTypes(), []v1.ResourceType{v1.ResourceType_RESOURCE_TYPE_RUNNER}) ||
				len(filter.GetResourceRoles()) != 0 {
				got.InvalidRequests = append(got.InvalidRequests, index)
			}
		}
		if diff := cmp.Diff(expected, got); diff != "" {
			return fmt.Errorf("runner role assignment list requests mismatch (-want +got):\n%s", diff)
		}
		return nil
	}
}

func testAccRunnerRoleAssignmentResourceConfig(host, runnerID, groupID, role string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner_role_assignment" "test" {
  runner_id = %[2]q
  group_id   = %[3]q
  role       = %[4]q
}
`, host, runnerID, groupID, role)
}

func runnerRoleAssignment(id, runnerID, groupID string, role v1.ResourceRole) *v1.RoleAssignment {
	return &v1.RoleAssignment{
		Id:           id,
		GroupId:      groupID,
		ResourceId:   runnerID,
		ResourceType: v1.ResourceType_RESOURCE_TYPE_RUNNER,
		ResourceRole: role,
	}
}
