// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ProjectRoleAssignmentIdentityModel struct {
	ProjectID types.String `tfsdk:"project_id"`
	GroupID   types.String `tfsdk:"group_id"`
}

func (r *ProjectRoleAssignmentResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: map[string]identityschema.Attribute{
		"project_id": identityschema.StringAttribute{RequiredForImport: true, Description: "Project ID receiving the group access."},
		"group_id":   identityschema.StringAttribute{RequiredForImport: true, Description: "Group ID receiving access to the project."},
	}}
}
