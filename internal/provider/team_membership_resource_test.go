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
	"sync"
	"testing"

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
)

const (
	teamMembershipTeamID           = "11111111-1111-4111-8111-111111111111"
	teamMembershipOtherTeamID      = "22222222-2222-4222-8222-222222222222"
	teamMembershipUserID           = "33333333-3333-4333-8333-333333333333"
	teamMembershipOtherUserID      = "44444444-4444-4444-8444-444444444444"
	teamMembershipID               = "55555555-5555-4555-8555-555555555555"
	teamMembershipReplacementID    = "66666666-6666-4666-8666-666666666666"
	teamMembershipSecondReplaceID  = "77777777-7777-4777-8777-777777777777"
	teamMembershipDriftID          = "88888888-8888-4888-8888-888888888888"
	teamMembershipUnrelatedID      = "00000000-0000-4000-8000-000000000001"
	teamMembershipOtherUnrelatedID = "00000000-0000-4000-8000-000000000002"
)

func TestAccTeamMembershipResourceLifecycle(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(state *terraform.State) error {
			if server.service.membershipCount() != 0 {
				return errors.New("team membership was not deleted")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_team_membership.test", "id", teamMembershipID),
					resource.TestCheckResourceAttr("ona_team_membership.test", "team_id", teamMembershipTeamID),
					resource.TestCheckResourceAttr("ona_team_membership.test", "user_id", teamMembershipUserID),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentity("ona_team_membership.test", map[string]knownvalue.Check{
						"team_id": knownvalue.StringExact(teamMembershipTeamID),
						"user_id": knownvalue.StringExact(teamMembershipUserID),
					}),
				},
			},
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
			},
			{
				ResourceName:      "ona_team_membership.test",
				ImportState:       true,
				ImportStateId:     teamMembershipTeamID + "/" + teamMembershipUserID,
				ImportStateVerify: true,
			},
			{
				ResourceName:    "ona_team_membership.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportPlanChecks: resource.ImportPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_team_membership.test", plancheck.ResourceActionNoop),
					plancheck.ExpectKnownValue("ona_team_membership.test", tfjsonpath.New("id"), knownvalue.StringExact(teamMembershipID)),
				}},
			},
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipOtherUserID),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_team_membership.test", plancheck.ResourceActionReplace),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_team_membership.test", "id", teamMembershipReplacementID),
					resource.TestCheckResourceAttr("ona_team_membership.test", "user_id", teamMembershipOtherUserID),
				),
			},
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipOtherTeamID, teamMembershipOtherUserID),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("ona_team_membership.test", plancheck.ResourceActionReplace),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_team_membership.test", "id", teamMembershipSecondReplaceID),
					resource.TestCheckResourceAttr("ona_team_membership.test", "team_id", teamMembershipOtherTeamID),
					func(state *terraform.State) error {
						want := []string{
							"add:" + teamMembershipTeamID + ":" + teamMembershipUserID,
							"remove:" + teamMembershipID,
							"add:" + teamMembershipTeamID + ":" + teamMembershipOtherUserID,
							"remove:" + teamMembershipReplacementID,
							"add:" + teamMembershipOtherTeamID + ":" + teamMembershipOtherUserID,
						}
						if diff := cmp.Diff(want, server.service.mutationLog()); diff != "" {
							return fmt.Errorf("unexpected team membership mutations (-want +got):\n%s", diff)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestAccTeamMembershipResourceReadPaginates(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
			},
			{
				PreConfig: func() {
					server.service.seedMembership(&v1.TeamMember{Id: teamMembershipUnrelatedID, TeamId: teamMembershipTeamID, UserId: teamMembershipOtherUserID})
					server.service.setSingleMemberPages()
				},
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
				Check: func(state *terraform.State) error {
					if calls := server.service.listCallCount(); calls < 2 {
						return fmt.Errorf("expected paginated ListTeamMembers calls, got %d", calls)
					}
					return nil
				},
			},
		},
	})
}

func TestAccTeamMembershipResourceImportPaginates(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
			},
			{
				PreConfig: func() {
					server.service.seedMembership(&v1.TeamMember{Id: teamMembershipUnrelatedID, TeamId: teamMembershipTeamID, UserId: teamMembershipOtherUserID})
					server.service.setSingleMemberPages()
				},
				Config:            testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ResourceName:      "ona_team_membership.test",
				ImportState:       true,
				ImportStateId:     teamMembershipTeamID + "/" + teamMembershipUserID,
				ImportStateVerify: true,
				Check: func(state *terraform.State) error {
					if calls := server.service.listCallCount(); calls < 2 {
						return fmt.Errorf("expected paginated ListTeamMembers calls, got %d", calls)
					}
					return nil
				},
			},
		},
	})
}

