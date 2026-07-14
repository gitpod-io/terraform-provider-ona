// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func (s *fakeGroupService) ListGroups(ctx context.Context, req *connect.Request[v1.ListGroupsRequest]) (*connect.Response[v1.ListGroupsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var groups []*v1.Group
	for _, group := range s.groups {
		if len(req.Msg.GetFilter().GetGroupIds()) > 0 && group.GetId() != req.Msg.GetFilter().GetGroupIds()[0] {
			continue
		}
		if group.GetSystemManaged() || group.GetDirectShare() {
			continue
		}
		groups = append(groups, cloneGroup(group))
	}
	return connect.NewResponse(&v1.ListGroupsResponse{Groups: groups}), nil
}

func TestAccGroupQuery(t *testing.T) {
	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedGroup()
	server.service.groups["system-group"] = &v1.Group{Id: "system-group", OrganizationId: accessControlOrgID, Name: "Everyone", SystemManaged: true}
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: groupQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_group.all", 1),
		querycheck.ExpectIdentity("ona_group.all", map[string]knownvalue.Check{"id": knownvalue.StringExact(accessControlGroupID)}),
		querycheck.ExpectResourceKnownValues("ona_group.all", queryfilter.ByDisplayName(knownvalue.StringExact("Terraform Admins")), []querycheck.KnownValueCheck{
			{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(accessControlGroupID)},
			{Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact("Terraform Admins")},
		}),
	}}))
}

func groupQueryConfig() string {
	return `
list "ona_group" "all" {
  provider = ona
  include_resource = true
  config { group_ids = ["22222222-2222-4222-8222-222222222222"] }
}
`
}
