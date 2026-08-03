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

func (s *fakeGroupService) ListMemberships(ctx context.Context, req *connect.Request[v1.ListMembershipsRequest]) (*connect.Response[v1.ListMembershipsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var members []*v1.GroupMembership
	for _, member := range s.memberships {
		if member.GetGroupId() == req.Msg.GetGroupId() {
			members = append(members, cloneMembership(member))
		}
	}
	return connect.NewResponse(&v1.ListMembershipsResponse{Members: members}), nil
}

func TestAccGroupMembershipQuery(t *testing.T) {
	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.memberships[memberKey(accessControlGroupID, accessControlServiceAccountID)] = &v1.GroupMembership{Id: accessControlMembershipID, GroupId: accessControlGroupID, Subject: &v1.Subject{Id: accessControlServiceAccountID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}, Name: "Terraform Service Account"}
	server.service.memberships[memberKey(accessControlGroupID, "user-1")] = &v1.GroupMembership{Id: "user-membership", GroupId: accessControlGroupID, Subject: &v1.Subject{Id: "user-1", Principal: v1.Principal_PRINCIPAL_USER}, Name: "User"}
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: groupMembershipQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_group_membership.all", 1),
		querycheck.ExpectIdentity("ona_group_membership.all", map[string]knownvalue.Check{"group_id": knownvalue.StringExact(accessControlGroupID), "service_account_id": knownvalue.StringExact(accessControlServiceAccountID)}),
		querycheck.ExpectResourceKnownValues("ona_group_membership.all", queryfilter.ByDisplayName(knownvalue.StringExact("Terraform Service Account")), []querycheck.KnownValueCheck{
			{Path: tfjsonpath.New("group_id"), KnownValue: knownvalue.StringExact(accessControlGroupID)},
			{Path: tfjsonpath.New("service_account_id"), KnownValue: knownvalue.StringExact(accessControlServiceAccountID)},
		}),
	}}))
}

func groupMembershipQueryConfig() string {
	return `
list "ona_group_membership" "all" {
  provider = ona
  include_resource = true
  config { group_id = "22222222-2222-4222-8222-222222222222" }
}
`
}
