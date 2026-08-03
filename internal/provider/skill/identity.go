// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type IdentityModel struct {
	SkillID types.String `tfsdk:"skill_id"`
}

func (r *Resource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: map[string]identityschema.Attribute{
		"skill_id": identityschema.StringAttribute{
			RequiredForImport: true,
			Description:       "Skill ID.",
		},
	}}
}
