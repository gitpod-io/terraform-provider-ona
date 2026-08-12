// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"regexp"
	"sort"
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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func (s *fakeGroupService) ListMemberships(ctx context.Context, req *connect.Request[v1.ListMembershipsRequest]) (*connect.Response[v1.ListMembershipsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.membershipListCalls = append(s.membershipListCalls, proto.CloneOf(req.Msg))
	var members []*v1.GroupMembership
	for _, member := range s.memberships {
		if member.GetGroupId() != req.Msg.GetGroupId() {
			continue
		}
		search := strings.ToLower(req.Msg.GetFilter().GetSearch())
		if search != "" && !strings.Contains(strings.ToLower(member.GetName()), search) && !strings.Contains(strings.ToLower(member.GetSubject().GetId()), search) {
			continue
		}
		members = append(members, cloneMembership(member))
	}
	sort.Slice(members, func(i, j int) bool {
		left := memberKey(members[i].GetGroupId(), members[i].GetSubject().GetPrincipal(), members[i].GetSubject().GetId()) + "/" + members[i].GetId()
		right := memberKey(members[j].GetGroupId(), members[j].GetSubject().GetPrincipal(), members[j].GetSubject().GetId()) + "/" + members[j].GetId()
		return left < right
	})
	start, end, nextToken, err := fakePage(req.Msg.GetPagination(), len(members), s.membershipPageLimit)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ListMembershipsResponse{
		Members:    members[start:end],
		Pagination: &v1.PaginationResponse{NextToken: nextToken},
	}), nil
}

func TestAccGroupMembershipQuery(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.memberships[memberKey(accessControlGroupID, v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, accessControlServiceAccountID)] = &v1.GroupMembership{Id: accessControlMembershipID, GroupId: accessControlGroupID, Subject: &v1.Subject{Id: accessControlServiceAccountID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}, Name: "Terraform Service Account"}
	server.service.memberships[memberKey(accessControlGroupID, v1.Principal_PRINCIPAL_USER, accessControlUserID)] = &v1.GroupMembership{Id: "user-membership", GroupId: accessControlGroupID, Subject: &v1.Subject{Id: accessControlUserID, Principal: v1.Principal_PRINCIPAL_USER}, Name: "Ona User"}
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: groupMembershipQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_group_membership.all", 2),
		querycheck.ExpectIdentity("ona_group_membership.all", map[string]knownvalue.Check{"group_id": knownvalue.StringExact(accessControlGroupID), "service_account_id": knownvalue.StringExact(accessControlServiceAccountID), "user_id": knownvalue.Null()}),
		querycheck.ExpectIdentity("ona_group_membership.all", map[string]knownvalue.Check{"group_id": knownvalue.StringExact(accessControlGroupID), "service_account_id": knownvalue.Null(), "user_id": knownvalue.StringExact(accessControlUserID)}),
		querycheck.ExpectResourceKnownValues("ona_group_membership.all", queryfilter.ByResourceIdentity(map[string]knownvalue.Check{"group_id": knownvalue.StringExact(accessControlGroupID), "service_account_id": knownvalue.StringExact(accessControlServiceAccountID), "user_id": knownvalue.Null()}), []querycheck.KnownValueCheck{
			{Path: tfjsonpath.New("group_id"), KnownValue: knownvalue.StringExact(accessControlGroupID)},
			{Path: tfjsonpath.New("service_account_id"), KnownValue: knownvalue.StringExact(accessControlServiceAccountID)},
			{Path: tfjsonpath.New("user_id"), KnownValue: knownvalue.Null()},
		}),
		querycheck.ExpectResourceKnownValues("ona_group_membership.all", queryfilter.ByResourceIdentity(map[string]knownvalue.Check{"group_id": knownvalue.StringExact(accessControlGroupID), "service_account_id": knownvalue.Null(), "user_id": knownvalue.StringExact(accessControlUserID)}), []querycheck.KnownValueCheck{
			{Path: tfjsonpath.New("group_id"), KnownValue: knownvalue.StringExact(accessControlGroupID)},
			{Path: tfjsonpath.New("service_account_id"), KnownValue: knownvalue.Null()},
			{Path: tfjsonpath.New("user_id"), KnownValue: knownvalue.StringExact(accessControlUserID)},
		}),
	}}))
}

