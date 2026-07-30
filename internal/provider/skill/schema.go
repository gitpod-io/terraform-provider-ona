// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var commandPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func resourceSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Manages an organization-level Ona skill. Organization scope is resolved from the configured provider token. The token must have Prompt management permission and access to Ona Agents.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable UUID of the skill Prompt returned by Ona.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable skill name.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
			},
			"description": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Description of when and why agents should use the skill.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 500),
				},
			},
			"prompt": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Skill instructions. Use Terraform's `file()` or `templatefile()` to keep this Markdown content in the repository.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 20000),
				},
			},
			"command": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional slash-command name. It must be unique in the organization and contain only ASCII letters, digits, underscores, and hyphens. Adding or renaming a command updates in place; removing it replaces the skill because Ona cannot clear a stored command value.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 50),
					stringvalidator.RegexMatches(commandPattern, "command must contain only ASCII letters, digits, underscores, and hyphens"),
				},
				PlanModifiers: []planmodifier.String{
					replaceWhenCommandRemoved{},
				},
			},
		},
	}
}
