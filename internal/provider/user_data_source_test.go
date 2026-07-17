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
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	userDataSourceOrganizationID = "00000000-0000-0000-0000-000000000100"
	userDataSourceAliceID        = "00000000-0000-0000-0000-000000000001"
	userDataSourceBobID          = "00000000-0000-0000-0000-000000000002"
	userDataSourceCarolID        = "00000000-0000-0000-0000-000000000003"
)

func TestAccUserDataSource(t *testing.T) {
	t.Parallel()

	server := newUserDataSourceAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserDataSourceConfig(server.URL, userDataSourceAliceID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.ona_user.test", "id", userDataSourceAliceID),
					resource.TestCheckResourceAttr("data.ona_user.test", "user_id", userDataSourceAliceID),
					resource.TestCheckResourceAttr("data.ona_user.test", "name", "Alice Admin"),
					resource.TestCheckResourceAttr("data.ona_user.test", "email", "alice@example.com"),
					resource.TestCheckResourceAttr("data.ona_user.test", "status", "active"),
					resource.TestCheckResourceAttr("data.ona_user.test", "role", "admin"),
					resource.TestCheckResourceAttr("data.ona_user.test", "member_since", "2026-07-17T10:00:00Z"),
					resource.TestCheckResourceAttr("data.ona_user.test", "login_provider", "github"),
					resource.TestCheckNoResourceAttr("data.ona_user.test", "organization_id"),
					resource.TestCheckNoResourceAttr("data.ona_user.test", "avatar_url"),
					resource.TestCheckNoResourceAttr("data.ona_user.test", "created_at"),
					resource.TestCheckResourceAttr("echo.consumer", "data", userDataSourceAliceID),
					checkSingularUserRequests(server, userDataSourceAliceID),
				),
			},
		},
	})
}

func TestAccUsersDataSource(t *testing.T) {
	t.Parallel()

	server := newUserDataSourceAPIServer(t)
	server.service.pageSize = 1
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUsersDataSourceConfig(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.ona_users.test", "users.#", "2"),
					resource.TestCheckResourceAttr("data.ona_users.test", "users.0.user_id", userDataSourceAliceID),
					resource.TestCheckResourceAttr("data.ona_users.test", "users.0.name", "Alice Admin"),
					resource.TestCheckResourceAttr("data.ona_users.test", "users.0.role", "admin"),
					resource.TestCheckResourceAttr("data.ona_users.test", "users.1.user_id", userDataSourceBobID),
					resource.TestCheckResourceAttr("data.ona_users.test", "users.1.name", "Bob Member"),
					resource.TestCheckResourceAttr("data.ona_users.test", "users.1.status", "suspended"),
					resource.TestCheckNoResourceAttr("data.ona_users.test", "id"),
					resource.TestCheckNoResourceAttr("data.ona_users.test", "organization_id"),
					resource.TestCheckNoResourceAttr("data.ona_users.test", "users.0.avatar_url"),
					resource.TestCheckTypeSetElemAttr("data.ona_users.test", "statuses.*", "active"),
					resource.TestCheckTypeSetElemAttr("data.ona_users.test", "statuses.*", "suspended"),
					resource.TestCheckTypeSetElemAttr("data.ona_users.test", "roles.*", "admin"),
					resource.TestCheckTypeSetElemAttr("data.ona_users.test", "roles.*", "member"),
					checkCollectionUserRequests(server),
				),
			},
			{
				Config: testAccUsersEmptyDataSourceConfig(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.ona_users.test", "users.#", "0"),
					resource.TestCheckNoResourceAttr("data.ona_users.test", "id"),
				),
			},
		},
	})
}

