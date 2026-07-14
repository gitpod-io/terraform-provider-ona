// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type OrganizationRoleAssignmentIdentityModel struct {
	GroupID        types.String `tfsdk:"group_id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Role           types.String `tfsdk:"role"`
}

func (r *OrganizationRoleAssignmentResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: map[string]identityschema.Attribute{
		"group_id":        identityschema.StringAttribute{RequiredForImport: true, Description: "Group ID receiving the role."},
		"organization_id": identityschema.StringAttribute{RequiredForImport: true, Description: "Organization ID targeted by the role."},
		"role":            identityschema.StringAttribute{RequiredForImport: true, Description: "Organization role name."},
	}}
}
