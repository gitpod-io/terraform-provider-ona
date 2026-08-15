// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"fmt"
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/proto"
)

type runnerRoleAssignmentDiagnostic struct {
	Summary string
	Detail  string
}

const (
	runnerRoleAssignmentTestRunnerID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	runnerRoleAssignmentTestGroupID  = "22222222-2222-4222-8222-222222222222"
)

func TestRunnerRoleAssignmentSchema(t *testing.T) {
	t.Parallel()

	type Attribute struct {
		Required      bool
		Computed      bool
		PlanModifiers []string
	}
	type Expectation struct {
		TypeName   string
		Attributes map[string]Attribute
		Errors     []string
	}

	resourceUnderTest := &RunnerRoleAssignmentResource{}
	var metadataResponse resource.MetadataResponse
	resourceUnderTest.Metadata(t.Context(), resource.MetadataRequest{ProviderTypeName: "ona"}, &metadataResponse)
	var schemaResponse resource.SchemaResponse
	resourceUnderTest.Schema(t.Context(), resource.SchemaRequest{}, &schemaResponse)

	got := Expectation{TypeName: metadataResponse.TypeName, Attributes: make(map[string]Attribute)}
	for _, diagnostic := range schemaResponse.Diagnostics.Errors() {
		got.Errors = append(got.Errors, diagnostic.Summary()+": "+diagnostic.Detail())
	}
	for name, attribute := range schemaResponse.Schema.Attributes {
		stringAttribute, ok := attribute.(resourceschema.StringAttribute)
		if !ok {
			got.Errors = append(got.Errors, fmt.Sprintf("attribute %s has type %T", name, attribute))
			continue
		}
		result := Attribute{Required: stringAttribute.Required, Computed: stringAttribute.Computed}
		for _, modifier := range stringAttribute.PlanModifiers {
			result.PlanModifiers = append(result.PlanModifiers, fmt.Sprintf("%T", modifier))
		}
		got.Attributes[name] = result
	}

	expected := Expectation{
		TypeName: "ona_runner_role_assignment",
		Attributes: map[string]Attribute{
			"id": {
				Computed:      true,
				PlanModifiers: []string{"stringplanmodifier.useStateForUnknownModifier"},
			},
			"runner_id": {
				Required:      true,
				PlanModifiers: []string{"stringplanmodifier.requiresReplaceIfModifier"},
			},
			"group_id": {
				Required:      true,
				PlanModifiers: []string{"stringplanmodifier.requiresReplaceIfModifier"},
			},
			"role": {
				Required:      true,
				PlanModifiers: []string{"stringplanmodifier.requiresReplaceIfModifier"},
			},
		},
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("RunnerRoleAssignmentResource schema mismatch (-want +got):\n%s", diff)
	}
}

func TestRunnerRoleAssignmentIdentitySchema(t *testing.T) {
	t.Parallel()

	type Attribute struct {
		RequiredForImport bool
		Description       string
	}
	type Expectation struct {
		Attributes map[string]Attribute
		Errors     []string
	}

	resourceUnderTest := &RunnerRoleAssignmentResource{}
	var response resource.IdentitySchemaResponse
	resourceUnderTest.IdentitySchema(t.Context(), resource.IdentitySchemaRequest{}, &response)

	got := Expectation{Attributes: make(map[string]Attribute)}
	for _, diagnostic := range response.Diagnostics.Errors() {
		got.Errors = append(got.Errors, diagnostic.Summary()+": "+diagnostic.Detail())
	}
	for name, attribute := range response.IdentitySchema.Attributes {
		stringAttribute, ok := attribute.(identityschema.StringAttribute)
		if !ok {
			got.Errors = append(got.Errors, fmt.Sprintf("attribute %s has type %T", name, attribute))
			continue
		}
		got.Attributes[name] = Attribute{
			RequiredForImport: stringAttribute.RequiredForImport,
			Description:       stringAttribute.Description,
		}
	}

	expected := Expectation{Attributes: map[string]Attribute{
		"runner_id": {RequiredForImport: true, Description: "Runner ID receiving the group access."},
		"group_id":  {RequiredForImport: true, Description: "Group ID receiving access to the runner."},
	}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("RunnerRoleAssignmentResource identity schema mismatch (-want +got):\n%s", diff)
	}
}