func TestAccUserDataSourceDiagnostics(t *testing.T) {
	t.Parallel()

	server := newUserDataSourceAPIServer(t)
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserDataSourceConfig(server.URL, "not-a-uuid"),
				ExpectError: regexp.MustCompile("Invalid UUID"),
			},
			{
				Config:      testAccUserDataSourceConfig(server.URL, "00000000-0000-0000-0000-000000000099"),
				ExpectError: regexp.MustCompile("Ona User Not Found or Not Visible"),
			},
			{
				Config:      testAccUsersInvalidRoleConfig(server.URL),
				ExpectError: regexp.MustCompile("Invalid Organization Role"),
			},
			{
				PreConfig: func() {
					server.service.setMembers(nil)
				},
				Config:      testAccUserDataSourceConfig(server.URL, userDataSourceAliceID),
				ExpectError: regexp.MustCompile("Ona User Not Found or Not Visible"),
			},
			{
				PreConfig: func() {
					member := testUserDataSourceMember(userDataSourceAliceID, "Alice Admin", "alice@example.com", v1.UserStatus_USER_STATUS_ACTIVE, v1.OrganizationRole_ORGANIZATION_ROLE_ADMIN, "github")
					server.service.setMembers([]*v1.OrganizationMember{member, proto.CloneOf(member)})
				},
				Config:      testAccUserDataSourceConfig(server.URL, userDataSourceAliceID),
				ExpectError: regexp.MustCompile("returned 2 memberships"),
			},
			{
				PreConfig: func() {
					server.service.setMembers([]*v1.OrganizationMember{testUserDataSourceMember(userDataSourceAliceID, "Alice Admin", "alice@example.com", v1.UserStatus_USER_STATUS_ACTIVE, v1.OrganizationRole_ORGANIZATION_ROLE_ADMIN, "github")})
					server.service.setUserOrganization(userDataSourceAliceID, "00000000-0000-0000-0000-000000000999")
				},
				Config:      testAccUserDataSourceConfig(server.URL, userDataSourceAliceID),
				ExpectError: regexp.MustCompile("Ona User Organization Mismatch"),
			},
		},
	})
}

func testAccUserDataSourceConfig(host string, userID string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

data "ona_user" "test" {
  user_id = %[2]q
}

provider "echo" {
  data = data.ona_user.test.user_id
}

resource "echo" "consumer" {}
`, host, userID)
}

func testAccUsersDataSourceConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

data "ona_users" "test" {
  search   = "example.com"
  statuses = ["suspended", "active"]
  roles    = ["member", "admin"]
  user_ids = [%[2]q, %[3]q]
}
`, host, userDataSourceBobID, userDataSourceAliceID)
}

func testAccUsersInvalidRoleConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

data "ona_users" "test" {
  roles = ["owner"]
}
`, host)
}

func testAccUsersEmptyDataSourceConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

data "ona_users" "test" {
  user_ids = ["00000000-0000-0000-0000-000000000099"]
}
`, host)
}

type userDataSourceAPIServer struct {
	*httptest.Server
	service *fakeUserDataSourceService
}

