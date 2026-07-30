// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func validateModelForWrite(data Model) diag.Diagnostics {
	var diags diag.Diagnostics
	validateRequiredString(data.Name, path.Root("name"), "Skill Name", 1, 255, &diags)
	validateRequiredString(data.Description, path.Root("description"), "Skill Description", 1, 500, &diags)
	validateRequiredString(data.Prompt, path.Root("prompt"), "Skill Prompt", 1, 20000, &diags)
	if data.Command.IsUnknown() {
		diags.AddAttributeError(path.Root("command"), "Unknown Skill Command", "command must be known before applying the skill.")
	} else if !data.Command.IsNull() {
		validateStringValue(data.Command.ValueString(), path.Root("command"), "Skill Command", 1, 50, &diags)
		if !commandPattern.MatchString(data.Command.ValueString()) {
			diags.AddAttributeError(path.Root("command"), "Invalid Skill Command", "command must contain only ASCII letters, digits, underscores, and hyphens.")
		}
	}
	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		validateUUID(data.ID.ValueString(), path.Root("id"), "Skill ID", &diags)
	}
	return diags
}

func validateRequiredString(value types.String, attrPath path.Path, label string, minLength, maxLength int, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		diags.AddAttributeError(attrPath, "Unknown "+label, fmt.Sprintf("%s must be known before applying the skill.", attrPath.String()))
		return
	}
	if value.IsNull() {
		diags.AddAttributeError(attrPath, "Missing "+label, fmt.Sprintf("Set %s before applying the skill.", attrPath.String()))
		return
	}
	validateStringValue(value.ValueString(), attrPath, label, minLength, maxLength, diags)
}

func validateStringValue(value string, attrPath path.Path, label string, minLength, maxLength int, diags *diag.Diagnostics) {
	length := utf8.RuneCountInString(value)
	if length < minLength || length > maxLength {
		diags.AddAttributeError(attrPath, "Invalid "+label+" Length", fmt.Sprintf("%s must contain between %d and %d characters.", attrPath.String(), minLength, maxLength))
	}
}

func validateUUID(value string, attrPath path.Path, label string, diags *diag.Diagnostics) {
	if _, err := uuid.Parse(value); err != nil {
		diags.AddAttributeError(attrPath, "Invalid "+label, fmt.Sprintf("%s must be a valid UUID.", attrPath.String()))
	}
}
