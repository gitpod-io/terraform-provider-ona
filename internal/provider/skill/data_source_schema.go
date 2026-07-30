// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"context"

	"github.com/google/uuid"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type uuidStringValidator struct{}

func (uuidStringValidator) Description(context.Context) string {
	return "value must be a UUID"
}

func (uuidStringValidator) MarkdownDescription(context.Context) string {
	return "Value must be a UUID."
}

func (uuidStringValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if _, err := uuid.Parse(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Skill ID", "skill_id must be a valid Prompt UUID.")
	}
}

func dataSourceSchema() datasourceschema.Schema {
	return datasourceschema.Schema{
		MarkdownDescription: "Reads one organization-level Ona skill by its Prompt UUID. Organization scope is resolved from the configured provider token.",
		Attributes: map[string]datasourceschema.Attribute{
			"skill_id": datasourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the existing Ona skill to read.",
				Validators: []validator.String{
					uuidStringValidator{},
				},
			},
			"id": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable UUID of the skill Prompt returned by Ona.",
			},
			"name": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Human-readable skill name.",
			},
			"description": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of when and why agents should use the skill.",
			},
			"prompt": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Complete skill instructions.",
			},
			"command": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Slash-command name when enabled, otherwise null.",
			},
		},
	}
}