func newUserDataSourceAPIServer(t *testing.T) *userDataSourceAPIServer {
	t.Helper()

	joined := timestamppb.New(time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC))
	members := []*v1.OrganizationMember{
		testUserDataSourceMember(userDataSourceCarolID, "Carol Former", "carol@example.net", v1.UserStatus_USER_STATUS_LEFT, v1.OrganizationRole_ORGANIZATION_ROLE_MEMBER, "google"),
		testUserDataSourceMember(userDataSourceBobID, "Bob Member", "bob@example.com", v1.UserStatus_USER_STATUS_SUSPENDED, v1.OrganizationRole_ORGANIZATION_ROLE_MEMBER, "google"),
		testUserDataSourceMember(userDataSourceAliceID, "Alice Admin", "alice@example.com", v1.UserStatus_USER_STATUS_ACTIVE, v1.OrganizationRole_ORGANIZATION_ROLE_ADMIN, "github"),
	}
	for _, member := range members {
		member.MemberSince = joined
	}
	service := &fakeUserDataSourceService{
		organizationID: userDataSourceOrganizationID,
		members:        members,
		users: map[string]*v1.User{
			userDataSourceAliceID: {Id: userDataSourceAliceID, OrganizationId: userDataSourceOrganizationID, Name: "Alice Admin", Email: "alice@example.com", Status: v1.UserStatus_USER_STATUS_ACTIVE},
			userDataSourceBobID:   {Id: userDataSourceBobID, OrganizationId: userDataSourceOrganizationID, Name: "Bob Member", Email: "bob@example.com", Status: v1.UserStatus_USER_STATUS_SUSPENDED},
			userDataSourceCarolID: {Id: userDataSourceCarolID, OrganizationId: userDataSourceOrganizationID, Name: "Carol Former", Email: "carol@example.net", Status: v1.UserStatus_USER_STATUS_LEFT},
		},
	}

	mux := http.NewServeMux()
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(service)
	organizationPath, organizationHandler := v1connect.NewOrganizationServiceHandler(service)
	userPath, userHandler := v1connect.NewUserServiceHandler(service)
	mux.Handle(identityPath, identityHandler)
	mux.Handle(organizationPath, organizationHandler)
	mux.Handle(userPath, userHandler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	return &userDataSourceAPIServer{Server: server, service: service}
}

func testUserDataSourceMember(id string, name string, email string, status v1.UserStatus, role v1.OrganizationRole, loginProvider string) *v1.OrganizationMember {
	return &v1.OrganizationMember{UserId: id, FullName: name, Email: email, Status: status, Role: role, LoginProvider: loginProvider}
}

type fakeUserDataSourceService struct {
	v1connect.UnimplementedIdentityServiceHandler
	v1connect.UnimplementedOrganizationServiceHandler
	v1connect.UnimplementedUserServiceHandler

	mu                   sync.Mutex
	organizationID       string
	users                map[string]*v1.User
	members              []*v1.OrganizationMember
	pageSize             int
	getUserCalls         int
	listMemberRequests   []*v1.ListMembersRequest
	identityRequestCount int
}

func (s *fakeUserDataSourceService) GetAuthenticatedIdentity(_ context.Context, req *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if got := req.Header().Get("Authorization"); got != "Bearer test-token" {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authorization header = %q, want Bearer test-token", got))
	}
	s.identityRequestCount++
	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{OrganizationId: s.organizationID}), nil
}

func (s *fakeUserDataSourceService) GetUser(_ context.Context, req *connect.Request[v1.GetUserRequest]) (*connect.Response[v1.GetUserResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getUserCalls++
	user := s.users[req.Msg.GetUserId()]
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}
	return connect.NewResponse(&v1.GetUserResponse{User: proto.CloneOf(user)}), nil
}

func (s *fakeUserDataSourceService) ListMembers(_ context.Context, req *connect.Request[v1.ListMembersRequest]) (*connect.Response[v1.ListMembersResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Msg.GetOrganizationId() != s.organizationID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("organization not found"))
	}
	s.listMemberRequests = append(s.listMemberRequests, proto.CloneOf(req.Msg))

	members := make([]*v1.OrganizationMember, 0, len(s.members))
	for _, member := range s.members {
		if memberMatchesFilter(member, req.Msg.GetFilter()) {
			members = append(members, proto.CloneOf(member))
		}
	}
	sort.SliceStable(members, func(i, j int) bool {
		if members[i].GetFullName() == members[j].GetFullName() {
			return members[i].GetUserId() < members[j].GetUserId()
		}
		return members[i].GetFullName() < members[j].GetFullName()
	})

	start := 0
	if req.Msg.GetPagination().GetToken() != "" {
		value, err := strconv.Atoi(req.Msg.GetPagination().GetToken())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pagination token"))
		}
		start = value
	}
	pageSize := s.pageSize
	if pageSize <= 0 {
		pageSize = int(req.Msg.GetPagination().GetPageSize())
	}
	if pageSize <= 0 {
		pageSize = len(members)
	}
	end := min(start+pageSize, len(members))
	nextToken := ""
	if end < len(members) {
		nextToken = strconv.Itoa(end)
	}
	return connect.NewResponse(&v1.ListMembersResponse{
		Members:    members[start:end],
		Pagination: &v1.PaginationResponse{NextToken: nextToken},
	}), nil
}

