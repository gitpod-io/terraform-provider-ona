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

type projectRoleAssignmentDiagnostic struct {
	Summary string
	Detail  string
}

const (
	projectRoleAssignmentTestProjectID = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	projectRoleAssignmentTestGroupID   = "22222222-2222-4222-8222-222222222222"
)

func TestProjectRoleAssignmentSchema(t *testing.T) {
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

	resourceUnderTest := &ProjectRoleAssignmentResource{}
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
		TypeName: "ona_project_role_assignment",
		Attributes: map[string]Attribute{
			"id": {
				Computed:      true,
				PlanModifiers: []string{"stringplanmodifier.useStateForUnknownModifier"},
			},
			"project_id": {
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
		t.Errorf("ProjectRoleAssignmentResource schema mismatch (-want +got):\n%s", diff)
	}
}

func TestProjectRoleAssignmentIdentitySchema(t *testing.T) {
	t.Parallel()

	type Attribute struct {
		RequiredForImport bool
		Description       string
	}
	type Expectation struct {
		Attributes map[string]Attribute
		Errors     []string
	}

	resourceUnderTest := &ProjectRoleAssignmentResource{}
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
		"project_id": {RequiredForImport: true, Description: "Project ID receiving the group access."},
		"group_id":   {RequiredForImport: true, Description: "Group ID receiving access to the project."},
	}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ProjectRoleAssignmentResource identity schema mismatch (-want +got):\n%s", diff)
	}
}

