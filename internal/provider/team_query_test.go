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
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestAccTeamQuery(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.teams[accessControlTeamID] = &v1.Team{
		Id:             accessControlTeamID,
		OrganizationId: accessControlOrgID,
		Name:           "Platform Engineering",
		CreatedAt:      timestampForTest(accessControlCreatedAt),
	}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: teamQueryConfig(true, 0),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_team.all", 1),
			querycheck.ExpectIdentity("ona_team.all", map[string]knownvalue.Check{"id": knownvalue.StringExact(accessControlTeamID)}),
			querycheck.ExpectResourceKnownValues("ona_team.all", queryfilter.ByDisplayName(knownvalue.StringExact("platform_engineering")), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(accessControlTeamID)},
				{Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact("Platform Engineering")},
				{Path: tfjsonpath.New("created_at"), KnownValue: knownvalue.StringExact(accessControlCreatedAt)},
			}),
		},
	}))
}

func TestAccTeamQueryPaginatesToLimit(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.teamListPageLimit = 1
	server.service.teams["11111111-1111-4111-8111-111111111111"] = &v1.Team{Id: "11111111-1111-4111-8111-111111111111", Name: "Alpha"}
	server.service.teams["22222222-2222-4222-8222-222222222222"] = &v1.Team{Id: "22222222-2222-4222-8222-222222222222", Name: "Beta"}
	server.service.teams["33333333-3333-4333-8333-333333333333"] = &v1.Team{Id: "33333333-3333-4333-8333-333333333333", Name: "Gamma"}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: teamQueryConfig(false, 2),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_team.all", 2),
			querycheck.ExpectIdentity("ona_team.all", map[string]knownvalue.Check{"id": knownvalue.StringExact("11111111-1111-4111-8111-111111111111")}),
			querycheck.ExpectIdentity("ona_team.all", map[string]knownvalue.Check{"id": knownvalue.StringExact("22222222-2222-4222-8222-222222222222")}),
		},
	}))

	expected := []*v1.ListTeamsRequest{
		{Pagination: &v1.PaginationRequest{PageSize: 2}},
		{Pagination: &v1.PaginationRequest{PageSize: 1, Token: "1"}},
	}
	if diff := cmp.Diff(expected, server.service.teamListRequestsSnapshot(), protocmp.Transform()); diff != "" {
		t.Errorf("team list requests mismatch (-want +got):\n%s", diff)
	}
}

func TestAccTeamQueryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name        string
		Configure   func(*fakeGroupService)
		ExpectError *regexp.Regexp
	}{
		{
			Name: "list_api_error",
			Configure: func(service *fakeGroupService) {
				service.nextTeamListError = connect.NewError(connect.CodeUnavailable, errors.New("team service unavailable"))
			},
			ExpectError: regexp.MustCompile(`Unable to List Ona Teams[\s\S]*team service unavailable`),
		},
		{
			Name: "missing_team_id",
			Configure: func(service *fakeGroupService) {
				service.teams["invalid"] = &v1.Team{Name: "Invalid Team"}
			},
			ExpectError: regexp.MustCompile(`Unable to List Ona Teams[\s\S]*team without an ID`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newAccessControlAPIServer(t)
			t.Cleanup(server.Close)
			tc.Configure(server.service)
			testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
				Query:       true,
				Config:      teamQueryConfig(false, 0),
				ExpectError: tc.ExpectError,
			}))
		})
	}
}

func (s *fakeGroupService) teamListRequestsSnapshot() []*v1.ListTeamsRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*v1.ListTeamsRequest, 0, len(s.teamListRequests))
	for _, request := range s.teamListRequests {
		result = append(result, proto.CloneOf(request))
	}
	return result
}

func teamQueryConfig(includeResource bool, limit int) string {
	config := `
list "ona_team" "all" {
  provider = ona
`
	if includeResource {
		config += "  include_resource = true\n"
	}
	if limit > 0 {
		config += fmt.Sprintf("  limit = %d\n", limit)
	}
	return config + "}\n"
}
