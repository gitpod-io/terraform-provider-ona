// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/proto"
)

type automationRoleAssignmentDiagnostic struct {
	Summary string
	Detail  string
}

func TestAutomationRoleMapping(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		RoleToAPI     map[string]v1.ResourceRole
		APIToRole     map[v1.ResourceRole]string
		SupportedRole []string
	}

	expected := Expectation{
		RoleToAPI: map[string]v1.ResourceRole{
			"admin":    v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_ADMIN,
			"executor": v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR,
			"viewer":   v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_VIEWER,
		},
		APIToRole: map[v1.ResourceRole]string{
			v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_ADMIN:    "admin",
			v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR: "executor",
			v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_VIEWER:   "viewer",
		},
		SupportedRole: []string{"admin", "executor", "viewer"},
	}

	got := Expectation{
		RoleToAPI:     automationRoleToAPI,
		APIToRole:     apiToAutomationRole,
		SupportedRole: supportedAutomationRoles(),
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("Automation role mapping mismatch (-want +got):\n%s", diff)
	}
}

func TestAutomationRoleAssignmentModelFromAPI(t *testing.T) {
	t.Parallel()

	type Result struct {
		ID           string
		AutomationID string
		GroupID      string
		Role         string
	}
	type Expectation struct {
		Result Result
		Err    string
	}

	valid := &v1.RoleAssignment{
		Id:           "assignment-id",
		GroupId:      "group-id",
		ResourceId:   "automation-id",
		ResourceType: v1.ResourceType_RESOURCE_TYPE_WORKFLOW,
		ResourceRole: v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR,
	}
	tests := []struct {
		Name       string
		Assignment *v1.RoleAssignment
		Expected   Expectation
	}{
		{
			Name:       "valid_executor",
			Assignment: valid,
			Expected: Expectation{Result: Result{
				ID:           "assignment-id",
				AutomationID: "automation-id",
				GroupID:      "group-id",
				Role:         "executor",
			}},
		},
		{Name: "missing_assignment", Expected: Expectation{Err: "role assignment is missing"}},
		{
			Name:       "missing_id",
			Assignment: cloneAutomationRoleAssignment(valid, func(a *v1.RoleAssignment) { a.Id = "" }),
			Expected:   Expectation{Err: "role assignment ID is empty"},
		},
		{
			Name:       "missing_group_id",
			Assignment: cloneAutomationRoleAssignment(valid, func(a *v1.RoleAssignment) { a.GroupId = "" }),
			Expected:   Expectation{Err: "role assignment group ID is empty"},
		},
		{
			Name:       "missing_automation_id",
			Assignment: cloneAutomationRoleAssignment(valid, func(a *v1.RoleAssignment) { a.ResourceId = "" }),
			Expected:   Expectation{Err: "role assignment Automation ID is empty"},
		},
		{
			Name: "wrong_resource_type",
			Assignment: cloneAutomationRoleAssignment(valid, func(a *v1.RoleAssignment) {
				a.ResourceType = v1.ResourceType_RESOURCE_TYPE_PROJECT
			}),
			Expected: Expectation{Err: "role assignment resource type is RESOURCE_TYPE_PROJECT, expected RESOURCE_TYPE_WORKFLOW"},
		},
		{
			Name: "internal_workflow_user_role",
			Assignment: cloneAutomationRoleAssignment(valid, func(a *v1.RoleAssignment) {
				a.ResourceRole = v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_USER
			}),
			Expected: Expectation{Err: "role assignment has unsupported Automation role RESOURCE_ROLE_WORKFLOW_USER"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			model, err := automationRoleAssignmentModelFromAPI(tc.Assignment)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Result = Result{
					ID:           model.ID.ValueString(),
					AutomationID: model.AutomationID.ValueString(),
					GroupID:      model.GroupID.ValueString(),
					Role:         model.Role.ValueString(),
				}
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("automationRoleAssignmentModelFromAPI() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMatchesAutomationRoleAssignment(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Matches bool
	}

	valid := &v1.RoleAssignment{
		Id:           "assignment-id",
		GroupId:      "group-id",
		ResourceId:   "automation-id",
		ResourceType: v1.ResourceType_RESOURCE_TYPE_WORKFLOW,
		ResourceRole: v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_VIEWER,
	}
	tests := []struct {
		Name       string
		Assignment *v1.RoleAssignment
		Expected   Expectation
	}{
		{Name: "exact_match", Assignment: valid, Expected: Expectation{Matches: true}},
		{Name: "nil_assignment", Expected: Expectation{}},
		{
			Name:       "different_group",
			Assignment: cloneAutomationRoleAssignment(valid, func(a *v1.RoleAssignment) { a.GroupId = "other-group" }),
			Expected:   Expectation{},
		},
		{
			Name: "different_resource_type",
			Assignment: cloneAutomationRoleAssignment(valid, func(a *v1.RoleAssignment) {
				a.ResourceType = v1.ResourceType_RESOURCE_TYPE_PROJECT
			}),
			Expected: Expectation{},
		},
		{
			Name:       "different_automation",
			Assignment: cloneAutomationRoleAssignment(valid, func(a *v1.RoleAssignment) { a.ResourceId = "other-automation" }),
			Expected:   Expectation{},
		},
		{
			Name: "different_role",
			Assignment: cloneAutomationRoleAssignment(valid, func(a *v1.RoleAssignment) {
				a.ResourceRole = v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_ADMIN
			}),
			Expected: Expectation{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{Matches: matchesAutomationRoleAssignment(
				tc.Assignment,
				"automation-id",
				"group-id",
				v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_VIEWER,
			)}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("matchesAutomationRoleAssignment() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSplitAutomationRoleAssignmentImportID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Parts       []string
		Diagnostics []automationRoleAssignmentDiagnostic
	}
	tests := []struct {
		Name     string
		ID       string
		Expected Expectation
	}{
		{
			Name: "valid",
			ID:   "automation-id/group-id/executor",
			Expected: Expectation{
				Parts: []string{"automation-id", "group-id", "executor"},
			},
		},
		{
			Name: "missing_component",
			ID:   "automation-id/group-id",
			Expected: Expectation{Diagnostics: []automationRoleAssignmentDiagnostic{{
				Summary: "Invalid Import ID",
				Detail:  "Expected import ID format: automation_id/group_id/role.",
			}}},
		},
		{
			Name: "extra_component",
			ID:   "automation-id/group-id/executor/extra",
			Expected: Expectation{Diagnostics: []automationRoleAssignmentDiagnostic{{
				Summary: "Invalid Import ID",
				Detail:  "Expected import ID format: automation_id/group_id/role.",
			}}},
		},
		{
			Name: "empty_component",
			ID:   "automation-id//executor",
			Expected: Expectation{Diagnostics: []automationRoleAssignmentDiagnostic{{
				Summary: "Invalid Import ID",
				Detail:  "Expected import ID format: automation_id/group_id/role.",
			}}},
		},
		{
			Name: "unsupported_role",
			ID:   "automation-id/group-id/user",
			Expected: Expectation{Diagnostics: []automationRoleAssignmentDiagnostic{{
				Summary: "Unsupported Automation Role",
				Detail:  "Unsupported Automation role \"user\". Supported values are: admin, executor, viewer.",
			}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			parts, diags := splitAutomationRoleAssignmentImportID(tc.ID)
			got := Expectation{
				Parts:       parts,
				Diagnostics: automationRoleAssignmentDiagnostics(diags),
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("splitAutomationRoleAssignmentImportID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateAutomationRoleAssignmentIdentity(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Diagnostics []automationRoleAssignmentDiagnostic
	}
	tests := []struct {
		Name     string
		Identity AutomationRoleAssignmentIdentityModel
		Expected Expectation
	}{
		{
			Name: "valid",
			Identity: AutomationRoleAssignmentIdentityModel{
				AutomationID: types.StringValue("automation-id"),
				GroupID:      types.StringValue("group-id"),
				Role:         types.StringValue("admin"),
			},
			Expected: Expectation{},
		},
		{
			Name: "empty_automation_id",
			Identity: AutomationRoleAssignmentIdentityModel{
				AutomationID: types.StringValue(""),
				GroupID:      types.StringValue("group-id"),
				Role:         types.StringValue("admin"),
			},
			Expected: Expectation{Diagnostics: []automationRoleAssignmentDiagnostic{{
				Summary: "Invalid Automation Role Assignment Identity",
				Detail:  "automation_id must be a non-empty string.",
			}}},
		},
		{
			Name: "whitespace_group_id",
			Identity: AutomationRoleAssignmentIdentityModel{
				AutomationID: types.StringValue("automation-id"),
				GroupID:      types.StringValue("  "),
				Role:         types.StringValue("admin"),
			},
			Expected: Expectation{Diagnostics: []automationRoleAssignmentDiagnostic{{
				Summary: "Invalid Automation Role Assignment Identity",
				Detail:  "group_id must be a non-empty string.",
			}}},
		},
		{
			Name: "null_role",
			Identity: AutomationRoleAssignmentIdentityModel{
				AutomationID: types.StringValue("automation-id"),
				GroupID:      types.StringValue("group-id"),
				Role:         types.StringNull(),
			},
			Expected: Expectation{Diagnostics: []automationRoleAssignmentDiagnostic{{
				Summary: "Invalid Automation Role Assignment Identity",
				Detail:  "role must be a non-empty string.",
			}}},
		},
		{
			Name: "unsupported_role",
			Identity: AutomationRoleAssignmentIdentityModel{
				AutomationID: types.StringValue("automation-id"),
				GroupID:      types.StringValue("group-id"),
				Role:         types.StringValue("user"),
			},
			Expected: Expectation{Diagnostics: []automationRoleAssignmentDiagnostic{{
				Summary: "Unsupported Automation Role",
				Detail:  "Unsupported Automation role \"user\". Supported values are: admin, executor, viewer.",
			}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateAutomationRoleAssignmentIdentity(tc.Identity, &diags)
			got := Expectation{Diagnostics: automationRoleAssignmentDiagnostics(diags)}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateAutomationRoleAssignmentIdentity() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func cloneAutomationRoleAssignment(source *v1.RoleAssignment, mutate func(*v1.RoleAssignment)) *v1.RoleAssignment {
	result := proto.CloneOf(source)
	mutate(result)
	return result
}

func automationRoleAssignmentDiagnostics(diags diag.Diagnostics) []automationRoleAssignmentDiagnostic {
	if len(diags) == 0 {
		return nil
	}
	result := make([]automationRoleAssignmentDiagnostic, 0, len(diags))
	for _, diagnostic := range diags {
		result = append(result, automationRoleAssignmentDiagnostic{
			Summary: diagnostic.Summary(),
			Detail:  diagnostic.Detail(),
		})
	}
	return result
}
