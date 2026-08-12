// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"fmt"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func createPromptRequest(data Model) (*v1.CreatePromptRequest, diag.Diagnostics) {
	diags := validateModelForWrite(data)
	if diags.HasError() {
		return nil, diags
	}
	req := &v1.CreatePromptRequest{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		Prompt:      data.Prompt.ValueString(),
		IsSkill:     true,
		IsTemplate:  false,
	}
	if !data.Command.IsNull() {
		req.IsCommand = true
		req.Command = data.Command.ValueString()
	}
	return req, diags
}

func updatePromptRequest(data Model) (*v1.UpdatePromptRequest, diag.Diagnostics) {
	diags := validateModelForWrite(data)
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		diags.AddAttributeError(path.Root("id"), "Missing Skill ID", "The skill ID must be known before updating the skill.")
	}
	if diags.HasError() {
		return nil, diags
	}

	name := data.Name.ValueString()
	description := data.Description.ValueString()
	prompt := data.Prompt.ValueString()
	isSkill := true
	isTemplate := false
	isCommand := !data.Command.IsNull()
	req := &v1.UpdatePromptRequest{
		PromptId: data.ID.ValueString(),
		Metadata: &v1.UpdatePromptRequest_Metadata{
			Name:        &name,
			Description: &description,
		},
		Spec: &v1.UpdatePromptRequest_Spec{
			Prompt:     &prompt,
			IsSkill:    &isSkill,
			IsTemplate: &isTemplate,
			IsCommand:  &isCommand,
		},
	}
	if isCommand {
		command := data.Command.ValueString()
		req.Spec.Command = &command
	}
	return req, diags
}

func modelFromPrompt(remote *v1.Prompt, expectedID, authenticatedOrganizationID string, includePrompt bool) (Model, diag.Diagnostics) {
	var data Model
	var diags diag.Diagnostics
	if remote == nil {
		diags.AddError("Missing Ona Skill", "The Ona API returned an empty Prompt object.")
		return data, diags
	}

	id := remote.GetId()
	if _, err := uuid.Parse(id); err != nil {
		diags.AddError("Invalid Ona Skill ID", "The Ona API returned a skill with an invalid or empty UUID.")
	}
	if expectedID != "" && id != expectedID {
		diags.AddError("Ona Skill ID Mismatch", fmt.Sprintf("The Ona API returned skill ID %q while reading skill ID %q.", id, expectedID))
	}
	metadata := remote.GetMetadata()
	if metadata == nil {
		diags.AddError("Missing Ona Skill Metadata", "The Ona API returned a skill without metadata.")
		return data, diags
	}
	if metadata.GetOrganizationId() == "" || metadata.GetOrganizationId() != authenticatedOrganizationID {
		diags.AddError("Ona Skill Organization Mismatch", fmt.Sprintf("The Ona API returned organization %q, but the configured provider token is scoped to organization %q.", metadata.GetOrganizationId(), authenticatedOrganizationID))
	}
	spec := remote.GetSpec()
	if spec == nil {
		diags.AddError("Missing Ona Skill Specification", "The Ona API returned a skill without a specification.")
		return data, diags
	}
	if !spec.GetIsSkill() {
		diags.AddError("Prompt Is Not an Ona Skill", fmt.Sprintf("Prompt %q does not have is_skill enabled and cannot be managed as ona_skill.", id))
	}
	if spec.GetIsTemplate() {
		diags.AddError("Unsupported Hybrid Ona Skill", fmt.Sprintf("Prompt %q is both a skill and a template and cannot be managed as ona_skill.", id))
	}

	validateStringValue(metadata.GetName(), path.Root("name"), "Skill Name", 1, 255, &diags)
	validateStringValue(metadata.GetDescription(), path.Root("description"), "Skill Description", 1, 500, &diags)
	if includePrompt {
		validateStringValue(spec.GetPrompt(), path.Root("prompt"), "Skill Prompt", 1, 20000, &diags)
	}

	command := types.StringNull()
	if spec.GetIsCommand() {
		validateStringValue(spec.GetCommand(), path.Root("command"), "Skill Command", 1, 50, &diags)
		if !commandPattern.MatchString(spec.GetCommand()) {
			diags.AddAttributeError(path.Root("command"), "Invalid Skill Command", "The Ona API returned a command containing unsupported characters.")
		}
		command = types.StringValue(spec.GetCommand())
	} else if spec.GetCommand() != "" {
		diags.AddError("Inconsistent Ona Skill Command", fmt.Sprintf("Prompt %q has is_command disabled but retains command %q. Recreate it before managing it with ona_skill.", id, spec.GetCommand()))
	}
	if diags.HasError() {
		return data, diags
	}

	data.ID = types.StringValue(id)
	data.Name = types.StringValue(metadata.GetName())
	data.Description = types.StringValue(metadata.GetDescription())
	if includePrompt {
		data.Prompt = types.StringValue(spec.GetPrompt())
	} else {
		data.Prompt = types.StringNull()
	}
	data.Command = command
	return data, diags
}
