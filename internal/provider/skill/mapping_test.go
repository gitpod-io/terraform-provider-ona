// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/testing/protocmp"
)

const (
	testSkillID          = "11111111-1111-4111-8111-111111111111"
	testOrganizationID   = "22222222-2222-4222-8222-222222222222"
	testOtherSkillID     = "33333333-3333-4333-8333-333333333333"
	testOtherOrgID       = "44444444-4444-4444-8444-444444444444"
	testSkillName        = "Security review"
	testSkillDescription = "Review changes for security issues."
	testSkillPrompt      = "Follow the organization security checklist."
)

func TestCreatePromptRequest(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Request *v1.CreatePromptRequest
		Errors  []string
	}
	tests := []struct {
		Name     string
		Model    Model
		Expected Expectation
	}{
		{
			Name:  "skill_without_command",
			Model: validModel(types.StringNull()),
			Expected: Expectation{Request: &v1.CreatePromptRequest{
				Name: testSkillName, Description: testSkillDescription, Prompt: testSkillPrompt, IsSkill: true,
			}},
		},
		{
			Name:  "skill_with_command",
			Model: validModel(types.StringValue("security-review")),
			Expected: Expectation{Request: &v1.CreatePromptRequest{
				Name: testSkillName, Description: testSkillDescription, Prompt: testSkillPrompt, IsSkill: true, IsCommand: true, Command: "security-review",
			}},
		},
		{Name: "unknown_prompt", Model: func() Model { m := validModel(types.StringNull()); m.Prompt = types.StringUnknown(); return m }(), Expected: Expectation{Errors: []string{"Unknown Skill Prompt"}}},
		{Name: "unknown_command", Model: validModel(types.StringUnknown()), Expected: Expectation{Errors: []string{"Unknown Skill Command"}}},
		{Name: "invalid_command", Model: validModel(types.StringValue("security review")), Expected: Expectation{Errors: []string{"Invalid Skill Command"}}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			request, diags := createPromptRequest(tc.Model)
			got := Expectation{Request: request, Errors: diagnosticSummaries(diags)}
			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("createPromptRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdatePromptRequest(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Request *v1.UpdatePromptRequest
		Errors  []string
	}
	trueValue := true
	falseValue := false
	name := testSkillName
	description := testSkillDescription
	prompt := testSkillPrompt
	command := "security-review"
	tests := []struct {
		Name     string
		Model    Model
		Expected Expectation
	}{
		{
			Name:  "without_command_omits_command_pointer",
			Model: validModel(types.StringNull()),
			Expected: Expectation{Request: &v1.UpdatePromptRequest{
				PromptId: testSkillID,
				Metadata: &v1.UpdatePromptRequest_Metadata{Name: &name, Description: &description},
				Spec:     &v1.UpdatePromptRequest_Spec{Prompt: &prompt, IsTemplate: &falseValue, IsCommand: &falseValue, IsSkill: &trueValue},
			}},
		},
		{
			Name:  "with_command_sets_command_pointer",
			Model: validModel(types.StringValue(command)),
			Expected: Expectation{Request: &v1.UpdatePromptRequest{
				PromptId: testSkillID,
				Metadata: &v1.UpdatePromptRequest_Metadata{Name: &name, Description: &description},
				Spec:     &v1.UpdatePromptRequest_Spec{Prompt: &prompt, IsTemplate: &falseValue, IsCommand: &trueValue, Command: &command, IsSkill: &trueValue},
			}},
		},
		{Name: "missing_id", Model: func() Model { m := validModel(types.StringNull()); m.ID = types.StringNull(); return m }(), Expected: Expectation{Errors: []string{"Missing Skill ID"}}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			request, diags := updatePromptRequest(tc.Model)
			got := Expectation{Request: request, Errors: diagnosticSummaries(diags)}
			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("updatePromptRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestModelFromPrompt(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Model  Model
		Errors []string
	}
	tests := []struct {
		Name          string
		Prompt        *v1.Prompt
		ExpectedID    string
		Organization  string
		IncludePrompt bool
		Expected      Expectation
	}{
		{Name: "ordinary_skill", Prompt: validPrompt(false, ""), ExpectedID: testSkillID, Organization: testOrganizationID, IncludePrompt: true, Expected: Expectation{Model: validModel(types.StringNull())}},
		{Name: "command_skill", Prompt: validPrompt(true, "security-review"), ExpectedID: testSkillID, Organization: testOrganizationID, IncludePrompt: true, Expected: Expectation{Model: validModel(types.StringValue("security-review"))}},
		{Name: "identity_only_omits_prompt", Prompt: validPrompt(false, ""), ExpectedID: testSkillID, Organization: testOrganizationID, IncludePrompt: false, Expected: Expectation{Model: func() Model { m := validModel(types.StringNull()); m.Prompt = types.StringNull(); return m }()}},
		{Name: "missing_prompt", Prompt: nil, ExpectedID: testSkillID, Organization: testOrganizationID, IncludePrompt: true, Expected: Expectation{Errors: []string{"Missing Ona Skill"}}},
		{Name: "mismatched_id", Prompt: validPrompt(false, ""), ExpectedID: testOtherSkillID, Organization: testOrganizationID, IncludePrompt: true, Expected: Expectation{Errors: []string{"Ona Skill ID Mismatch"}}},
		{Name: "wrong_organization", Prompt: validPrompt(false, ""), ExpectedID: testSkillID, Organization: testOtherOrgID, IncludePrompt: true, Expected: Expectation{Errors: []string{"Ona Skill Organization Mismatch"}}},
		{Name: "not_a_skill", Prompt: func() *v1.Prompt { p := validPrompt(false, ""); p.Spec.IsSkill = false; return p }(), ExpectedID: testSkillID, Organization: testOrganizationID, IncludePrompt: true, Expected: Expectation{Errors: []string{"Prompt Is Not an Ona Skill"}}},
		{Name: "hybrid_template", Prompt: func() *v1.Prompt { p := validPrompt(false, ""); p.Spec.IsTemplate = true; return p }(), ExpectedID: testSkillID, Organization: testOrganizationID, IncludePrompt: true, Expected: Expectation{Errors: []string{"Unsupported Hybrid Ona Skill"}}},
		{Name: "stale_command", Prompt: validPrompt(false, "reserved"), ExpectedID: testSkillID, Organization: testOrganizationID, IncludePrompt: true, Expected: Expectation{Errors: []string{"Inconsistent Ona Skill Command"}}},
		{Name: "enabled_empty_command", Prompt: validPrompt(true, ""), ExpectedID: testSkillID, Organization: testOrganizationID, IncludePrompt: true, Expected: Expectation{Errors: []string{"Invalid Skill Command Length", "Invalid Skill Command"}}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			model, diags := modelFromPrompt(tc.Prompt, tc.ExpectedID, tc.Organization, tc.IncludePrompt)
			got := Expectation{Model: model, Errors: diagnosticSummaries(diags)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("modelFromPrompt() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func validModel(command types.String) Model {
	return Model{
		ID:          types.StringValue(testSkillID),
		Name:        types.StringValue(testSkillName),
		Description: types.StringValue(testSkillDescription),
		Prompt:      types.StringValue(testSkillPrompt),
		Command:     command,
	}
}

func validPrompt(isCommand bool, command string) *v1.Prompt {
	return &v1.Prompt{
		Id: testSkillID,
		Metadata: &v1.PromptMetadata{
			OrganizationId: testOrganizationID,
			Name:           testSkillName,
			Description:    testSkillDescription,
		},
		Spec: &v1.PromptSpec{
			Prompt:    testSkillPrompt,
			IsCommand: isCommand,
			Command:   command,
			IsSkill:   true,
		},
	}
}

func diagnosticSummaries(diags diag.Diagnostics) []string {
	if len(diags) == 0 {
		return nil
	}
	result := make([]string, 0, len(diags))
	for _, diagnostic := range diags {
		if diagnostic.Severity() == diag.SeverityError {
			result = append(result, diagnostic.Summary())
		}
	}
	return result
}