func TestAccTeamMembershipResourceListErrorPreservesState(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
			},
			{
				PreConfig: func() {
					server.service.seedMembership(&v1.TeamMember{Id: teamMembershipUnrelatedID, TeamId: teamMembershipTeamID, UserId: teamMembershipOtherUserID})
					server.service.setSingleMemberPages()
					server.service.failListOnCall(2, errors.New("second page unavailable"))
				},
				Config:      testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ExpectError: regexp.MustCompile(`Unable to Read Ona Team Membership[\s\S]*second page unavailable`),
			},
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
			},
		},
	})
}

func TestAccTeamMembershipResourceMalformedListPreservesState(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
			},
			{
				PreConfig:   server.service.setNextListMalformed,
				Config:      testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ExpectError: regexp.MustCompile(`Unable to Read Ona Team Membership[\s\S]*without an ID`),
			},
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
			},
		},
	})
}

func TestAccTeamMembershipResourceRepeatedPageTokenPreservesState(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
			},
			{
				PreConfig: func() {
					server.service.deleteMembership(teamMembershipID)
					server.service.seedMembership(&v1.TeamMember{Id: teamMembershipUnrelatedID, TeamId: teamMembershipTeamID, UserId: teamMembershipOtherUserID})
					server.service.seedMembership(&v1.TeamMember{Id: teamMembershipOtherUnrelatedID, TeamId: teamMembershipTeamID, UserId: teamMembershipOtherUserID})
					server.service.setSingleMemberPages()
					server.service.setRepeatPageToken(true)
				},
				Config:      testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ExpectError: regexp.MustCompile(`Unable to Read Ona Team Membership[\s\S]*repeated pagination token`),
			},
			{
				PreConfig: func() {
					server.service.setRepeatPageToken(false)
					server.service.deleteMembership(teamMembershipUnrelatedID)
					server.service.deleteMembership(teamMembershipOtherUnrelatedID)
					server.service.seedMembership(&v1.TeamMember{Id: teamMembershipID, TeamId: teamMembershipTeamID, UserId: teamMembershipUserID})
				},
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectEmptyPlan(),
				}},
			},
		},
	})
}

func TestAccTeamMembershipResourceDoesNotAdoptReplacementID(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
			},
			{
				PreConfig: func() {
					server.service.deleteMembership(teamMembershipID)
					server.service.seedMembership(&v1.TeamMember{Id: teamMembershipDriftID, TeamId: teamMembershipTeamID, UserId: teamMembershipUserID})
				},
				Config:             testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccTeamMembershipResourceAmbiguousImport(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)
	server.service.seedMembership(&v1.TeamMember{Id: teamMembershipID, TeamId: teamMembershipTeamID, UserId: teamMembershipUserID})
	server.service.seedMembership(&v1.TeamMember{Id: teamMembershipReplacementID, TeamId: teamMembershipTeamID, UserId: teamMembershipUserID})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:            testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ResourceName:      "ona_team_membership.test",
				ImportState:       true,
				ImportStateId:     teamMembershipTeamID + "/" + teamMembershipUserID,
				ImportStateVerify: false,
				ExpectError:       regexp.MustCompile(`returned 2 memberships[\s\S]*expected exactly one`),
			},
		},
	})
}

func TestAccTeamMembershipResourceRejectsInvalidImportID(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:        testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ResourceName:  "ona_team_membership.test",
				ImportState:   true,
				ImportStateId: teamMembershipTeamID,
				ExpectError:   regexp.MustCompile(`Invalid Import ID[\s\S]*team_id/user_id`),
			},
		},
	})
}

func TestAccTeamMembershipResourceRecoversPartialCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name        string
		Configure   func(*fakeTeamMembershipService)
		ExpectedErr string
	}{
		{Name: "empty_response", Configure: func(service *fakeTeamMembershipService) { service.setNextAddEmpty() }, ExpectedErr: `empty team membership`},
		{Name: "missing_id", Configure: func(service *fakeTeamMembershipService) { service.setNextAddWithoutID() }, ExpectedErr: `without an ID`},
		{Name: "wrong_user", Configure: func(service *fakeTeamMembershipService) { service.setNextAddWrongUser() }, ExpectedErr: `instead of`},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newTeamMembershipAPIServer(t)
			tc.Configure(server.service)

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
						ExpectError: regexp.MustCompile(`Unable to Create Ona Team Membership[\s\S]*` + tc.ExpectedErr),
					},
					{
						Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
						ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("ona_team_membership.test", plancheck.ResourceActionReplace),
						}},
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("ona_team_membership.test", "id", teamMembershipReplacementID),
							func(state *terraform.State) error {
								adds, removes := server.service.mutationCounts()
								if adds != 2 || removes != 1 {
									return fmt.Errorf("expected two adds and one recovery removal, got %d adds and %d removals", adds, removes)
								}
								return nil
							},
						),
					},
				},
			})
		})
	}
}

