// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"fmt"
	"sort"
	"strings"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

type userSelector struct {
	byUserID      bool
	userID        string
	email         string
	loginProvider string
}

func validateUserSelectorConfig(data UserModel, diags *diag.Diagnostics) {
	userIDSet := !data.UserID.IsNull() && !data.UserID.IsUnknown()
	emailSet := !data.Email.IsNull() && !data.Email.IsUnknown()
	loginProviderSet := !data.LoginProvider.IsNull() && !data.LoginProvider.IsUnknown()

	if userIDSet && (emailSet || loginProviderSet) {
		diags.AddError(
			"Conflicting Ona User Selectors",
			"Specify either user_id or both email and login_provider. Do not combine the two lookup modes.",
		)
		return
	}
	if data.UserID.IsUnknown() || data.Email.IsUnknown() || data.LoginProvider.IsUnknown() {
		return
	}
	if userIDSet {
		return
	}
	if emailSet != loginProviderSet {
		diags.AddError(
			"Incomplete Ona User Selector",
			"Email and login_provider must be specified together when user_id is omitted.",
		)
		return
	}
	if !emailSet {
		diags.AddError(
			"Missing Ona User Selector",
			"Specify either user_id or both email and login_provider.",
		)
	}
}

func userSelectorFromModel(data UserModel, diags *diag.Diagnostics) (userSelector, bool) {
	validateUserSelectorConfig(data, diags)
	if diags.HasError() {
		return userSelector{}, false
	}

	if !data.UserID.IsNull() {
		if data.UserID.IsUnknown() {
			diags.AddAttributeError(path.Root("user_id"), "Unknown Ona User ID", "user_id must be known before reading the data source.")
			return userSelector{}, false
		}
		return userSelector{byUserID: true, userID: data.UserID.ValueString()}, true
	}
	if data.Email.IsUnknown() {
		diags.AddAttributeError(path.Root("email"), "Unknown Ona User Email", "email must be known before reading the data source.")
	}
	if data.LoginProvider.IsUnknown() {
		diags.AddAttributeError(path.Root("login_provider"), "Unknown Ona Login Provider", "login_provider must be known before reading the data source.")
	}
	if diags.HasError() {
		return userSelector{}, false
	}
	return userSelector{email: data.Email.ValueString(), loginProvider: data.LoginProvider.ValueString()}, true
}

func (s userSelector) listFilter() *v1.ListMembersRequest_Filter {
	if s.byUserID {
		return &v1.ListMembersRequest_Filter{UserIds: []string{s.userID}}
	}
	return &v1.ListMembersRequest_Filter{
		Email:         s.email,
		LoginProvider: loginProviderKind(s.loginProvider),
	}
}

func loginProviderKind(provider string) v1.LoginProviderKind {
	switch provider {
	case "custom":
		return v1.LoginProviderKind_LOGIN_PROVIDER_KIND_SSO
	case "github":
		return v1.LoginProviderKind_LOGIN_PROVIDER_KIND_GITHUB
	case "google":
		return v1.LoginProviderKind_LOGIN_PROVIDER_KIND_GOOGLE
	default:
		return v1.LoginProviderKind_LOGIN_PROVIDER_KIND_UNSPECIFIED
	}
}

func (s userSelector) exactMatches(members []*v1.OrganizationMember) []*v1.OrganizationMember {
	matches := make([]*v1.OrganizationMember, 0, len(members))
	for _, member := range members {
		if member == nil {
			continue
		}
		if s.byUserID {
			if member.GetUserId() == s.userID {
				matches = append(matches, member)
			}
			continue
		}
		if strings.EqualFold(member.GetEmail(), s.email) && member.GetLoginProvider() == s.loginProvider {
			matches = append(matches, member)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].GetUserId() < matches[j].GetUserId()
	})
	return matches
}

func describeUserMatches(members []*v1.OrganizationMember) string {
	descriptions := make([]string, 0, len(members))
	for _, member := range members {
		status := strings.ToLower(strings.TrimPrefix(member.GetStatus().String(), "USER_STATUS_"))
		descriptions = append(descriptions, fmt.Sprintf("%q (status %q, login_provider %q)", member.GetUserId(), status, member.GetLoginProvider()))
	}
	return strings.Join(descriptions, ", ")
}
