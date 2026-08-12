// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"strings"
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestGroupMembershipSubject(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Subject *v1.Subject
		Err     string
	}
	tests := []struct {
		Name             string
		UserID           types.String
		ServiceAccountID types.String
		Expected         Expectation
	}{
		{
			Name:             "maps_user",
			UserID:           types.StringValue("user-1"),
			ServiceAccountID: types.StringNull(),
			Expected: Expectation{Subject: &v1.Subject{
				Id:        "user-1",
				Principal: v1.Principal_PRINCIPAL_USER,
			}},
		},
		{
			Name:             "maps_service_account",
			UserID:           types.StringNull(),
			ServiceAccountID: types.StringValue("service-account-1"),
			Expected: Expectation{Subject: &v1.Subject{
				Id:        "service-account-1",
				Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT,
			}},
		},
		{
			Name:             "rejects_missing_member",
			UserID:           types.StringNull(),
			ServiceAccountID: types.StringNull(),
			Expected:         Expectation{Err: "exactly one of user_id or service_account_id must be configured"},
		},
		{
			Name:             "rejects_multiple_members",
			UserID:           types.StringValue("user-1"),
			ServiceAccountID: types.StringValue("service-account-1"),
			Expected:         Expectation{Err: "exactly one of user_id or service_account_id must be configured"},
		},
		{
			Name:             "rejects_unknown_user",
			UserID:           types.StringUnknown(),
			ServiceAccountID: types.StringNull(),
			Expected:         Expectation{Err: "exactly one of user_id or service_account_id must be known and configured"},
		},
		{
			Name:             "rejects_unknown_service_account",
			UserID:           types.StringNull(),
			ServiceAccountID: types.StringUnknown(),
			Expected:         Expectation{Err: "exactly one of user_id or service_account_id must be known and configured"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			var err error
			got.Subject, err = groupMembershipSubject(tc.UserID, tc.ServiceAccountID)
			if err != nil {
				got.Err = err.Error()
			}
			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("groupMembershipSubject() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPopulateGroupMembershipModel(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		ID                   string
		GroupID              string
		ServiceAccountID     string
		ServiceAccountIDNull bool
		UserID               string
		UserIDNull           bool
		Err                  string
	}
	tests := []struct {
		Name     string
		Member   *v1.GroupMembership
		Expected Expectation
	}{
		{
			Name: "maps_user",
			Member: &v1.GroupMembership{
				Id:      "membership-1",
				GroupId: "group-1",
				Subject: &v1.Subject{Id: "user-1", Principal: v1.Principal_PRINCIPAL_USER},
			},
			Expected: Expectation{ID: "membership-1", GroupID: "group-1", ServiceAccountIDNull: true, UserID: "user-1"},
		},
		{
			Name: "maps_service_account",
			Member: &v1.GroupMembership{
				Id:      "membership-1",
				GroupId: "group-1",
				Subject: &v1.Subject{Id: "service-account-1", Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT},
			},
			Expected: Expectation{ID: "membership-1", GroupID: "group-1", ServiceAccountID: "service-account-1", UserIDNull: true},
		},
		{
			Name:     "rejects_empty_membership",
			Expected: Expectation{Err: "the Ona API returned an empty membership"},
		},
		{
			Name:     "rejects_missing_subject",
			Member:   &v1.GroupMembership{Id: "membership-1", GroupId: "group-1"},
			Expected: Expectation{Err: "the Ona API returned a membership without a subject"},
		},
		{
			Name: "rejects_unsupported_principal",
			Member: &v1.GroupMembership{
				Id:      "membership-1",
				GroupId: "group-1",
				Subject: &v1.Subject{Id: "runner-1", Principal: v1.Principal_PRINCIPAL_RUNNER},
			},
			Expected: Expectation{Err: `the Ona API returned unsupported membership principal "PRINCIPAL_RUNNER"`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			var model GroupMembershipModel
			if err := populateGroupMembershipModel(&model, tc.Member); err != nil {
				got.Err = err.Error()
			} else {
				got.ID = model.ID.ValueString()
				got.GroupID = model.GroupID.ValueString()
				got.ServiceAccountID = model.ServiceAccountID.ValueString()
				got.ServiceAccountIDNull = model.ServiceAccountID.IsNull()
				got.UserID = model.UserID.ValueString()
				got.UserIDNull = model.UserID.IsNull()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("populateGroupMembershipModel() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseGroupMembershipImportID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		GroupID string
		Subject *v1.Subject
		Err     string
	}
	const formats = "expected import ID format: group_id/service_account_id, group_id/service_account/service_account_id, or group_id/user/user_id"
	tests := []struct {
		Name     string
		ID       string
		Expected Expectation
	}{
		{
			Name: "parses_legacy_service_account",
			ID:   "group-1/service-account-1",
			Expected: Expectation{GroupID: "group-1", Subject: &v1.Subject{
				Id:        "service-account-1",
				Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT,
			}},
		},
		{
			Name: "parses_typed_service_account",
			ID:   "group-1/service_account/service-account-1",
			Expected: Expectation{GroupID: "group-1", Subject: &v1.Subject{
				Id:        "service-account-1",
				Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT,
			}},
		},
		{
			Name: "parses_typed_user",
			ID:   "group-1/user/user-1",
			Expected: Expectation{GroupID: "group-1", Subject: &v1.Subject{
				Id:        "user-1",
				Principal: v1.Principal_PRINCIPAL_USER,
			}},
		},
		{Name: "rejects_too_few_parts", ID: "group-1", Expected: Expectation{Err: formats}},
		{Name: "rejects_too_many_parts", ID: "group-1/user/user-1/extra", Expected: Expectation{Err: formats}},
		{Name: "rejects_empty_part", ID: "group-1/user/", Expected: Expectation{Err: formats}},
		{Name: "rejects_unknown_principal", ID: "group-1/group/group-1", Expected: Expectation{Err: formats}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			var err error
			got.GroupID, got.Subject, err = parseGroupMembershipImportID(tc.ID)
			if err != nil {
				got.Err = err.Error()
			}
			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("parseGroupMembershipImportID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGroupMembershipImportStateValidatesStructuredIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Identity GroupMembershipIdentityModel
		Expected string
	}{
		{
			Name: "rejects_missing_member",
			Identity: GroupMembershipIdentityModel{
				GroupID:          types.StringValue("group-1"),
				ServiceAccountID: types.StringNull(),
				UserID:           types.StringNull(),
			},
			Expected: "Invalid Group Membership Identity: exactly one of user_id or service_account_id must be configured",
		},
		{
			Name: "rejects_multiple_members",
			Identity: GroupMembershipIdentityModel{
				GroupID:          types.StringValue("group-1"),
				ServiceAccountID: types.StringValue("service-account-1"),
				UserID:           types.StringValue("user-1"),
			},
			Expected: "Invalid Group Membership Identity: exactly one of user_id or service_account_id must be configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			resourceUnderTest := &GroupMembershipResource{}
			var schemaResponse resource.IdentitySchemaResponse
			resourceUnderTest.IdentitySchema(ctx, resource.IdentitySchemaRequest{}, &schemaResponse)
			if schemaResponse.Diagnostics.HasError() {
				t.Fatalf("IdentitySchema() diagnostics: %v", schemaResponse.Diagnostics)
			}

			identity := &tfsdk.ResourceIdentity{Schema: schemaResponse.IdentitySchema}
			if diagnostics := identity.Set(ctx, tc.Identity); diagnostics.HasError() {
				t.Fatalf("setting structured import identity: %v", diagnostics)
			}
			response := resource.ImportStateResponse{}
			resourceUnderTest.ImportState(ctx, resource.ImportStateRequest{Identity: identity}, &response)

			var errors []string
			for _, diagnostic := range response.Diagnostics.Errors() {
				errors = append(errors, diagnostic.Summary()+": "+diagnostic.Detail())
			}
			if diff := cmp.Diff(tc.Expected, strings.Join(errors, "\n")); diff != "" {
				t.Errorf("ImportState() diagnostics mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSortGroupMemberships(t *testing.T) {
	t.Parallel()

	members := []*v1.GroupMembership{
		{Id: "service-account-membership", Subject: &v1.Subject{Id: "shared-id", Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}},
		{Id: "user-membership-b", Subject: &v1.Subject{Id: "shared-id", Principal: v1.Principal_PRINCIPAL_USER}},
		{Id: "other-user-membership", Subject: &v1.Subject{Id: "z-user", Principal: v1.Principal_PRINCIPAL_USER}},
		{Id: "user-membership-a", Subject: &v1.Subject{Id: "shared-id", Principal: v1.Principal_PRINCIPAL_USER}},
	}
	sortGroupMemberships(members)

	got := make([]string, 0, len(members))
	for _, member := range members {
		got = append(got, member.GetId())
	}
	expected := []string{"user-membership-a", "user-membership-b", "other-user-membership", "service-account-membership"}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("sortGroupMemberships() mismatch (-want +got):\n%s", diff)
	}
}
