// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import "github.com/hashicorp/terraform-plugin-framework/types"

type UserModel struct {
	ID            types.String `tfsdk:"id"`
	UserID        types.String `tfsdk:"user_id"`
	Name          types.String `tfsdk:"name"`
	Email         types.String `tfsdk:"email"`
	Status        types.String `tfsdk:"status"`
	Role          types.String `tfsdk:"role"`
	MemberSince   types.String `tfsdk:"member_since"`
	LoginProvider types.String `tfsdk:"login_provider"`
}

type UserCollectionDataSourceModel struct {
	Search   types.String `tfsdk:"search"`
	Statuses types.Set    `tfsdk:"statuses"`
	Roles    types.Set    `tfsdk:"roles"`
	UserIDs  types.Set    `tfsdk:"user_ids"`
	Users    []UserModel  `tfsdk:"users"`
}
