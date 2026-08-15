// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestAccTeamMembershipQuery(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)
	server.service.seedMembership(&v1.TeamMember{Id: teamMembershipID, TeamId: teamMembershipTeamID, UserId: teamMembershipUserID, Name: "Ona User"})
	server.service.seedMembership(&v1.TeamMember{Id: teamMembershipReplacementID, TeamId: teamMembershipTeamID, UserId: teamMembershipOtherUserID, Name: "Another User"})

	firstIdentity := teamMembershipQueryIdentity(teamMembershipUserID)
	secondIdentity := teamMembershipQueryIdentity(teamMembershipOtherUserID)
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: teamMembershipQueryConfig(true, 0),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_team_membership.all", 2),
			querycheck.ExpectIdentity("ona_team_membership.all", firstIdentity),
			querycheck.ExpectIdentity("ona_team_membership.all", secondIdentity),
			querycheck.ExpectResourceKnownValues("ona_team_membership.all", queryfilter.ByResourceIdentity(firstIdentity), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(teamMembershipID)},
				{Path: tfjsonpath.New("team_id"), KnownValue: knownvalue.StringExact(teamMembershipTeamID)},
				{Path: tfjsonpath.New("user_id"), KnownValue: knownvalue.StringExact(teamMembershipUserID)},
			}),
			querycheck.ExpectResourceKnownValues("ona_team_membership.all", queryfilter.ByResourceIdentity(secondIdentity), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(teamMembershipReplacementID)},
				{Path: tfjsonpath.New("team_id"), KnownValue: knownvalue.StringExact(teamMembershipTeamID)},
				{Path: tfjsonpath.New("user_id"), KnownValue: knownvalue.StringExact(teamMembershipOtherUserID)},
			}),
		},
	}))
}

func TestAccTeamMembershipQueryReturnsIdentityWithoutResource(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)
	server.service.seedMembership(&v1.TeamMember{Id: teamMembershipID, TeamId: teamMembershipTeamID, UserId: teamMembershipUserID})

	identity := teamMembershipQueryIdentity(teamMembershipUserID)
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: teamMembershipQueryConfig(false, 0),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_team_membership.all", 1),
			querycheck.ExpectIdentity("ona_team_membership.all", identity),
			querycheck.ExpectResourceDisplayName("ona_team_membership.all", queryfilter.ByResourceIdentity(identity), knownvalue.StringExact("user_33333333_3333_4333_8333_333333333333")),
			expectTeamMembershipResourceOmitted{},
		},
	}))
}

func TestAccTeamMembershipQueryPaginatesToLimitWithStableNames(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)
	server.service.setSingleMemberPages()
	server.service.seedMembership(&v1.TeamMember{Id: "00000000-0000-4000-8000-000000000001", TeamId: teamMembershipTeamID, UserId: teamMembershipOtherUserID, Name: "Shared Name"})
	server.service.seedMembership(&v1.TeamMember{Id: "00000000-0000-4000-8000-000000000002", TeamId: teamMembershipTeamID, UserId: teamMembershipUserID, Name: "Shared Name"})
	server.service.seedMembership(&v1.TeamMember{Id: "00000000-0000-4000-8000-000000000003", TeamId: teamMembershipTeamID, UserId: "99999999-9999-4999-8999-999999999999", Name: "Not Returned"})

	firstIdentity := teamMembershipQueryIdentity(teamMembershipUserID)
	secondIdentity := teamMembershipQueryIdentity(teamMembershipOtherUserID)
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: teamMembershipQueryConfig(false, 2),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_team_membership.all", 2),
			querycheck.ExpectIdentity("ona_team_membership.all", firstIdentity),
			querycheck.ExpectIdentity("ona_team_membership.all", secondIdentity),
			querycheck.ExpectResourceDisplayName("ona_team_membership.all", queryfilter.ByResourceIdentity(firstIdentity), knownvalue.StringExact("shared_name")),
			querycheck.ExpectResourceDisplayName("ona_team_membership.all", queryfilter.ByResourceIdentity(secondIdentity), knownvalue.StringExact("shared_name_2")),
		},
	}))

	expected := []*v1.ListTeamMembersRequest{
		{TeamId: teamMembershipTeamID, Pagination: &v1.PaginationRequest{PageSize: 2}},
		{TeamId: teamMembershipTeamID, Pagination: &v1.PaginationRequest{PageSize: 1, Token: "1"}},
	}
	if diff := cmp.Diff(expected, server.service.listRequestsSnapshot(), protocmp.Transform()); diff != "" {
		t.Errorf("team membership list requests mismatch (-want +got):\n%s", diff)
	}
}

func TestAccTeamMembershipQueryReportsMalformedMembership(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)
	server.service.setNextListMalformed()

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      teamMembershipQueryConfig(false, 0),
		ExpectError: regexp.MustCompile(`Unable to Map Ona Team Membership[\s\S]*without an ID`),
	}))
}

func TestAccTeamMembershipQueryReportsListError(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)
	server.service.failListOnCall(1, connect.NewError(connect.CodeUnavailable, errors.New("team service unavailable")))

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      teamMembershipQueryConfig(false, 0),
		ExpectError: regexp.MustCompile(`Unable to List Ona Team Memberships[\s\S]*team service unavailable`),
	}))
}

func TestAccTeamMembershipQueryRejectsRepeatedPageToken(t *testing.T) {
	t.Parallel()

	server := newTeamMembershipAPIServer(t)
	server.service.setSingleMemberPages()
	server.service.setRepeatPageToken(true)
	server.service.seedMembership(&v1.TeamMember{Id: teamMembershipID, TeamId: teamMembershipTeamID, UserId: teamMembershipUserID})
	server.service.seedMembership(&v1.TeamMember{Id: teamMembershipReplacementID, TeamId: teamMembershipTeamID, UserId: teamMembershipOtherUserID})

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      teamMembershipQueryConfig(false, 0),
		ExpectError: regexp.MustCompile(`Unable to List Ona Team Memberships[\s\S]*repeated pagination token "1"`),
	}))
}

type expectTeamMembershipResourceOmitted struct{}

func (expectTeamMembershipResourceOmitted) CheckQuery(_ context.Context, req querycheck.CheckQueryRequest, resp *querycheck.CheckQueryResponse) {
	for _, result := range req.Query {
		if strings.TrimPrefix(result.Address, "list.") != "ona_team_membership.all" {
			continue
		}
		if result.ResourceObject != nil {
			resp.Error = errors.New("ona_team_membership.all returned a resource object without include_resource")
		}
		return
	}
	resp.Error = errors.New("ona_team_membership.all returned no results")
}

func teamMembershipQueryIdentity(userID string) map[string]knownvalue.Check {
	return map[string]knownvalue.Check{
		"team_id": knownvalue.StringExact(teamMembershipTeamID),
		"user_id": knownvalue.StringExact(userID),
	}
}

func teamMembershipQueryConfig(includeResource bool, limit int) string {
	includeResourceLine := ""
	if includeResource {
		includeResourceLine = "  include_resource = true\n"
	}
	limitLine := ""
	if limit > 0 {
		limitLine = fmt.Sprintf("  limit            = %d\n", limit)
	}
	return `
list "ona_team_membership" "all" {
  provider = ona
` + includeResourceLine + limitLine + `  config {
    team_id = "11111111-1111-4111-8111-111111111111"
  }
}
`
}
