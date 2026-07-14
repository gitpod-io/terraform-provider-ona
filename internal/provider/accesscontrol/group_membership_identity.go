// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type GroupMembershipIdentityModel struct {
	GroupID          types.String `tfsdk:"group_id"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
}

func (r *GroupMembershipResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: map[string]identityschema.Attribute{
		"group_id":           identityschema.StringAttribute{RequiredForImport: true, Description: "Group ID."},
		"service_account_id": identityschema.StringAttribute{RequiredForImport: true, Description: "Service account member ID."},
	}}
}
