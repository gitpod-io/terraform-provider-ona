// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type InsightsIdentityModel struct {
	ProjectID types.String `tfsdk:"project_id"`
}

func (r *InsightsResource) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: map[string]identityschema.Attribute{"project_id": identityschema.StringAttribute{RequiredForImport: true, Description: "Project ID whose Insights status is managed."}}}
}