func TestProjectRoleAssignmentImportState(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		ProjectID  string
		GroupID    string
		RoleIsNull bool
		Errors     []string
	}
	tests := []struct {
		Name       string
		LegacyID   string
		Identity   ProjectRoleAssignmentIdentityModel
		Structured bool
		Expected   Expectation
	}{
		{
			Name:     "legacy_string_id",
			LegacyID: projectRoleAssignmentTestProjectID + "/" + projectRoleAssignmentTestGroupID,
			Expected: Expectation{ProjectID: projectRoleAssignmentTestProjectID, GroupID: projectRoleAssignmentTestGroupID, RoleIsNull: true},
		},
		{
			Name:       "structured_identity",
			Structured: true,
			Identity: ProjectRoleAssignmentIdentityModel{
				ProjectID: types.StringValue(projectRoleAssignmentTestProjectID),
				GroupID:   types.StringValue(projectRoleAssignmentTestGroupID),
			},
			Expected: Expectation{ProjectID: projectRoleAssignmentTestProjectID, GroupID: projectRoleAssignmentTestGroupID, RoleIsNull: true},
		},
		{
			Name:     "malformed_legacy_id",
			LegacyID: "project-id",
			Expected: Expectation{RoleIsNull: true, Errors: []string{
				"Invalid Import ID: Expected import ID format: project_id/group_id.",
			}},
		},
		{
			Name:       "invalid_structured_identity",
			Structured: true,
			Identity: ProjectRoleAssignmentIdentityModel{
				ProjectID: types.StringValue(""),
				GroupID:   types.StringValue(projectRoleAssignmentTestGroupID),
			},
			Expected: Expectation{RoleIsNull: true, Errors: []string{
				"Invalid Project Role Assignment Identity: project_id must be a non-empty string.",
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			resourceUnderTest := &ProjectRoleAssignmentResource{}
			var schemaResponse resource.SchemaResponse
			resourceUnderTest.Schema(t.Context(), resource.SchemaRequest{}, &schemaResponse)
			state := tfsdk.State{Schema: schemaResponse.Schema}
			emptyModel := ProjectRoleAssignmentModel{
				ID:        types.StringNull(),
				ProjectID: types.StringNull(),
				GroupID:   types.StringNull(),
				Role:      types.StringNull(),
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

			var imported ProjectRoleAssignmentModel
			for _, diagnostic := range response.State.Get(t.Context(), &imported).Errors() {
				got.Errors = append(got.Errors, diagnostic.Summary()+": "+diagnostic.Detail())
			}
			if !imported.ProjectID.IsNull() && !imported.ProjectID.IsUnknown() {
				got.ProjectID = imported.ProjectID.ValueString()
			}
			if !imported.GroupID.IsNull() && !imported.GroupID.IsUnknown() {
				got.GroupID = imported.GroupID.ValueString()
			}
			got.RoleIsNull = imported.Role.IsNull()

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("ProjectRoleAssignmentResource.ImportState() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestProjectRoleMapping(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		RoleToAPI     map[string]v1.ResourceRole
		APIToRole     map[v1.ResourceRole]string
		SupportedRole []string
	}

	expected := Expectation{
		RoleToAPI: map[string]v1.ResourceRole{
			"admin":  v1.ResourceRole_RESOURCE_ROLE_PROJECT_ADMIN,
			"editor": v1.ResourceRole_RESOURCE_ROLE_PROJECT_EDITOR,
			"user":   v1.ResourceRole_RESOURCE_ROLE_PROJECT_USER,
		},
		APIToRole: map[v1.ResourceRole]string{
			v1.ResourceRole_RESOURCE_ROLE_PROJECT_ADMIN:  "admin",
			v1.ResourceRole_RESOURCE_ROLE_PROJECT_EDITOR: "editor",
			v1.ResourceRole_RESOURCE_ROLE_PROJECT_USER:   "user",
		},
		SupportedRole: []string{"admin", "editor", "user"},
	}
	got := Expectation{
		RoleToAPI:     projectRoleToAPI,
		APIToRole:     apiToProjectRole,
		SupportedRole: supportedProjectRoles(),
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("Project role mapping mismatch (-want +got):\n%s", diff)
	}
}

func TestProjectRoleAssignmentModelFromAPI(t *testing.T) {
	t.Parallel()

	type Result struct {
		ID        string
		ProjectID string
		GroupID   string
		Role      string
	}
	type Expectation struct {
		Result Result
		Err    string
	}

	valid := &v1.RoleAssignment{
		Id:           "assignment-id",
		GroupId:      "group-id",
		ResourceId:   "project-id",
		ResourceType: v1.ResourceType_RESOURCE_TYPE_PROJECT,
		ResourceRole: v1.ResourceRole_RESOURCE_ROLE_PROJECT_EDITOR,
	}
	derivedRole := v1.ResourceRole_RESOURCE_ROLE_ORG_PROJECTS_ADMIN
	tests := []struct {
		Name       string
		Assignment *v1.RoleAssignment
		Expected   Expectation
	}{
		{
			Name:       "valid_editor",
			Assignment: valid,
			Expected: Expectation{Result: Result{
				ID:        "assignment-id",
				ProjectID: "project-id",
				GroupID:   "group-id",
				Role:      "editor",
			}},
		},
		{Name: "missing_assignment", Expected: Expectation{Err: "role assignment is missing"}},
		{
			Name:       "missing_id",
			Assignment: cloneProjectRoleAssignment(valid, func(a *v1.RoleAssignment) { a.Id = "" }),
			Expected:   Expectation{Err: "role assignment ID is empty"},
		},
		{
			Name:       "missing_group_id",
			Assignment: cloneProjectRoleAssignment(valid, func(a *v1.RoleAssignment) { a.GroupId = "" }),
			Expected:   Expectation{Err: "role assignment group ID is empty"},
		},
		{
			Name:       "missing_project_id",
			Assignment: cloneProjectRoleAssignment(valid, func(a *v1.RoleAssignment) { a.ResourceId = "" }),
			Expected:   Expectation{Err: "role assignment project ID is empty"},
		},
		{
			Name: "wrong_resource_type",
			Assignment: cloneProjectRoleAssignment(valid, func(a *v1.RoleAssignment) {
				a.ResourceType = v1.ResourceType_RESOURCE_TYPE_WORKFLOW
			}),
			Expected: Expectation{Err: "role assignment resource type is RESOURCE_TYPE_WORKFLOW, expected RESOURCE_TYPE_PROJECT"},
		},
		{
			Name: "derived_assignment",
			Assignment: cloneProjectRoleAssignment(valid, func(a *v1.RoleAssignment) {
				a.DerivedFromOrgRole = &derivedRole
			}),
			Expected: Expectation{Err: "role assignment is derived from organization role RESOURCE_ROLE_ORG_PROJECTS_ADMIN"},
		},
		{
			Name: "unsupported_project_role",
			Assignment: cloneProjectRoleAssignment(valid, func(a *v1.RoleAssignment) {
				a.ResourceRole = v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_VIEWER
			}),
			Expected: Expectation{Err: "role assignment has unsupported project role RESOURCE_ROLE_WORKFLOW_VIEWER"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			model, err := projectRoleAssignmentModelFromAPI(tc.Assignment)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Result = Result{
					ID:        model.ID.ValueString(),
					ProjectID: model.ProjectID.ValueString(),
					GroupID:   model.GroupID.ValueString(),
					Role:      model.Role.ValueString(),
				}
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("projectRoleAssignmentModelFromAPI() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestClassifyProjectRoleAssignments(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		DirectIDs  []string
		DerivedIDs []string
	}

	direct := &v1.RoleAssignment{
		Id:           "direct-id",
		GroupId:      "group-id",
		ResourceId:   "project-id",
		ResourceType: v1.ResourceType_RESOURCE_TYPE_PROJECT,
		ResourceRole: v1.ResourceRole_RESOURCE_ROLE_PROJECT_USER,
	}
	derivedRole := v1.ResourceRole_RESOURCE_ROLE_ORG_PROJECTS_ADMIN
	derived := cloneProjectRoleAssignment(direct, func(a *v1.RoleAssignment) {
		a.Id = "derived-id"
		a.DerivedFromOrgRole = &derivedRole
	})
	unspecifiedRole := v1.ResourceRole_RESOURCE_ROLE_UNSPECIFIED
	explicitlyDirect := cloneProjectRoleAssignment(direct, func(a *v1.RoleAssignment) {
		a.Id = "explicitly-direct-id"
		a.DerivedFromOrgRole = &unspecifiedRole
	})
	assignments := []*v1.RoleAssignment{
		direct,
		derived,
		explicitlyDirect,
		nil,
		cloneProjectRoleAssignment(direct, func(a *v1.RoleAssignment) { a.GroupId = "other-group" }),
		cloneProjectRoleAssignment(direct, func(a *v1.RoleAssignment) { a.ResourceId = "other-project" }),
		cloneProjectRoleAssignment(direct, func(a *v1.RoleAssignment) {
			a.ResourceType = v1.ResourceType_RESOURCE_TYPE_WORKFLOW
		}),
	}

	matches := classifyProjectRoleAssignments(assignments, "project-id", "group-id")
	got := Expectation{
		DirectIDs:  projectRoleAssignmentIDs(matches.direct),
		DerivedIDs: projectRoleAssignmentIDs(matches.derived),
	}
	expected := Expectation{
		DirectIDs:  []string{"direct-id", "explicitly-direct-id"},
		DerivedIDs: []string{"derived-id"},
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("classifyProjectRoleAssignments() mismatch (-want +got):\n%s", diff)
	}
}

func TestSelectCreatedProjectRoleAssignment(t *testing.T) {
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
				assignment("user-id", v1.ResourceRole_RESOURCE_ROLE_PROJECT_USER),
				assignment("editor-id", v1.ResourceRole_RESOURCE_ROLE_PROJECT_EDITOR),
			},
			Expected: Expectation{ID: "editor-id"},
		},
		{
			Name: "no_matching_role",
			Assignments: []*v1.RoleAssignment{
				assignment("user-id", v1.ResourceRole_RESOURCE_ROLE_PROJECT_USER),
			},
			Expected: Expectation{Err: "no direct assignment with role RESOURCE_ROLE_PROJECT_EDITOR was found"},
		},
		{
			Name: "multiple_matching_roles",
			Assignments: []*v1.RoleAssignment{
				assignment("z-id", v1.ResourceRole_RESOURCE_ROLE_PROJECT_EDITOR),
				assignment("a-id", v1.ResourceRole_RESOURCE_ROLE_PROJECT_EDITOR),
			},
			Expected: Expectation{Err: "multiple direct assignments with role RESOURCE_ROLE_PROJECT_EDITOR were found: \"a-id\", \"z-id\""},
		},
		{
			Name: "matching_role_without_id",
			Assignments: []*v1.RoleAssignment{
				assignment("", v1.ResourceRole_RESOURCE_ROLE_PROJECT_EDITOR),
			},
			Expected: Expectation{Err: "the matching direct assignment has an empty ID"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			selected, err := selectCreatedProjectRoleAssignment(tc.Assignments, v1.ResourceRole_RESOURCE_ROLE_PROJECT_EDITOR)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.ID = selected.GetId()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("selectCreatedProjectRoleAssignment() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFormatProjectRoleAssignments(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		IDs     string
		Derived string
	}
	projectsAdmin := v1.ResourceRole_RESOURCE_ROLE_ORG_PROJECTS_ADMIN
	orgAdmin := v1.ResourceRole_RESOURCE_ROLE_ORG_ADMIN
	assignments := []*v1.RoleAssignment{
		{Id: "z-id", DerivedFromOrgRole: &projectsAdmin},
		{Id: "", DerivedFromOrgRole: &orgAdmin},
		{Id: "a-id", DerivedFromOrgRole: &orgAdmin},
	}
	got := Expectation{
		IDs:     formatProjectRoleAssignmentIDs(assignments),
		Derived: formatDerivedProjectRoleAssignments(assignments),
	}
	expected := Expectation{
		IDs:     "\"<empty>\", \"a-id\", \"z-id\"",
		Derived: "ID \"\" from RESOURCE_ROLE_ORG_ADMIN, ID \"a-id\" from RESOURCE_ROLE_ORG_ADMIN, ID \"z-id\" from RESOURCE_ROLE_ORG_PROJECTS_ADMIN",
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("project role assignment formatting mismatch (-want +got):\n%s", diff)
	}
}

func TestProjectRoleAssignmentUpdateIsRejected(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Diagnostics []projectRoleAssignmentDiagnostic
	}
	var response resource.UpdateResponse
	(&ProjectRoleAssignmentResource{}).Update(t.Context(), resource.UpdateRequest{}, &response)
	got := Expectation{Diagnostics: projectRoleAssignmentDiagnostics(response.Diagnostics)}
	expected := Expectation{Diagnostics: []projectRoleAssignmentDiagnostic{{
		Summary: "Unable to Update Ona Project Role Assignment",
		Detail:  "Project role assignments are immutable. Change project_id, group_id, or role by replacing the resource.",
	}}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ProjectRoleAssignmentResource.Update() mismatch (-want +got):\n%s", diff)
	}
}

func TestSplitProjectRoleAssignmentImportID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Parts       []string
		Diagnostics []projectRoleAssignmentDiagnostic
	}
	tests := []struct {
		Name     string
		ID       string
		Expected Expectation
	}{
		{
			Name:     "valid",
			ID:       projectRoleAssignmentTestProjectID + "/" + projectRoleAssignmentTestGroupID,
			Expected: Expectation{Parts: []string{projectRoleAssignmentTestProjectID, projectRoleAssignmentTestGroupID}},
		},
		{
			Name: "missing_component",
			ID:   "project-id",
			Expected: Expectation{Diagnostics: []projectRoleAssignmentDiagnostic{{
				Summary: "Invalid Import ID",
				Detail:  "Expected import ID format: project_id/group_id.",
			}}},
		},
		{
			Name: "extra_component",
			ID:   "project-id/group-id/extra",
			Expected: Expectation{Diagnostics: []projectRoleAssignmentDiagnostic{{
				Summary: "Invalid Import ID",
				Detail:  "Expected import ID format: project_id/group_id.",
			}}},
		},
		{
			Name: "empty_component",
			ID:   projectRoleAssignmentTestProjectID + "/",
			Expected: Expectation{Diagnostics: []projectRoleAssignmentDiagnostic{{
				Summary: "Invalid Import ID",
				Detail:  "Expected import ID format: project_id/group_id.",
			}}},
		},
		{
			Name: "invalid_project_uuid",
			ID:   "project-id/" + projectRoleAssignmentTestGroupID,
			Expected: Expectation{Diagnostics: []projectRoleAssignmentDiagnostic{{
				Summary: "Invalid Import ID",
				Detail:  "project_id must be a valid UUID.",
			}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			parts, diags := splitProjectRoleAssignmentImportID(tc.ID)
			got := Expectation{Parts: parts, Diagnostics: projectRoleAssignmentDiagnostics(diags)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("splitProjectRoleAssignmentImportID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateProjectRoleAssignmentIdentity(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Diagnostics []projectRoleAssignmentDiagnostic
	}
	tests := []struct {
		Name     string
		Identity ProjectRoleAssignmentIdentityModel
		Expected Expectation
	}{
		{
			Name: "valid",
			Identity: ProjectRoleAssignmentIdentityModel{
				ProjectID: types.StringValue(projectRoleAssignmentTestProjectID),
				GroupID:   types.StringValue(projectRoleAssignmentTestGroupID),
			},
		},
		{
			Name: "empty_project_id",
			Identity: ProjectRoleAssignmentIdentityModel{
				ProjectID: types.StringValue(""),
				GroupID:   types.StringValue(projectRoleAssignmentTestGroupID),
			},
			Expected: Expectation{Diagnostics: []projectRoleAssignmentDiagnostic{{
				Summary: "Invalid Project Role Assignment Identity",
				Detail:  "project_id must be a non-empty string.",
			}}},
		},
		{
			Name: "whitespace_group_id",
			Identity: ProjectRoleAssignmentIdentityModel{
				ProjectID: types.StringValue(projectRoleAssignmentTestProjectID),
				GroupID:   types.StringValue("  "),
			},
			Expected: Expectation{Diagnostics: []projectRoleAssignmentDiagnostic{{
				Summary: "Invalid Project Role Assignment Identity",
				Detail:  "group_id must be a non-empty string.",
			}}},
		},
		{
			Name: "unknown_project_id",
			Identity: ProjectRoleAssignmentIdentityModel{
				ProjectID: types.StringUnknown(),
				GroupID:   types.StringValue(projectRoleAssignmentTestGroupID),
			},
			Expected: Expectation{Diagnostics: []projectRoleAssignmentDiagnostic{{
				Summary: "Invalid Project Role Assignment Identity",
				Detail:  "project_id must be a non-empty string.",
			}}},
		},
		{
			Name: "invalid_group_uuid",
			Identity: ProjectRoleAssignmentIdentityModel{
				ProjectID: types.StringValue(projectRoleAssignmentTestProjectID),
				GroupID:   types.StringValue("group-id"),
			},
			Expected: Expectation{Diagnostics: []projectRoleAssignmentDiagnostic{{
				Summary: "Invalid Project Role Assignment Identity",
				Detail:  "group_id must be a valid UUID.",
			}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateProjectRoleAssignmentIdentity(tc.Identity, &diags)
			got := Expectation{Diagnostics: projectRoleAssignmentDiagnostics(diags)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateProjectRoleAssignmentIdentity() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateConfiguredProjectRoleAssignmentID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name         string
		Value        types.String
		RequireKnown bool
		Expected     []projectRoleAssignmentDiagnostic
	}{
		{Name: "valid", Value: types.StringValue(projectRoleAssignmentTestProjectID)},
		{Name: "unknown_during_planning", Value: types.StringUnknown()},
		{
			Name:         "unknown_during_create",
			Value:        types.StringUnknown(),
			RequireKnown: true,
			Expected: []projectRoleAssignmentDiagnostic{{
				Summary: "Invalid Project Role Assignment Configuration",
				Detail:  "project_id must be a known UUID before creating the assignment.",
			}},
		},
		{
			Name:  "invalid_uuid",
			Value: types.StringValue("project-id"),
			Expected: []projectRoleAssignmentDiagnostic{{
				Summary: "Invalid Project Role Assignment Configuration",
				Detail:  "project_id must be a valid UUID.",
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateConfiguredProjectRoleAssignmentID(path.Root("project_id"), "project_id", tc.Value, tc.RequireKnown, &diags)
			if diff := cmp.Diff(tc.Expected, projectRoleAssignmentDiagnostics(diags)); diff != "" {
				t.Errorf("validateConfiguredProjectRoleAssignmentID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateProjectRole(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Diagnostics []projectRoleAssignmentDiagnostic
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
			Expected: Expectation{Diagnostics: []projectRoleAssignmentDiagnostic{{
				Summary: "Unsupported Project Role",
				Detail:  "Unsupported project role \"viewer\". Supported values are: admin, editor, user.",
			}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateProjectRole(tc.Role, &diags)
			got := Expectation{Diagnostics: projectRoleAssignmentDiagnostics(diags)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateProjectRole() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func cloneProjectRoleAssignment(source *v1.RoleAssignment, mutate func(*v1.RoleAssignment)) *v1.RoleAssignment {
	result := proto.CloneOf(source)
	mutate(result)
	return result
}

func projectRoleAssignmentIDs(assignments []*v1.RoleAssignment) []string {
	result := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		result = append(result, assignment.GetId())
	}
	return result
}

func projectRoleAssignmentDiagnostics(diags diag.Diagnostics) []projectRoleAssignmentDiagnostic {
	if len(diags) == 0 {
		return nil
	}
	result := make([]projectRoleAssignmentDiagnostic, 0, len(diags))
	for _, diagnostic := range diags {
		result = append(result, projectRoleAssignmentDiagnostic{
			Summary: diagnostic.Summary(),
			Detail:  diagnostic.Detail(),
		})
	}
	return result
}
