// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const maxUserFilterIDs = 25

func userCollectionFilter(data UserCollectionDataSourceModel, diags *diag.Diagnostics) *v1.ListMembersRequest_Filter {
	filter := &v1.ListMembersRequest_Filter{}

	if data.Search.IsUnknown() {
		diags.AddAttributeError(path.Root("search"), "Unknown User Search", "search must be known before reading the data source.")
	} else if !data.Search.IsNull() {
		filter.Search = strings.TrimSpace(data.Search.ValueString())
		if utf8.RuneCountInString(data.Search.ValueString()) > 256 {
			diags.AddAttributeError(path.Root("search"), "User Search Is Too Long", "search must not exceed 256 characters.")
		}
	}

	statusNames := collectionStringSet(data.Statuses, path.Root("statuses"), -1, false, diags)
	for _, status := range statusNames {
		value, ok := userStatusToAPI(status)
		if !ok {
			diags.AddAttributeError(path.Root("statuses"), "Invalid User Status", fmt.Sprintf("Unsupported status %q. Use active, suspended, or left.", status))
			continue
		}
		filter.Statuses = append(filter.Statuses, value)
	}

	roleNames := collectionStringSet(data.Roles, path.Root("roles"), -1, false, diags)
	for _, role := range roleNames {
		value, ok := organizationRoleToAPI(role)
		if !ok {
			diags.AddAttributeError(path.Root("roles"), "Invalid Organization Role", fmt.Sprintf("Unsupported role %q. Use admin or member.", role))
			continue
		}
		filter.Roles = append(filter.Roles, value)
	}

	filter.UserIds = collectionStringSet(data.UserIDs, path.Root("user_ids"), maxUserFilterIDs, true, diags)
	if diags.HasError() {
		return nil
	}
	if filter.Search == "" && len(filter.Statuses) == 0 && len(filter.Roles) == 0 && len(filter.UserIds) == 0 {
		return nil
	}
	return filter
}

func collectionStringSet(value types.Set, p path.Path, maxItems int, validateUUID bool, diags *diag.Diagnostics) []string {
	if value.IsNull() {
		return nil
	}
	if value.IsUnknown() {
		diags.AddAttributeError(p, "Unknown User Filter", "This filter must be known before reading the data source.")
		return nil
	}

	values := make([]string, 0, len(value.Elements()))
	for _, element := range value.Elements() {
		stringValue, ok := element.(types.String)
		if !ok {
			diags.AddAttributeError(p, "Invalid User Filter", "This filter must contain only string values.")
			continue
		}
		if stringValue.IsNull() || stringValue.IsUnknown() {
			diags.AddAttributeError(p, "Unknown User Filter", "Every filter value must be known before reading the data source.")
			continue
		}
		values = append(values, stringValue.ValueString())
	}

	sort.Strings(values)
	if maxItems >= 0 && len(values) > maxItems {
		diags.AddAttributeError(p, "Too Many User Filter Values", fmt.Sprintf("The Ona API accepts at most %d values.", maxItems))
	}
	if validateUUID {
		for _, value := range values {
			if _, err := uuid.Parse(value); err != nil {
				diags.AddAttributeError(p, "Invalid User Filter UUID", fmt.Sprintf("Value %q is not a valid UUID.", value))
			}
		}
	}
	return values
}

func userModelFromMember(member *v1.OrganizationMember) (UserModel, error) {
	if member == nil {
		return UserModel{}, fmt.Errorf("the Ona API returned an empty organization member")
	}
	userID := member.GetUserId()
	if _, err := uuid.Parse(userID); err != nil {
		return UserModel{}, fmt.Errorf("the Ona API returned invalid user ID %q: %w", userID, err)
	}
	status, ok := userStatusFromAPI(member.GetStatus())
	if !ok {
		return UserModel{}, fmt.Errorf("the Ona API returned unsupported user status %q", member.GetStatus())
	}
	role, ok := organizationRoleFromAPI(member.GetRole())
	if !ok {
		return UserModel{}, fmt.Errorf("the Ona API returned unsupported organization role %q", member.GetRole())
	}
	memberSince, err := timestampValue(member.GetMemberSince())
	if err != nil {
		return UserModel{}, fmt.Errorf("the Ona API returned invalid member_since for user %q: %w", userID, err)
	}

	return UserModel{
		ID:            types.StringValue(userID),
		UserID:        types.StringValue(userID),
		Name:          types.StringValue(member.GetFullName()),
		Email:         optionalStringValue(member.GetEmail()),
		Status:        types.StringValue(status),
		Role:          types.StringValue(role),
		MemberSince:   memberSince,
		LoginProvider: optionalStringValue(member.GetLoginProvider()),
	}, nil
}

func userModelFromResponses(apiUser *v1.User, member *v1.OrganizationMember) (UserModel, error) {
	if apiUser == nil {
		return UserModel{}, fmt.Errorf("the Ona API returned an empty user")
	}
	if member == nil {
		return UserModel{}, fmt.Errorf("the Ona API returned an empty organization member")
	}
	if apiUser.GetId() != member.GetUserId() {
		return UserModel{}, fmt.Errorf("the Ona API returned mismatched user IDs %q and %q", apiUser.GetId(), member.GetUserId())
	}
	if apiUser.GetStatus() != member.GetStatus() {
		return UserModel{}, fmt.Errorf("the Ona API returned mismatched statuses for user %q", apiUser.GetId())
	}

	result, err := userModelFromMember(member)
	if err != nil {
		return UserModel{}, err
	}
	result.Name = types.StringValue(apiUser.GetName())
	result.Email = optionalStringValue(apiUser.GetEmail())
	return result, nil
}

func userStatusToAPI(value string) (v1.UserStatus, bool) {
	switch value {
	case "active":
		return v1.UserStatus_USER_STATUS_ACTIVE, true
	case "suspended":
		return v1.UserStatus_USER_STATUS_SUSPENDED, true
	case "left":
		return v1.UserStatus_USER_STATUS_LEFT, true
	default:
		return v1.UserStatus_USER_STATUS_UNSPECIFIED, false
	}
}

func userStatusFromAPI(value v1.UserStatus) (string, bool) {
	switch value {
	case v1.UserStatus_USER_STATUS_ACTIVE:
		return "active", true
	case v1.UserStatus_USER_STATUS_SUSPENDED:
		return "suspended", true
	case v1.UserStatus_USER_STATUS_LEFT:
		return "left", true
	default:
		return "", false
	}
}

func organizationRoleToAPI(value string) (v1.OrganizationRole, bool) {
	switch value {
	case "admin":
		return v1.OrganizationRole_ORGANIZATION_ROLE_ADMIN, true
	case "member":
		return v1.OrganizationRole_ORGANIZATION_ROLE_MEMBER, true
	default:
		return v1.OrganizationRole_ORGANIZATION_ROLE_UNSPECIFIED, false
	}
}

func organizationRoleFromAPI(value v1.OrganizationRole) (string, bool) {
	switch value {
	case v1.OrganizationRole_ORGANIZATION_ROLE_ADMIN:
		return "admin", true
	case v1.OrganizationRole_ORGANIZATION_ROLE_MEMBER:
		return "member", true
	default:
		return "", false
	}
}

func timestampValue(value *timestamppb.Timestamp) (types.String, error) {
	if value == nil {
		return types.StringNull(), nil
	}
	if err := value.CheckValid(); err != nil {
		return types.StringNull(), err
	}
	return types.StringValue(value.AsTime().UTC().Format(time.RFC3339Nano)), nil
}

func optionalStringValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}
