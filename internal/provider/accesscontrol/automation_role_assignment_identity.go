// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type AutomationRoleAssignmentIdentityModel struct {
	AutomationID types.String `tfsdk:"automation_id"`
	GroupID      types.String `tfsdk:"group_id"`
	Role         types.String `tfsdk:"role"`
}

func (r *AutomationRoleAssignmentResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: map[string]identityschema.Attribute{
		"automation_id": identityschema.StringAttribute{RequiredForImport: true, Description: "Automation ID receiving the group access."},
		"group_id":      identityschema.StringAttribute{RequiredForImport: true, Description: "Group ID receiving access to the Automation."},
		"role":          identityschema.StringAttribute{RequiredForImport: true, Description: "Automation role name."},
	}}
}
