// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestValidateTeamMembership(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Err string
	}

	tests := []struct {
		Name     string
		Member   *v1.TeamMember
		TeamID   string
		UserID   string
		Expected Expectation
	}{
		{Name: "valid", Member: &v1.TeamMember{Id: "membership", TeamId: "team", UserId: "user"}, TeamID: "team", UserID: "user"},
		{Name: "empty_member", Expected: Expectation{Err: "the Ona API returned an empty team membership"}},
		{Name: "missing_id", Member: &v1.TeamMember{TeamId: "team", UserId: "user"}, Expected: Expectation{Err: "the Ona API returned a team membership without an ID"}},
		{Name: "missing_team_id", Member: &v1.TeamMember{Id: "membership", UserId: "user"}, Expected: Expectation{Err: `the Ona API returned team membership "membership" without a team ID`}},
		{Name: "missing_user_id", Member: &v1.TeamMember{Id: "membership", TeamId: "team"}, Expected: Expectation{Err: `the Ona API returned team membership "membership" without a user ID`}},
		{Name: "wrong_team", Member: &v1.TeamMember{Id: "membership", TeamId: "other-team", UserId: "user"}, TeamID: "team", Expected: Expectation{Err: `the Ona API returned team membership "membership" for team "other-team" instead of "team"`}},
		{Name: "wrong_user", Member: &v1.TeamMember{Id: "membership", TeamId: "team", UserId: "other-user"}, TeamID: "team", UserID: "user", Expected: Expectation{Err: `the Ona API returned team membership "membership" for user "other-user" instead of "user"`}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			if err := validateTeamMembership(tc.Member, tc.TeamID, tc.UserID); err != nil {
				got.Err = err.Error()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateTeamMembership() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPopulateTeamMembershipModel(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		ID     string
		TeamID string
		UserID string
	}

	var data TeamMembershipModel
	populateTeamMembershipModel(&data, &v1.TeamMember{Id: "membership", TeamId: "team", UserId: "user"})
	got := Expectation{ID: data.ID.ValueString(), TeamID: data.TeamID.ValueString(), UserID: data.UserID.ValueString()}
	expected := Expectation{ID: "membership", TeamID: "team", UserID: "user"}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("populateTeamMembershipModel() mismatch (-want +got):\n%s", diff)
	}
}

func TestTeamMembershipIdentity(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		TeamID string
		UserID string
		Err    string
	}

	tests := []struct {
		Name     string
		Input    TeamMembershipModel
		Expected Expectation
	}{
		{Name: "valid", Input: TeamMembershipModel{TeamID: types.StringValue("team"), UserID: types.StringValue("user")}, Expected: Expectation{TeamID: "team", UserID: "user"}},
		{Name: "null_team", Input: TeamMembershipModel{TeamID: types.StringNull(), UserID: types.StringValue("user")}, Expected: Expectation{Err: "team_id must be known and configured"}},
		{Name: "unknown_team", Input: TeamMembershipModel{TeamID: types.StringUnknown(), UserID: types.StringValue("user")}, Expected: Expectation{Err: "team_id must be known and configured"}},
		{Name: "empty_team", Input: TeamMembershipModel{TeamID: types.StringValue(""), UserID: types.StringValue("user")}, Expected: Expectation{Err: "team_id must be known and configured"}},
		{Name: "null_user", Input: TeamMembershipModel{TeamID: types.StringValue("team"), UserID: types.StringNull()}, Expected: Expectation{Err: "user_id must be known and configured"}},
		{Name: "unknown_user", Input: TeamMembershipModel{TeamID: types.StringValue("team"), UserID: types.StringUnknown()}, Expected: Expectation{Err: "user_id must be known and configured"}},
		{Name: "empty_user", Input: TeamMembershipModel{TeamID: types.StringValue("team"), UserID: types.StringValue("")}, Expected: Expectation{Err: "user_id must be known and configured"}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			identity, err := teamMembershipIdentity(tc.Input)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.TeamID = identity.TeamID.ValueString()
				got.UserID = identity.UserID.ValueString()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("teamMembershipIdentity() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSortTeamMemberships(t *testing.T) {
	t.Parallel()

	members := []*v1.TeamMember{
		{Id: "membership-3", UserId: "user-2"},
		{Id: "membership-2", UserId: "user-1"},
		{Id: "membership-1", UserId: "user-1"},
	}
	sortTeamMemberships(members)

	got := make([]string, 0, len(members))
	for _, member := range members {
		got = append(got, member.GetUserId()+"/"+member.GetId())
	}
	expected := []string{"user-1/membership-1", "user-1/membership-2", "user-2/membership-3"}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("sortTeamMemberships() mismatch (-want +got):\n%s", diff)
	}
}

func TestTeamMembershipDisplayNameFallback(t *testing.T) {
	t.Parallel()

	got := teamMembershipDisplayNameFallback(&v1.TeamMember{UserId: "user-1"})
	if diff := cmp.Diff("user_user-1", got); diff != "" {
		t.Errorf("teamMembershipDisplayNameFallback() mismatch (-want +got):\n%s", diff)
	}
}