func TestAccTeamMembershipResourceAPIErrors(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		server := newTeamMembershipAPIServer(t)
		server.service.setNextAddError(errors.New("add denied"))
		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreCheck(t) },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{{
				Config:      testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID),
				ExpectError: regexp.MustCompile(`Unable to Create Ona Team Membership[\s\S]*add denied`),
			}},
		})
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		server := newTeamMembershipAPIServer(t)
		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreCheck(t) },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID)},
				{
					PreConfig:   func() { server.service.setNextRemoveError(errors.New("remove denied")) },
					Config:      testAccProviderConfig(server.URL),
					ExpectError: regexp.MustCompile(`Unable to Delete Ona Team Membership[\s\S]*remove denied`),
				},
				{Config: testAccProviderConfig(server.URL)},
			},
		})
	})

	t.Run("delete_not_found", func(t *testing.T) {
		t.Parallel()

		server := newTeamMembershipAPIServer(t)
		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { testAccPreCheck(t) },
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{Config: testAccTeamMembershipResourceConfig(server.URL, teamMembershipTeamID, teamMembershipUserID)},
				{
					PreConfig: func() {
						server.service.setNextRemoveError(connect.NewError(connect.CodeNotFound, errors.New("membership not found")))
					},
					Config: testAccProviderConfig(server.URL),
				},
			},
		})
	})
}

