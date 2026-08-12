// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSkillDataSource(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	tests := []struct {
		Name    string
		Command string
	}{
		{Name: "skill_without_command"},
		{Name: "skill_with_command", Command: "security-review"},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			server := newSkillAPIServer(t)
			t.Cleanup(server.Close)
			server.service.seed(testSkillPrompt(skillTestID1, tc.Command))

			checks := []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("data.ona_skill.test", "skill_id", skillTestID1),
				resource.TestCheckResourceAttr("data.ona_skill.test", "id", skillTestID1),
				resource.TestCheckResourceAttr("data.ona_skill.test", "name", "Security review"),
				resource.TestCheckResourceAttr("data.ona_skill.test", "description", "Review security-sensitive changes."),
				resource.TestCheckResourceAttr("data.ona_skill.test", "prompt", "Follow the security checklist."),
			}
			if tc.Command == "" {
				checks = append(checks, resource.TestCheckNoResourceAttr("data.ona_skill.test", "command"))
			} else {
				checks = append(checks, resource.TestCheckResourceAttr("data.ona_skill.test", "command", tc.Command))
			}

			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config: testAccSkillDataSourceConfig(server.URL, skillTestID1),
					Check:  resource.ComposeAggregateTestCheckFunc(checks...),
				}},
			})
		})
	}
}

func TestAccSkillDataSourceDiagnostics(t *testing.T) {
	// not parallel: terraform-plugin-testing manages per-test Terraform workdirs and process state.
	tests := []struct {
		Name        string
		SkillID     string
		Seed        *v1.Prompt
		ExpectedErr string
	}{
		{Name: "invalid_uuid", SkillID: "not-a-uuid", ExpectedErr: "Invalid Skill ID"},
		{Name: "not_found", SkillID: skillTestID1, ExpectedErr: "Ona Skill Not Found"},
		{Name: "non_skill", SkillID: skillTestID1, Seed: func() *v1.Prompt {
			prompt := testSkillPrompt(skillTestID1, "")
			prompt.Spec.IsSkill = false
			return prompt
		}(), ExpectedErr: "Prompt Is Not an Ona Skill"},
		{Name: "hybrid_skill", SkillID: skillTestID1, Seed: func() *v1.Prompt {
			prompt := testSkillPrompt(skillTestID1, "")
			prompt.Spec.IsTemplate = true
			return prompt
		}(), ExpectedErr: "Unsupported Hybrid Ona Skill"},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			server := newSkillAPIServer(t)
			t.Cleanup(server.Close)
			if tc.Seed != nil {
				server.service.seed(tc.Seed)
			}

			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      testAccSkillDataSourceConfig(server.URL, tc.SkillID),
					ExpectError: regexp.MustCompile(tc.ExpectedErr),
				}},
			})
		})
	}
}

func testAccSkillDataSourceConfig(host, skillID string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %q
  token = "test-token"
}

data "ona_skill" "test" {
  skill_id = %q
}
`, host, skillID)
}

func testSkillPrompt(id, command string) *v1.Prompt {
	return &v1.Prompt{
		Id: id,
		Metadata: &v1.PromptMetadata{
			OrganizationId: skillTestOrgID,
			Name:           "Security review",
			Description:    "Review security-sensitive changes.",
		},
		Spec: &v1.PromptSpec{
			Prompt:    "Follow the security checklist.",
			IsSkill:   true,
			IsCommand: command != "",
			Command:   command,
		},
	}
}