func TestRunnerRoleAssignmentImportState(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		RunnerID   string
		GroupID    string
		RoleIsNull bool
		Errors     []string
	}
	tests := []struct {
		Name       string
		LegacyID   string
		Identity   RunnerRoleAssignmentIdentityModel
		Structured bool
		Expected   Expectation
	}{
		{
			Name:     "legacy_string_id",
			LegacyID: runnerRoleAssignmentTestRunnerID + "/" + runnerRoleAssignmentTestGroupID,
			Expected: Expectation{RunnerID: runnerRoleAssignmentTestRunnerID, GroupID: runnerRoleAssignmentTestGroupID, RoleIsNull: true},
		},
		{
			Name:       "structured_identity",
			Structured: true,
			Identity: RunnerRoleAssignmentIdentityModel{
				RunnerID: types.StringValue(runnerRoleAssignmentTestRunnerID),
				GroupID:  types.StringValue(runnerRoleAssignmentTestGroupID),
			},
			Expected: Expectation{RunnerID: runnerRoleAssignmentTestRunnerID, GroupID: runnerRoleAssignmentTestGroupID, RoleIsNull: true},
		},
		{
			Name:     "malformed_legacy_id",
			LegacyID: "runner-id",
			Expected: Expectation{RoleIsNull: true, Errors: []string{
				"Invalid Import ID: Expected import ID format: runner_id/group_id.",
			}},
		},
		{
			Name:       "invalid_structured_identity",
			Structured: true,
			Identity: RunnerRoleAssignmentIdentityModel{
				RunnerID: types.StringValue(""),
				GroupID:  types.StringValue(runnerRoleAssignmentTestGroupID),
			},
			Expected: Expectation{RoleIsNull: true, Errors: []string{
				"Invalid Runner Role Assignment Identity: runner_id must be a non-empty string.",
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			resourceUnderTest := &RunnerRoleAssignmentResource{}
			var schemaResponse resource.SchemaResponse
			resourceUnderTest.Schema(t.Context(), resource.SchemaRequest{}, &schemaResponse)
			state := tfsdk.State{Schema: schemaResponse.Schema}
			emptyModel := RunnerRoleAssignmentModel{
				ID:       types.StringNull(),
				RunnerID: types.StringNull(),
				GroupID:  types.StringNull(),
				Role:     types.StringNull(),
			}

			var got Expectation
			for _, diagnostic := range state.Set(t.Context(), emptyModel).Errors() {
				got.Errors = append(got.Errors, diagnostic.Summary()+": "+diagnostic.Detail())
			}
			request := resource.ImportStateRequest{ID: tc.LegacyID}
			var responseIdentity *tfsdk.ResourceIdentity
			if tc.Structured {
				var identitySchemaResponse resource.IdentitySchemaResponse
				resourceUnderTest.IdentitySchema(t.Context(), resource.IdentitySchemaRequest{}, &identitySchemaResponse)
				identity := &tfsdk.ResourceIdentity{Schema: identitySchemaResponse.IdentitySchema}
				for _, diagnostic := range identity.Set(t.Context(), tc.Identity).Errors() {
					got.Errors = append(got.Errors, diagnostic.Summary()+": "+diagnostic.Detail())
				}
				request = resource.ImportStateRequest{Identity: identity}
				responseIdentity = &tfsdk.ResourceIdentity{Schema: identitySchemaResponse.IdentitySchema}
			}

			response := resource.ImportStateResponse{State: state, Identity: responseIdentity}
			if len(got.Errors) == 0 {
				resourceUnderTest.ImportState(t.Context(), request, &response)
				for _, diagnostic := range response.Diagnostics.Errors() {
					got.Errors = append(got.Errors, diagnostic.Summary()+": "+diagnostic.Detail())
				}
			}

			var imported RunnerRoleAssignmentModel
			for _, diagnostic := range response.State.Get(t.Context(), &imported).Errors() {
				got.Errors = append(got.Errors, diagnostic.Summary()+": "+diagnostic.Detail())
			}
			if !imported.RunnerID.IsNull() && !imported.RunnerID.IsUnknown() {
				got.RunnerID = imported.RunnerID.ValueString()
			}
			if !imported.GroupID.IsNull() && !imported.GroupID.IsUnknown() {
				got.GroupID = imported.GroupID.ValueString()
			}
			got.RoleIsNull = imported.Role.IsNull()

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("RunnerRoleAssignmentResource.ImportState() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRunnerRoleMapping(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		RoleToAPI     map[string]v1.ResourceRole
		APIToRole     map[v1.ResourceRole]string
		SupportedRole []string
	}

	expected := Expectation{
		RoleToAPI: map[string]v1.ResourceRole{
			"admin": v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN,
			"user":  v1.ResourceRole_RESOURCE_ROLE_RUNNER_USER,
		},
		APIToRole: map[v1.ResourceRole]string{
			v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN: "admin",
			v1.ResourceRole_RESOURCE_ROLE_RUNNER_USER:  "user",
		},
		SupportedRole: []string{"admin", "user"},
	}
	got := Expectation{
		RoleToAPI:     runnerRoleToAPI,
		APIToRole:     apiToRunnerRole,
		SupportedRole: supportedRunnerRoles(),
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("Runner role mapping mismatch (-want +got):\n%s", diff)
	}
}

func TestRunnerRoleAssignmentModelFromAPI(t *testing.T) {
	t.Parallel()

	type Result struct {
		ID       string
		RunnerID string
		GroupID  string
		Role     string
	}
	type Expectation struct {
		Result Result
		Err    string
	}

	valid := &v1.RoleAssignment{
		Id:           "assignment-id",
		GroupId:      "group-id",
		ResourceId:   "runner-id",
		ResourceType: v1.ResourceType_RESOURCE_TYPE_RUNNER,
		ResourceRole: v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN,
	}
	derivedRole := v1.ResourceRole_RESOURCE_ROLE_ORG_RUNNERS_ADMIN
	tests := []struct {
		Name       string
		Assignment *v1.RoleAssignment
		Expected   Expectation
	}{
		{
			Name:       "valid_admin",
			Assignment: valid,
			Expected: Expectation{Result: Result{
				ID:       "assignment-id",
				RunnerID: "runner-id",
				GroupID:  "group-id",
				Role:     "admin",
			}},
		},
		{Name: "missing_assignment", Expected: Expectation{Err: "role assignment is missing"}},
		{
			Name:       "missing_id",
			Assignment: cloneRunnerRoleAssignment(valid, func(a *v1.RoleAssignment) { a.Id = "" }),
			Expected:   Expectation{Err: "role assignment ID is empty"},
		},
		{
			Name:       "missing_group_id",
			Assignment: cloneRunnerRoleAssignment(valid, func(a *v1.RoleAssignment) { a.GroupId = "" }),
			Expected:   Expectation{Err: "role assignment group ID is empty"},
		},
		{
			Name:       "missing_runner_id",
			Assignment: cloneRunnerRoleAssignment(valid, func(a *v1.RoleAssignment) { a.ResourceId = "" }),
			Expected:   Expectation{Err: "role assignment runner ID is empty"},
		},
		{
			Name: "wrong_resource_type",
			Assignment: cloneRunnerRoleAssignment(valid, func(a *v1.RoleAssignment) {
				a.ResourceType = v1.ResourceType_RESOURCE_TYPE_WORKFLOW
			}),
			Expected: Expectation{Err: "role assignment resource type is RESOURCE_TYPE_WORKFLOW, expected RESOURCE_TYPE_RUNNER"},
		},
		{
			Name: "derived_assignment",
			Assignment: cloneRunnerRoleAssignment(valid, func(a *v1.RoleAssignment) {
				a.DerivedFromOrgRole = &derivedRole
			}),
			Expected: Expectation{Err: "role assignment is derived from organization role RESOURCE_ROLE_ORG_RUNNERS_ADMIN"},
		},
		{
			Name: "unsupported_runner_role",
			Assignment: cloneRunnerRoleAssignment(valid, func(a *v1.RoleAssignment) {
				a.ResourceRole = v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_VIEWER
			}),
			Expected: Expectation{Err: "role assignment has unsupported runner role RESOURCE_ROLE_WORKFLOW_VIEWER"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			model, err := runnerRoleAssignmentModelFromAPI(tc.Assignment)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Result = Result{
					ID:       model.ID.ValueString(),
					RunnerID: model.RunnerID.ValueString(),
					GroupID:  model.GroupID.ValueString(),
					Role:     model.Role.ValueString(),
				}
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("runnerRoleAssignmentModelFromAPI() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestClassifyRunnerRoleAssignments(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		DirectIDs  []string
		DerivedIDs []string
	}

	direct := &v1.RoleAssignment{
		Id:           "direct-id",
		GroupId:      "group-id",
		ResourceId:   "runner-id",
		ResourceType: v1.ResourceType_RESOURCE_TYPE_RUNNER,
		ResourceRole: v1.ResourceRole_RESOURCE_ROLE_RUNNER_USER,
	}
	derivedRole := v1.ResourceRole_RESOURCE_ROLE_ORG_RUNNERS_ADMIN
	derived := cloneRunnerRoleAssignment(direct, func(a *v1.RoleAssignment) {
		a.Id = "derived-id"
		a.DerivedFromOrgRole = &derivedRole
	})
	unspecifiedRole := v1.ResourceRole_RESOURCE_ROLE_UNSPECIFIED
	explicitlyDirect := cloneRunnerRoleAssignment(direct, func(a *v1.RoleAssignment) {
		a.Id = "explicitly-direct-id"
		a.DerivedFromOrgRole = &unspecifiedRole
	})
	assignments := []*v1.RoleAssignment{
		direct,
		derived,
		explicitlyDirect,
		nil,
		cloneRunnerRoleAssignment(direct, func(a *v1.RoleAssignment) { a.GroupId = "other-group" }),
		cloneRunnerRoleAssignment(direct, func(a *v1.RoleAssignment) { a.ResourceId = "other-runner" }),
		cloneRunnerRoleAssignment(direct, func(a *v1.RoleAssignment) {
			a.ResourceType = v1.ResourceType_RESOURCE_TYPE_WORKFLOW
		}),
	}

	matches := classifyRunnerRoleAssignments(assignments, "runner-id", "group-id")
	got := Expectation{
		DirectIDs:  runnerRoleAssignmentIDs(matches.direct),
		DerivedIDs: runnerRoleAssignmentIDs(matches.derived),
	}
	expected := Expectation{
		DirectIDs:  []string{"direct-id", "explicitly-direct-id"},
		DerivedIDs: []string{"derived-id"},
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("classifyRunnerRoleAssignments() mismatch (-want +got):\n%s", diff)
	}
}

func TestSelectCreatedRunnerRoleAssignment(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		ID  string
		Err string
	}
	assignment := func(id string, role v1.ResourceRole) *v1.RoleAssignment {
		return &v1.RoleAssignment{Id: id, ResourceRole: role}
	}
	tests := []struct {
		Name        string
		Assignments []*v1.RoleAssignment
		Expected    Expectation
	}{
		{
			Name: "single_matching_role",
			Assignments: []*v1.RoleAssignment{
				assignment("user-id", v1.ResourceRole_RESOURCE_ROLE_RUNNER_USER),
				assignment("admin-id", v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN),
			},
			Expected: Expectation{ID: "admin-id"},
		},
		{
			Name: "no_matching_role",
			Assignments: []*v1.RoleAssignment{
				assignment("user-id", v1.ResourceRole_RESOURCE_ROLE_RUNNER_USER),
			},
			Expected: Expectation{Err: "no direct assignment with role RESOURCE_ROLE_RUNNER_ADMIN was found"},
		},
		{
			Name: "multiple_matching_roles",
			Assignments: []*v1.RoleAssignment{
				assignment("z-id", v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN),
				assignment("a-id", v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN),
			},
			Expected: Expectation{Err: "multiple direct assignments with role RESOURCE_ROLE_RUNNER_ADMIN were found: \"a-id\", \"z-id\""},
		},
		{
			Name: "matching_role_without_id",
			Assignments: []*v1.RoleAssignment{
				assignment("", v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN),
			},
			Expected: Expectation{Err: "the matching direct assignment has an empty ID"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			selected, err := selectCreatedRunnerRoleAssignment(tc.Assignments, v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.ID = selected.GetId()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("selectCreatedRunnerRoleAssignment() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFormatRunnerRoleAssignments(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		IDs     string
		Derived string
	}
	runnersAdmin := v1.ResourceRole_RESOURCE_ROLE_ORG_RUNNERS_ADMIN
	orgAdmin := v1.ResourceRole_RESOURCE_ROLE_ORG_ADMIN
	assignments := []*v1.RoleAssignment{
		{Id: "z-id", DerivedFromOrgRole: &runnersAdmin},
		{Id: "", DerivedFromOrgRole: &orgAdmin},
		{Id: "a-id", DerivedFromOrgRole: &orgAdmin},
	}
	got := Expectation{
		IDs:     formatRunnerRoleAssignmentIDs(assignments),
		Derived: formatDerivedRunnerRoleAssignments(assignments),
	}
	expected := Expectation{
		IDs:     "\"<empty>\", \"a-id\", \"z-id\"",
		Derived: "ID \"\" from RESOURCE_ROLE_ORG_ADMIN, ID \"a-id\" from RESOURCE_ROLE_ORG_ADMIN, ID \"z-id\" from RESOURCE_ROLE_ORG_RUNNERS_ADMIN",
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("runner role assignment formatting mismatch (-want +got):\n%s", diff)
	}
}

func TestRunnerRoleAssignmentUpdateIsRejected(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Diagnostics []runnerRoleAssignmentDiagnostic
	}
	var response resource.UpdateResponse
	(&RunnerRoleAssignmentResource{}).Update(t.Context(), resource.UpdateRequest{}, &response)
	got := Expectation{Diagnostics: runnerRoleAssignmentDiagnostics(response.Diagnostics)}
	expected := Expectation{Diagnostics: []runnerRoleAssignmentDiagnostic{{
		Summary: "Unable to Update Ona Runner Role Assignment",
		Detail:  "Runner role assignments are immutable. Change runner_id, group_id, or role by replacing the resource.",
	}}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("RunnerRoleAssignmentResource.Update() mismatch (-want +got):\n%s", diff)
	}
}

func TestSplitRunnerRoleAssignmentImportID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Parts       []string
		Diagnostics []runnerRoleAssignmentDiagnostic
	}
	tests := []struct {
		Name     string
		ID       string
		Expected Expectation
	}{
		{
			Name:     "valid",
			ID:       runnerRoleAssignmentTestRunnerID + "/" + runnerRoleAssignmentTestGroupID,
			Expected: Expectation{Parts: []string{runnerRoleAssignmentTestRunnerID, runnerRoleAssignmentTestGroupID}},
		},
		{
			Name: "missing_component",
			ID:   "runner-id",
			Expected: Expectation{Diagnostics: []runnerRoleAssignmentDiagnostic{{
				Summary: "Invalid Import ID",
				Detail:  "Expected import ID format: runner_id/group_id.",
			}}},
		},
		{
			Name: "extra_component",
			ID:   "runner-id/group-id/extra",
			Expected: Expectation{Diagnostics: []runnerRoleAssignmentDiagnostic{{
				Summary: "Invalid Import ID",
				Detail:  "Expected import ID format: runner_id/group_id.",
			}}},
		},
		{
			Name: "empty_component",
			ID:   runnerRoleAssignmentTestRunnerID + "/",
			Expected: Expectation{Diagnostics: []runnerRoleAssignmentDiagnostic{{
				Summary: "Invalid Import ID",
				Detail:  "Expected import ID format: runner_id/group_id.",
			}}},
		},
		{
			Name: "invalid_runner_uuid",
			ID:   "runner-id/" + runnerRoleAssignmentTestGroupID,
			Expected: Expectation{Diagnostics: []runnerRoleAssignmentDiagnostic{{
				Summary: "Invalid Import ID",
				Detail:  "runner_id must be a valid UUID.",
			}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			parts, diags := splitRunnerRoleAssignmentImportID(tc.ID)
			got := Expectation{Parts: parts, Diagnostics: runnerRoleAssignmentDiagnostics(diags)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("splitRunnerRoleAssignmentImportID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateRunnerRoleAssignmentIdentity(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Diagnostics []runnerRoleAssignmentDiagnostic
	}
	tests := []struct {
		Name     string
		Identity RunnerRoleAssignmentIdentityModel
		Expected Expectation
	}{
		{
			Name: "valid",
			Identity: RunnerRoleAssignmentIdentityModel{
				RunnerID: types.StringValue(runnerRoleAssignmentTestRunnerID),
				GroupID:  types.StringValue(runnerRoleAssignmentTestGroupID),
			},
		},
		{
			Name: "empty_runner_id",
			Identity: RunnerRoleAssignmentIdentityModel{
				RunnerID: types.StringValue(""),
				GroupID:  types.StringValue(runnerRoleAssignmentTestGroupID),
			},
			Expected: Expectation{Diagnostics: []runnerRoleAssignmentDiagnostic{{
				Summary: "Invalid Runner Role Assignment Identity",
				Detail:  "runner_id must be a non-empty string.",
			}}},
		},
		{
			Name: "whitespace_group_id",
			Identity: RunnerRoleAssignmentIdentityModel{
				RunnerID: types.StringValue(runnerRoleAssignmentTestRunnerID),
				GroupID:  types.StringValue("  "),
			},
			Expected: Expectation{Diagnostics: []runnerRoleAssignmentDiagnostic{{
				Summary: "Invalid Runner Role Assignment Identity",
				Detail:  "group_id must be a non-empty string.",
			}}},
		},
		{
			Name: "unknown_runner_id",
			Identity: RunnerRoleAssignmentIdentityModel{
				RunnerID: types.StringUnknown(),
				GroupID:  types.StringValue(runnerRoleAssignmentTestGroupID),
			},
			Expected: Expectation{Diagnostics: []runnerRoleAssignmentDiagnostic{{
				Summary: "Invalid Runner Role Assignment Identity",
				Detail:  "runner_id must be a non-empty string.",
			}}},
		},
		{
			Name: "invalid_group_uuid",
			Identity: RunnerRoleAssignmentIdentityModel{
				RunnerID: types.StringValue(runnerRoleAssignmentTestRunnerID),
				GroupID:  types.StringValue("group-id"),
			},
			Expected: Expectation{Diagnostics: []runnerRoleAssignmentDiagnostic{{
				Summary: "Invalid Runner Role Assignment Identity",
				Detail:  "group_id must be a valid UUID.",
			}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateRunnerRoleAssignmentIdentity(tc.Identity, &diags)
			got := Expectation{Diagnostics: runnerRoleAssignmentDiagnostics(diags)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateRunnerRoleAssignmentIdentity() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateConfiguredRunnerRoleAssignmentID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name         string
		Value        types.String
		RequireKnown bool
		Expected     []runnerRoleAssignmentDiagnostic
	}{
		{Name: "valid", Value: types.StringValue(runnerRoleAssignmentTestRunnerID)},
		{Name: "unknown_during_planning", Value: types.StringUnknown()},
		{
			Name:         "unknown_during_create",
			Value:        types.StringUnknown(),
			RequireKnown: true,
			Expected: []runnerRoleAssignmentDiagnostic{{
				Summary: "Invalid Runner Role Assignment Configuration",
				Detail:  "runner_id must be a known UUID before creating the assignment.",
			}},
		},
		{
			Name:  "invalid_uuid",
			Value: types.StringValue("runner-id"),
			Expected: []runnerRoleAssignmentDiagnostic{{
				Summary: "Invalid Runner Role Assignment Configuration",
				Detail:  "runner_id must be a valid UUID.",
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateConfiguredRunnerRoleAssignmentID(path.Root("runner_id"), "runner_id", tc.Value, tc.RequireKnown, &diags)
			if diff := cmp.Diff(tc.Expected, runnerRoleAssignmentDiagnostics(diags)); diff != "" {
				t.Errorf("validateConfiguredRunnerRoleAssignmentID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateRunnerRole(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Diagnostics []runnerRoleAssignmentDiagnostic
	}
	tests := []struct {
		Name     string
		Role     types.String
		Expected Expectation
	}{
		{Name: "valid", Role: types.StringValue("admin")},
		{Name: "null", Role: types.StringNull()},
		{Name: "unknown", Role: types.StringUnknown()},
		{
			Name: "unsupported",
			Role: types.StringValue("viewer"),
			Expected: Expectation{Diagnostics: []runnerRoleAssignmentDiagnostic{{
				Summary: "Unsupported Runner Role",
				Detail:  "Unsupported runner role \"viewer\". Supported values are: admin, user.",
			}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateRunnerRole(tc.Role, &diags)
			got := Expectation{Diagnostics: runnerRoleAssignmentDiagnostics(diags)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateRunnerRole() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func cloneRunnerRoleAssignment(source *v1.RoleAssignment, mutate func(*v1.RoleAssignment)) *v1.RoleAssignment {
	result := proto.CloneOf(source)
	mutate(result)
	return result
}

func runnerRoleAssignmentIDs(assignments []*v1.RoleAssignment) []string {
	result := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		result = append(result, assignment.GetId())
	}
	return result
}

func runnerRoleAssignmentDiagnostics(diags diag.Diagnostics) []runnerRoleAssignmentDiagnostic {
	if len(diags) == 0 {
		return nil
	}
	result := make([]runnerRoleAssignmentDiagnostic, 0, len(diags))
	for _, diagnostic := range diags {
		result = append(result, runnerRoleAssignmentDiagnostic{
			Summary: diagnostic.Summary(),
			Detail:  diagnostic.Detail(),
		})
	}
	return result
}
