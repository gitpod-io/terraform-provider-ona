// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type RunnerRoleAssignmentIdentityModel struct {
	RunnerID types.String `tfsdk:"runner_id"`
	GroupID  types.String `tfsdk:"group_id"`
}

func (r *RunnerRoleAssignmentResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: map[string]identityschema.Attribute{
		"runner_id": identityschema.StringAttribute{RequiredForImport: true, Description: "Runner ID receiving the group access."},
		"group_id":  identityschema.StringAttribute{RequiredForImport: true, Description: "Group ID receiving access to the runner."},
	}}
}