func TestAccGroupMembershipQueryDistinguishesPrincipalWithSharedID(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	const sharedID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	server.service.membershipPageLimit = 1
	server.service.memberships[memberKey(accessControlGroupID, v1.Principal_PRINCIPAL_USER, sharedID)] = &v1.GroupMembership{Id: "user-membership", GroupId: accessControlGroupID, Subject: &v1.Subject{Id: sharedID, Principal: v1.Principal_PRINCIPAL_USER}}
	server.service.memberships[memberKey(accessControlGroupID, v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, sharedID)] = &v1.GroupMembership{Id: "service-account-membership", GroupId: accessControlGroupID, Subject: &v1.Subject{Id: sharedID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}}
	userIdentity := map[string]knownvalue.Check{"group_id": knownvalue.StringExact(accessControlGroupID), "service_account_id": knownvalue.Null(), "user_id": knownvalue.StringExact(sharedID)}
	serviceAccountIdentity := map[string]knownvalue.Check{"group_id": knownvalue.StringExact(accessControlGroupID), "service_account_id": knownvalue.StringExact(sharedID), "user_id": knownvalue.Null()}
	normalizedSharedID := strings.ReplaceAll(sharedID, "-", "_")

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: groupMembershipIdentityQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_group_membership.all", 2),
		querycheck.ExpectIdentity("ona_group_membership.all", userIdentity),
		querycheck.ExpectIdentity("ona_group_membership.all", serviceAccountIdentity),
		querycheck.ExpectResourceDisplayName("ona_group_membership.all", queryfilter.ByResourceIdentity(userIdentity), knownvalue.StringExact("user_"+normalizedSharedID)),
		querycheck.ExpectResourceDisplayName("ona_group_membership.all", queryfilter.ByResourceIdentity(serviceAccountIdentity), knownvalue.StringExact("service_account_"+normalizedSharedID)),
	}}))
}

func TestAccGroupMembershipQuerySearchesAndPaginatesToLimit(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.membershipPageLimit = 1
	server.service.memberships[memberKey(accessControlGroupID, v1.Principal_PRINCIPAL_USER, accessControlUserID)] = &v1.GroupMembership{Id: "user-membership", GroupId: accessControlGroupID, Subject: &v1.Subject{Id: accessControlUserID, Principal: v1.Principal_PRINCIPAL_USER}, Name: "Match User"}
	server.service.memberships[memberKey(accessControlGroupID, v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, accessControlServiceAccountID)] = &v1.GroupMembership{Id: accessControlMembershipID, GroupId: accessControlGroupID, Subject: &v1.Subject{Id: accessControlServiceAccountID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}, Name: "Match Service Account"}
	server.service.memberships[memberKey(accessControlGroupID, v1.Principal_PRINCIPAL_USER, accessControlOtherUserID)] = &v1.GroupMembership{Id: "other-user-membership", GroupId: accessControlGroupID, Subject: &v1.Subject{Id: accessControlOtherUserID, Principal: v1.Principal_PRINCIPAL_USER}, Name: "Other User"}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: groupMembershipLimitedQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_group_membership.all", 2),
		querycheck.ExpectIdentity("ona_group_membership.all", map[string]knownvalue.Check{"group_id": knownvalue.StringExact(accessControlGroupID), "service_account_id": knownvalue.Null(), "user_id": knownvalue.StringExact(accessControlUserID)}),
		querycheck.ExpectIdentity("ona_group_membership.all", map[string]knownvalue.Check{"group_id": knownvalue.StringExact(accessControlGroupID), "service_account_id": knownvalue.StringExact(accessControlServiceAccountID), "user_id": knownvalue.Null()}),
	}}))

	expected := []*v1.ListMembershipsRequest{
		{GroupId: accessControlGroupID, Filter: &v1.ListMembershipsRequest_Filter{Search: "match"}, Pagination: &v1.PaginationRequest{PageSize: 2}},
		{GroupId: accessControlGroupID, Filter: &v1.ListMembershipsRequest_Filter{Search: "match"}, Pagination: &v1.PaginationRequest{PageSize: 1, Token: "1"}},
	}
	if diff := cmp.Diff(expected, server.service.membershipListRequestsSnapshot(), protocmp.Transform()); diff != "" {
		t.Errorf("membership list requests mismatch (-want +got):\n%s", diff)
	}
}

func TestAccGroupMembershipQueryRejectsUnsupportedPrincipal(t *testing.T) {
	t.Parallel()

	server := newAccessControlAPIServer(t)
	t.Cleanup(server.Close)
	server.service.memberships[memberKey(accessControlGroupID, v1.Principal_PRINCIPAL_RUNNER, "runner-1")] = &v1.GroupMembership{Id: "runner-membership", GroupId: accessControlGroupID, Subject: &v1.Subject{Id: "runner-1", Principal: v1.Principal_PRINCIPAL_RUNNER}, Name: "Runner"}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      groupMembershipIdentityQueryConfig(),
		ExpectError: regexp.MustCompile(`Unable to Map Ona Group Membership[\s\S]*PRINCIPAL_RUNNER`),
	}))
}

func (s *fakeGroupService) membershipListRequestsSnapshot() []*v1.ListMembershipsRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*v1.ListMembershipsRequest, 0, len(s.membershipListCalls))
	for _, request := range s.membershipListCalls {
		result = append(result, proto.CloneOf(request))
	}
	return result
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

func groupMembershipIdentityQueryConfig() string {
	return `
list "ona_group_membership" "all" {
  provider = ona
  config { group_id = "22222222-2222-4222-8222-222222222222" }
}
`
}

func groupMembershipLimitedQueryConfig() string {
	return `
list "ona_group_membership" "all" {
  provider = ona
  limit    = 2
  config {
    group_id = "22222222-2222-4222-8222-222222222222"
    search   = "match"
  }
}
`
}
