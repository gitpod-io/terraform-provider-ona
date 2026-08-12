// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"strings"
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestEmailSelectorValidator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Input    types.String
		Expected []string
	}{
		{Name: "accepts_email", Input: types.StringValue("alice@example.com")},
		{Name: "accepts_uppercase_email", Input: types.StringValue("ALICE@EXAMPLE.COM")},
		{Name: "accepts_unknown", Input: types.StringUnknown()},
		{Name: "rejects_empty_email", Input: types.StringValue(""), Expected: []string{"Invalid Ona User Email"}},
		{Name: "rejects_email_without_at_sign", Input: types.StringValue("alice.example.com"), Expected: []string{"Invalid Ona User Email"}},
		{Name: "rejects_display_name", Input: types.StringValue("Alice <alice@example.com>"), Expected: []string{"Invalid Ona User Email"}},
		{Name: "rejects_email_over_256_characters", Input: types.StringValue(strings.Repeat("a", 257)), Expected: []string{"Invalid Ona User Email"}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var resp validator.StringResponse
			EmailSelectorValidator{}.ValidateString(t.Context(), validator.StringRequest{
				ConfigValue: tc.Input,
				Path:        path.Root("email"),
			}, &resp)
			if diff := cmp.Diff(tc.Expected, diagnosticSummaries(resp.Diagnostics), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("EmailSelectorValidator.ValidateString() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateUserSelectorConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Input    UserModel
		Expected []string
	}{
		{Name: "accepts_user_id", Input: UserModel{UserID: types.StringValue(testUserID), Email: types.StringNull(), LoginProvider: types.StringNull()}},
		{Name: "accepts_unknown_user_id", Input: UserModel{UserID: types.StringUnknown(), Email: types.StringNull(), LoginProvider: types.StringNull()}},
		{Name: "defers_all_unknown_selectors", Input: UserModel{UserID: types.StringUnknown(), Email: types.StringUnknown(), LoginProvider: types.StringUnknown()}},
		{Name: "defers_unknown_user_id_with_known_identity", Input: UserModel{UserID: types.StringUnknown(), Email: types.StringValue("alice@example.com"), LoginProvider: types.StringValue("github")}},
		{Name: "defers_unknown_provider_with_known_email", Input: UserModel{UserID: types.StringNull(), Email: types.StringValue("alice@example.com"), LoginProvider: types.StringUnknown()}},
		{Name: "accepts_email_and_provider", Input: UserModel{UserID: types.StringNull(), Email: types.StringValue("alice@example.com"), LoginProvider: types.StringValue("github")}},
		{Name: "accepts_unknown_email_and_provider", Input: UserModel{UserID: types.StringNull(), Email: types.StringUnknown(), LoginProvider: types.StringUnknown()}},
		{Name: "rejects_missing_selector", Input: UserModel{UserID: types.StringNull(), Email: types.StringNull(), LoginProvider: types.StringNull()}, Expected: []string{"Missing Ona User Selector"}},
		{Name: "rejects_email_without_provider", Input: UserModel{UserID: types.StringNull(), Email: types.StringValue("alice@example.com"), LoginProvider: types.StringNull()}, Expected: []string{"Incomplete Ona User Selector"}},
		{Name: "rejects_provider_without_email", Input: UserModel{UserID: types.StringNull(), Email: types.StringNull(), LoginProvider: types.StringValue("github")}, Expected: []string{"Incomplete Ona User Selector"}},
		{Name: "rejects_conflicting_selectors", Input: UserModel{UserID: types.StringValue(testUserID), Email: types.StringValue("alice@example.com"), LoginProvider: types.StringValue("github")}, Expected: []string{"Conflicting Ona User Selectors"}},
		{Name: "rejects_definite_conflict_with_unknown_provider", Input: UserModel{UserID: types.StringValue(testUserID), Email: types.StringValue("alice@example.com"), LoginProvider: types.StringUnknown()}, Expected: []string{"Conflicting Ona User Selectors"}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateUserSelectorConfig(tc.Input, &diags)
			if diff := cmp.Diff(tc.Expected, diagnosticSummaries(diags), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("validateUserSelectorConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUserSelectorFromModelRejectsUnknownValues(t *testing.T) {
	t.Parallel()

	data := UserModel{UserID: types.StringNull(), Email: types.StringUnknown(), LoginProvider: types.StringUnknown()}
	var diags diag.Diagnostics
	_, ok := userSelectorFromModel(data, &diags)
	if ok {
		t.Error("userSelectorFromModel() succeeded, want unknown-value diagnostics")
	}
	want := []string{"Unknown Ona User Email", "Unknown Ona Login Provider"}
	if diff := cmp.Diff(want, diagnosticSummaries(diags)); diff != "" {
		t.Errorf("userSelectorFromModel() diagnostics mismatch (-want +got):\n%s", diff)
	}
}

func TestUserSelectorListFilter(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Search        string
		Email         string
		LoginProvider v1.LoginProviderKind
		UserIDs       []string
	}

	tests := []struct {
		Name     string
		Selector userSelector
		Expected Expectation
	}{
		{
			Name:     "filters_by_user_id",
			Selector: userSelector{byUserID: true, userID: testUserID},
			Expected: Expectation{UserIDs: []string{testUserID}},
		},
		{
			Name:     "filters_by_email_and_github",
			Selector: userSelector{email: "alice@example.com", loginProvider: "github"},
			Expected: Expectation{Email: "alice@example.com", LoginProvider: v1.LoginProviderKind_LOGIN_PROVIDER_KIND_GITHUB},
		},
		{
			Name:     "filters_by_email_and_google",
			Selector: userSelector{email: "alice@example.com", loginProvider: "google"},
			Expected: Expectation{Email: "alice@example.com", LoginProvider: v1.LoginProviderKind_LOGIN_PROVIDER_KIND_GOOGLE},
		},
		{
			Name:     "maps_custom_to_sso_filter",
			Selector: userSelector{email: "alice@example.com", loginProvider: "custom"},
			Expected: Expectation{Email: "alice@example.com", LoginProvider: v1.LoginProviderKind_LOGIN_PROVIDER_KIND_SSO},
		},
		{
			Name:     "leaves_unknown_provider_unspecified",
			Selector: userSelector{email: "alice@example.com", loginProvider: "unknown"},
			Expected: Expectation{Email: "alice@example.com"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			filter := tc.Selector.listFilter()
			got := Expectation{
				Search:        filter.GetSearch(),
				Email:         filter.GetEmail(),
				LoginProvider: filter.GetLoginProvider(),
				UserIDs:       filter.GetUserIds(),
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("userSelector.listFilter() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUserSelectorExactMatches(t *testing.T) {
	t.Parallel()

	members := []*v1.OrganizationMember{
		{UserId: testOtherUserID, Email: "ALICE@example.com", LoginProvider: "custom"},
		nil,
		{UserId: testUserID, Email: "alice@example.com", LoginProvider: "github"},
		{UserId: "00000000-0000-0000-0000-000000000003", Email: "Alice@Example.Com", LoginProvider: "github"},
	}

	tests := []struct {
		Name     string
		Selector userSelector
		Expected []string
	}{
		{Name: "matches_user_id", Selector: userSelector{byUserID: true, userID: testOtherUserID}, Expected: []string{testOtherUserID}},
		{Name: "matches_case_insensitive_email_and_exact_provider", Selector: userSelector{email: "ALICE@EXAMPLE.COM", loginProvider: "github"}, Expected: []string{testUserID, "00000000-0000-0000-0000-000000000003"}},
		{Name: "distinguishes_login_provider", Selector: userSelector{email: "alice@example.com", loginProvider: "custom"}, Expected: []string{testOtherUserID}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			matches := tc.Selector.exactMatches(members)
			got := make([]string, 0, len(matches))
			for _, member := range matches {
				got = append(got, member.GetUserId())
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("exactMatches() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
