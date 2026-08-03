// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestListPromptsRequest(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Request *v1.ListPromptsRequest
	}
	tests := []struct {
		Name            string
		Config          listModel
		IncludeResource bool
		Expected        Expectation
	}{
		{
			Name:     "identity_only",
			Config:   emptyListModel(),
			Expected: Expectation{Request: &v1.ListPromptsRequest{Pagination: &v1.PaginationRequest{PageSize: 25, Token: "next"}, Filter: &v1.ListPromptsRequest_Filter{IsSkill: true, ExcludePromptContent: true}}},
		},
		{
			Name:            "full_resource_with_search",
			Config:          listModel{Search: types.StringValue("security"), Command: types.StringNull(), CommandPrefix: types.StringNull()},
			IncludeResource: true,
			Expected:        Expectation{Request: &v1.ListPromptsRequest{Pagination: &v1.PaginationRequest{PageSize: 25, Token: "next"}, Filter: &v1.ListPromptsRequest_Filter{IsSkill: true, Search: "security"}}},
		},
		{
			Name:   "exact_command_implies_enabled",
			Config: listModel{Search: types.StringNull(), Command: types.StringValue("security-review"), CommandPrefix: types.StringNull()},
			Expected: Expectation{Request: &v1.ListPromptsRequest{Pagination: &v1.PaginationRequest{PageSize: 25, Token: "next"}, Filter: &v1.ListPromptsRequest_Filter{
				IsSkill: true, IsCommand: true, Command: "security-review", ExcludePromptContent: true,
			}}},
		},
		{
			Name:   "command_prefix_implies_enabled",
			Config: listModel{Search: types.StringNull(), Command: types.StringNull(), CommandPrefix: types.StringValue("security")},
			Expected: Expectation{Request: &v1.ListPromptsRequest{Pagination: &v1.PaginationRequest{PageSize: 25, Token: "next"}, Filter: &v1.ListPromptsRequest_Filter{
				IsSkill: true, IsCommand: true, CommandPrefix: "security", ExcludePromptContent: true,
			}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{Request: listPromptsRequest(tc.Config, tc.IncludeResource, 25, "next")}
			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("listPromptsRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateListConfig(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Errors []string
	}
	tests := []struct {
		Name     string
		Config   listModel
		Expected Expectation
	}{
		{Name: "empty", Config: emptyListModel()},
		{Name: "valid_search", Config: listModel{Search: types.StringValue("security"), Command: types.StringNull(), CommandPrefix: types.StringNull()}},
		{Name: "unknown_search", Config: listModel{Search: types.StringUnknown(), Command: types.StringNull(), CommandPrefix: types.StringNull()}, Expected: Expectation{Errors: []string{"Unknown Skill Search"}}},
		{Name: "invalid_prefix", Config: listModel{Search: types.StringNull(), Command: types.StringNull(), CommandPrefix: types.StringValue("bad prefix")}, Expected: Expectation{Errors: []string{"Invalid Skill Command Prefix"}}},
		{Name: "conflicting_commands", Config: listModel{Search: types.StringNull(), Command: types.StringValue("exact"), CommandPrefix: types.StringValue("prefix")}, Expected: Expectation{Errors: []string{"Conflicting Skill Command Filters"}}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{Errors: diagnosticSummaries(validateListConfig(tc.Config))}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateListConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSkillDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Prompt   *v1.Prompt
		Expected string
	}{
		{Name: "name_and_id", Prompt: validPrompt(false, ""), Expected: "security_review_11111111_1111_4111_8111_111111111111"},
		{Name: "numeric_name", Prompt: func() *v1.Prompt { p := validPrompt(false, ""); p.Metadata.Name = "123 review"; return p }(), Expected: "skill_123_review_11111111_1111_4111_8111_111111111111"},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			if diff := cmp.Diff(tc.Expected, skillDisplayName(tc.Prompt)); diff != "" {
				t.Errorf("skillDisplayName() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func emptyListModel() listModel {
	return listModel{Search: types.StringNull(), Command: types.StringNull(), CommandPrefix: types.StringNull()}
}