func testAccTeamMembershipResourceConfig(host, teamID, userID string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_team_membership" "test" {
  team_id = %[2]q
  user_id = %[3]q
}
`, host, teamID, userID)
}

type teamMembershipAPIServer struct {
	*httptest.Server
	service *fakeTeamMembershipService
}

func newTeamMembershipAPIServer(t *testing.T) *teamMembershipAPIServer {
	t.Helper()

	service := &fakeTeamMembershipService{memberships: make(map[string]*v1.TeamMember)}
	mux := http.NewServeMux()
	path, handler := v1connect.NewTeamServiceHandler(service)
	mux.Handle(path, handler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	t.Cleanup(server.Close)
	return &teamMembershipAPIServer{Server: server, service: service}
}

type fakeTeamMembershipService struct {
	v1connect.UnimplementedTeamServiceHandler

	mu                sync.Mutex
	memberships       map[string]*v1.TeamMember
	nextMembership    int
	mutations         []string
	pageLimit         int
	listCalls         int
	listRequests      []*v1.ListTeamMembersRequest
	failListCall      int
	nextListError     error
	nextListMalformed bool
	repeatPageToken   bool
	nextAddError      error
	nextAddEmpty      bool
	nextAddWithoutID  bool
	nextAddWrongUser  bool
	nextRemoveError   error
}

func (s *fakeTeamMembershipService) AddTeamMember(ctx context.Context, req *connect.Request[v1.AddTeamMemberRequest]) (*connect.Response[v1.AddTeamMemberResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.nextAddError != nil {
		err := s.nextAddError
		s.nextAddError = nil
		return nil, err
	}
	for _, member := range s.memberships {
		if member.GetUserId() == req.Msg.GetUserId() {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("user already belongs to a team"))
		}
	}

	ids := []string{teamMembershipID, teamMembershipReplacementID, teamMembershipSecondReplaceID, teamMembershipDriftID}
	id := ids[s.nextMembership]
	s.nextMembership++
	member := &v1.TeamMember{Id: id, TeamId: req.Msg.GetTeamId(), UserId: req.Msg.GetUserId(), Name: "Terraform User"}
	s.memberships[id] = member
	s.mutations = append(s.mutations, "add:"+member.GetTeamId()+":"+member.GetUserId())

	if s.nextAddEmpty {
		s.nextAddEmpty = false
		return connect.NewResponse(&v1.AddTeamMemberResponse{}), nil
	}
	responseMember := proto.CloneOf(member)
	if s.nextAddWithoutID {
		s.nextAddWithoutID = false
		responseMember.Id = ""
	}
	if s.nextAddWrongUser {
		s.nextAddWrongUser = false
		responseMember.UserId = teamMembershipOtherUserID
	}
	return connect.NewResponse(&v1.AddTeamMemberResponse{Member: responseMember}), nil
}

func (s *fakeTeamMembershipService) ListTeamMembers(ctx context.Context, req *connect.Request[v1.ListTeamMembersRequest]) (*connect.Response[v1.ListTeamMembersResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listCalls++
	s.listRequests = append(s.listRequests, proto.CloneOf(req.Msg))
	if s.failListCall > 0 && s.listCalls == s.failListCall {
		err := s.nextListError
		s.failListCall = 0
		s.nextListError = nil
		return nil, err
	}

	var members []*v1.TeamMember
	for _, member := range s.memberships {
		if member.GetTeamId() == req.Msg.GetTeamId() {
			members = append(members, proto.CloneOf(member))
		}
	}
	if s.nextListMalformed {
		s.nextListMalformed = false
		members = append(members, &v1.TeamMember{TeamId: req.Msg.GetTeamId(), UserId: teamMembershipOtherUserID})
	}
	sort.Slice(members, func(i, j int) bool { return members[i].GetId() < members[j].GetId() })

	start, _ := strconv.Atoi(req.Msg.GetPagination().GetToken())
	if start > len(members) {
		start = len(members)
	}
	pageSize := int(req.Msg.GetPagination().GetPageSize())
	if pageSize <= 0 || pageSize > len(members) {
		pageSize = len(members)
	}
	if s.pageLimit > 0 && pageSize > s.pageLimit {
		pageSize = s.pageLimit
	}
	end := start + pageSize
	if end > len(members) {
		end = len(members)
	}
	var nextToken string
	if end < len(members) {
		nextToken = strconv.Itoa(end)
	}
	if s.repeatPageToken && req.Msg.GetPagination().GetToken() != "" {
		nextToken = req.Msg.GetPagination().GetToken()
	}
	return connect.NewResponse(&v1.ListTeamMembersResponse{
		Members:    members[start:end],
		Pagination: &v1.PaginationResponse{NextToken: nextToken},
	}), nil
}

func (s *fakeTeamMembershipService) RemoveTeamMember(ctx context.Context, req *connect.Request[v1.RemoveTeamMemberRequest]) (*connect.Response[v1.RemoveTeamMemberResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mutations = append(s.mutations, "remove:"+req.Msg.GetTeamMemberId())
	if s.nextRemoveError != nil {
		err := s.nextRemoveError
		s.nextRemoveError = nil
		if connect.CodeOf(err) == connect.CodeNotFound {
			delete(s.memberships, req.Msg.GetTeamMemberId())
		}
		return nil, err
	}
	if _, ok := s.memberships[req.Msg.GetTeamMemberId()]; !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("membership not found"))
	}
	delete(s.memberships, req.Msg.GetTeamMemberId())
	return connect.NewResponse(&v1.RemoveTeamMemberResponse{}), nil
}

func (s *fakeTeamMembershipService) seedMembership(member *v1.TeamMember) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memberships[member.GetId()] = proto.CloneOf(member)
}

func (s *fakeTeamMembershipService) deleteMembership(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.memberships, id)
}

func (s *fakeTeamMembershipService) setSingleMemberPages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pageLimit = 1
	s.listCalls = 0
}

func (s *fakeTeamMembershipService) failListOnCall(call int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listCalls = 0
	s.failListCall = call
	s.nextListError = err
}

func (s *fakeTeamMembershipService) setNextListMalformed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextListMalformed = true
}

func (s *fakeTeamMembershipService) setRepeatPageToken(repeat bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.repeatPageToken = repeat
}

func (s *fakeTeamMembershipService) setNextAddError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAddError = err
}

func (s *fakeTeamMembershipService) setNextAddEmpty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAddEmpty = true
}

func (s *fakeTeamMembershipService) setNextAddWithoutID() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAddWithoutID = true
}

func (s *fakeTeamMembershipService) setNextAddWrongUser() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAddWrongUser = true
}

func (s *fakeTeamMembershipService) setNextRemoveError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextRemoveError = err
}

func (s *fakeTeamMembershipService) membershipCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.memberships)
}

func (s *fakeTeamMembershipService) listCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listCalls
}

func (s *fakeTeamMembershipService) listRequestsSnapshot() []*v1.ListTeamMembersRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*v1.ListTeamMembersRequest, 0, len(s.listRequests))
	for _, request := range s.listRequests {
		result = append(result, proto.CloneOf(request))
	}
	return result
}

func (s *fakeTeamMembershipService) mutationLog() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.mutations...)
}

func (s *fakeTeamMembershipService) mutationCounts() (adds, removes int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, mutation := range s.mutations {
		if len(mutation) >= 4 && mutation[:4] == "add:" {
			adds++
		}
		if len(mutation) >= 7 && mutation[:7] == "remove:" {
			removes++
		}
	}
	return adds, removes
}
