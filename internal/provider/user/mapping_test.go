// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	testOrganizationID = "00000000-0000-0000-0000-000000000100"
	testUserID         = "00000000-0000-0000-0000-000000000001"
	testOtherUserID    = "00000000-0000-0000-0000-000000000002"
)

func TestUserCollectionFilter(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Filter *v1.ListMembersRequest_Filter
		Errors []string
	}
	tests := []struct {
		Name     string
		Input    UserCollectionDataSourceModel
		Expected Expectation
	}{
		{
			Name:     "omits_empty_filter",
			Input:    UserCollectionDataSourceModel{},
			Expected: Expectation{},
		},
		{
			Name: "maps_and_sorts_filters",
			Input: UserCollectionDataSourceModel{
				Search:   types.StringValue("  example.com  "),
				Statuses: stringSet("left", "active"),
				Roles:    stringSet("member", "admin"),
				UserIDs:  stringSet(testOtherUserID, testUserID),
			},
			Expected: Expectation{Filter: &v1.ListMembersRequest_Filter{
				Search:   "example.com",
				Statuses: []v1.UserStatus{v1.UserStatus_USER_STATUS_ACTIVE, v1.UserStatus_USER_STATUS_LEFT},
				Roles:    []v1.OrganizationRole{v1.OrganizationRole_ORGANIZATION_ROLE_ADMIN, v1.OrganizationRole_ORGANIZATION_ROLE_MEMBER},
				UserIds:  []string{testUserID, testOtherUserID},
			}},
		},
		{
			Name:     "ignores_blank_search",
			Input:    UserCollectionDataSourceModel{Search: types.StringValue("  ")},
			Expected: Expectation{},
		},
		{
			Name:     "rejects_unknown_search",
			Input:    UserCollectionDataSourceModel{Search: types.StringUnknown()},
			Expected: Expectation{Errors: []string{"Unknown User Search"}},
		},
		{
			Name:     "rejects_long_search",
			Input:    UserCollectionDataSourceModel{Search: types.StringValue(strings.Repeat("a", 257))},
			Expected: Expectation{Errors: []string{"User Search Is Too Long"}},
		},
		{
			Name:     "rejects_invalid_status",
			Input:    UserCollectionDataSourceModel{Statuses: stringSet("pending")},
			Expected: Expectation{Errors: []string{"Invalid User Status"}},
		},
		{
			Name:     "rejects_invalid_role",
			Input:    UserCollectionDataSourceModel{Roles: stringSet("owner")},
			Expected: Expectation{Errors: []string{"Invalid Organization Role"}},
		},
		{
			Name:     "rejects_invalid_user_id",
			Input:    UserCollectionDataSourceModel{UserIDs: stringSet("not-a-uuid")},
			Expected: Expectation{Errors: []string{"Invalid User Filter UUID"}},
		},
		{
			Name:     "rejects_unknown_user_ids",
			Input:    UserCollectionDataSourceModel{UserIDs: types.SetUnknown(types.StringType)},
			Expected: Expectation{Errors: []string{"Unknown User Filter"}},
		},
		{
			Name:     "rejects_too_many_user_ids",
			Input:    UserCollectionDataSourceModel{UserIDs: sequentialUUIDSet(26)},
			Expected: Expectation{Errors: []string{"Too Many User Filter Values"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			filter := userCollectionFilter(tc.Input, &diags)
			got := Expectation{Filter: filter, Errors: diagnosticSummaries(diags)}
			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform(), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("userCollectionFilter() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUserModelFromMember(t *testing.T) {
	t.Parallel()

	memberSince := timestamppb.New(time.Date(2026, 7, 17, 12, 30, 0, 123, time.UTC))
	type Expectation struct {
		Result UserModel
		Err    string
	}
	tests := []struct {
		Name     string
		Input    *v1.OrganizationMember
		Expected Expectation
	}{
		{
			Name:  "maps_member",
			Input: testOrganizationMember(testUserID, "Alice", "alice@example.com", v1.UserStatus_USER_STATUS_ACTIVE, v1.OrganizationRole_ORGANIZATION_ROLE_MEMBER, memberSince),
			Expected: Expectation{Result: UserModel{
				ID:            types.StringValue(testUserID),
				UserID:        types.StringValue(testUserID),
				Name:          types.StringValue("Alice"),
				Email:         types.StringValue("alice@example.com"),
				Status:        types.StringValue("active"),
				Role:          types.StringValue("member"),
				MemberSince:   types.StringValue("2026-07-17T12:30:00.000000123Z"),
				LoginProvider: types.StringValue("github"),
			}},
		},
		{
			Name:     "maps_empty_optional_strings_to_null",
			Input:    testOrganizationMember(testUserID, "Alice", "", v1.UserStatus_USER_STATUS_SUSPENDED, v1.OrganizationRole_ORGANIZATION_ROLE_ADMIN, nil),
			Expected: Expectation{Result: UserModel{ID: types.StringValue(testUserID), UserID: types.StringValue(testUserID), Name: types.StringValue("Alice"), Email: types.StringNull(), Status: types.StringValue("suspended"), Role: types.StringValue("admin"), MemberSince: types.StringNull(), LoginProvider: types.StringValue("github")}},
		},
		{
			Name:     "rejects_nil_member",
			Expected: Expectation{Err: "the Ona API returned an empty organization member"},
		},
		{
			Name:     "rejects_invalid_user_id",
			Input:    testOrganizationMember("invalid", "Alice", "alice@example.com", v1.UserStatus_USER_STATUS_ACTIVE, v1.OrganizationRole_ORGANIZATION_ROLE_MEMBER, memberSince),
			Expected: Expectation{Err: `the Ona API returned invalid user ID "invalid": invalid UUID length: 7`},
		},
		{
			Name:     "rejects_unspecified_status",
			Input:    testOrganizationMember(testUserID, "Alice", "alice@example.com", v1.UserStatus_USER_STATUS_UNSPECIFIED, v1.OrganizationRole_ORGANIZATION_ROLE_MEMBER, memberSince),
			Expected: Expectation{Err: `the Ona API returned unsupported user status "USER_STATUS_UNSPECIFIED"`},
		},
		{
			Name:     "rejects_unspecified_role",
			Input:    testOrganizationMember(testUserID, "Alice", "alice@example.com", v1.UserStatus_USER_STATUS_ACTIVE, v1.OrganizationRole_ORGANIZATION_ROLE_UNSPECIFIED, memberSince),
			Expected: Expectation{Err: `the Ona API returned unsupported organization role "ORGANIZATION_ROLE_UNSPECIFIED"`},
		},
		{
			Name:     "rejects_invalid_timestamp",
			Input:    testOrganizationMember(testUserID, "Alice", "alice@example.com", v1.UserStatus_USER_STATUS_ACTIVE, v1.OrganizationRole_ORGANIZATION_ROLE_MEMBER, &timestamppb.Timestamp{Seconds: 253402300800}),
			Expected: Expectation{Err: `the Ona API returned invalid member_since for user "00000000-0000-0000-0000-000000000001": proto: timestamp (seconds:253402300800) after 9999-12-31`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, err := userModelFromMember(tc.Input)
			if err != nil {
				got.Err = normalizeWhitespace(err.Error())
			} else {
				got.Result = result
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("userModelFromMember() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUserModelFromResponses(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result UserModel
		Err    string
	}
	member := testOrganizationMember(testUserID, "Member Name", "member@example.com", v1.UserStatus_USER_STATUS_ACTIVE, v1.OrganizationRole_ORGANIZATION_ROLE_MEMBER, nil)
	tests := []struct {
		Name     string
		User     *v1.User
		Member   *v1.OrganizationMember
		Expected Expectation
	}{
		{
			Name:   "prefers_core_user_fields",
			User:   &v1.User{Id: testUserID, Name: "Canonical Name", Email: "canonical@example.com", Status: v1.UserStatus_USER_STATUS_ACTIVE},
			Member: member,
			Expected: Expectation{Result: UserModel{
				ID: types.StringValue(testUserID), UserID: types.StringValue(testUserID), Name: types.StringValue("Canonical Name"), Email: types.StringValue("canonical@example.com"), Status: types.StringValue("active"), Role: types.StringValue("member"), MemberSince: types.StringNull(), LoginProvider: types.StringValue("github"),
			}},
		},
		{Name: "rejects_nil_user", Member: member, Expected: Expectation{Err: "the Ona API returned an empty user"}},
		{Name: "rejects_nil_member", User: &v1.User{Id: testUserID}, Expected: Expectation{Err: "the Ona API returned an empty organization member"}},
		{Name: "rejects_mismatched_ids", User: &v1.User{Id: testOtherUserID, Status: v1.UserStatus_USER_STATUS_ACTIVE}, Member: member, Expected: Expectation{Err: `the Ona API returned mismatched user IDs "00000000-0000-0000-0000-000000000002" and "00000000-0000-0000-0000-000000000001"`}},
		{Name: "rejects_mismatched_statuses", User: &v1.User{Id: testUserID, Status: v1.UserStatus_USER_STATUS_LEFT}, Member: member, Expected: Expectation{Err: `the Ona API returned mismatched statuses for user "00000000-0000-0000-0000-000000000001"`}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, err := userModelFromResponses(tc.User, tc.Member)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Result = result
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("userModelFromResponses() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func stringSet(values ...string) types.Set {
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, types.StringValue(value))
	}
	return types.SetValueMust(types.StringType, elements)
}

func sequentialUUIDSet(count int) types.Set {
	values := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		values = append(values, fmt.Sprintf("00000000-0000-0000-0000-%012d", i))
	}
	return stringSet(values...)
}

func diagnosticSummaries(diags diag.Diagnostics) []string {
	result := make([]string, 0, len(diags))
	for _, diagnostic := range diags {
		if diagnostic.Severity() == diag.SeverityError {
			result = append(result, diagnostic.Summary())
		}
	}
	return result
}

func normalizeWhitespace(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, value)
}

func testOrganizationMember(id string, name string, email string, status v1.UserStatus, role v1.OrganizationRole, memberSince *timestamppb.Timestamp) *v1.OrganizationMember {
	return &v1.OrganizationMember{
		UserId:        id,
		FullName:      name,
		Email:         email,
		Status:        status,
		Role:          role,
		MemberSince:   memberSince,
		LoginProvider: "github",
	}
}