func (s *fakeUserDataSourceService) setMembers(members []*v1.OrganizationMember) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = members
}

func (s *fakeUserDataSourceService) setUserOrganization(userID string, organizationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := proto.CloneOf(s.users[userID])
	user.OrganizationId = organizationID
	s.users[userID] = user
}

func memberMatchesFilter(member *v1.OrganizationMember, filter *v1.ListMembersRequest_Filter) bool {
	if filter == nil {
		return true
	}
	search := strings.ToLower(strings.TrimSpace(filter.GetSearch()))
	if search != "" && search != strings.ToLower(member.GetUserId()) &&
		!strings.Contains(strings.ToLower(member.GetFullName()), search) &&
		!strings.Contains(strings.ToLower(member.GetEmail()), search) {
		return false
	}
	if len(filter.GetStatuses()) > 0 && !containsUserStatus(filter.GetStatuses(), member.GetStatus()) {
		return false
	}
	if len(filter.GetRoles()) > 0 && !containsOrganizationRole(filter.GetRoles(), member.GetRole()) {
		return false
	}
	if len(filter.GetUserIds()) > 0 && !containsString(filter.GetUserIds(), member.GetUserId()) {
		return false
	}
	return true
}

func containsUserStatus(values []v1.UserStatus, value v1.UserStatus) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func containsOrganizationRole(values []v1.OrganizationRole, value v1.OrganizationRole) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func checkSingularUserRequests(server *userDataSourceAPIServer, userID string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		server.service.mu.Lock()
		defer server.service.mu.Unlock()
		if server.service.getUserCalls == 0 {
			return errors.New("GetUser was not called")
		}
		if server.service.identityRequestCount == 0 {
			return errors.New("GetAuthenticatedIdentity was not called")
		}
		if len(server.service.listMemberRequests) == 0 {
			return errors.New("ListMembers was not called")
		}
		request := server.service.listMemberRequests[len(server.service.listMemberRequests)-1]
		if request.GetOrganizationId() != userDataSourceOrganizationID || len(request.GetFilter().GetUserIds()) != 1 || request.GetFilter().GetUserIds()[0] != userID {
			return fmt.Errorf("unexpected singular ListMembers request: %v", request)
		}
		return nil
	}
}

func checkCollectionUserRequests(server *userDataSourceAPIServer) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		server.service.mu.Lock()
		defer server.service.mu.Unlock()
		if server.service.getUserCalls != 0 {
			return fmt.Errorf("GetUser calls = %d, want 0", server.service.getUserCalls)
		}
		if len(server.service.listMemberRequests) < 2 {
			return fmt.Errorf("ListMembers calls = %d, want at least 2 to prove pagination", len(server.service.listMemberRequests))
		}
		for _, request := range server.service.listMemberRequests {
			if request.GetOrganizationId() != userDataSourceOrganizationID {
				return fmt.Errorf("organization_id = %q, want %q", request.GetOrganizationId(), userDataSourceOrganizationID)
			}
			if request.GetPagination().GetPageSize() != 100 {
				return fmt.Errorf("page_size = %d, want 100", request.GetPagination().GetPageSize())
			}
			if request.GetSort().GetField() != v1.ListMembersRequest_SORT_FIELD_NAME || request.GetSort().GetOrder() != v1.SortOrder_SORT_ORDER_ASC {
				return fmt.Errorf("unexpected sort: %v", request.GetSort())
			}
		}
		return nil
	}
}
