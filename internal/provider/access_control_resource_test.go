// Copyright IBM Corp. 2021, 2026
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
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
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
						plancheck.ExpectKnownValue("ona_group_membership.test", tfjsonpath.New("principal"), knownvalue.StringExact("service_account")),
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
		"principal":          "service_account",
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
		memberships:         map[string]*v1.GroupMembership{},
		deletedMemberships:  map[string]bool{},
		assignments:         map[string]*v1.RoleAssignment{},
		deletedAssignments:  map[string]bool{},
		serviceAccountNames: map[string]string{accessControlServiceAccountID: "Terraform Service Account", accessControlOtherServiceID: "Other Service Account"},
	}
	mux := http.NewServeMux()
	groupPath, groupHandler := v1connect.NewGroupServiceHandler(service)
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(service)
	mux.Handle(groupPath, groupHandler)
	mux.Handle(identityPath, identityHandler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	return &accessControlAPIServer{
		Server:  server,
		service: service,
	}
}

type fakeGroupService struct {
	v1connect.UnimplementedGroupServiceHandler
	v1connect.UnimplementedIdentityServiceHandler

	mu                  sync.Mutex
	groups              map[string]*v1.Group
	deletedGroups       map[string]bool
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
