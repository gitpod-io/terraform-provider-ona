// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestImportStateSeedsEquivalentAutomationID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name       string
		Structured bool
	}{
		{Name: "legacy_string_id"},
		{Name: "structured_identity", Structured: true},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := runAutomationImport(t.Context(), tc.Structured)
			expected := automationImportExpectation{ID: "automation-1"}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("ImportState() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type automationImportExpectation struct {
	ID     string
	Errors []string
}

func runAutomationImport(ctx context.Context, structured bool) automationImportExpectation {
	resourceUnderTest := &Resource{}
	request := resource.ImportStateRequest{ID: "automation-1"}
	var result automationImportExpectation

	if structured {
		var identitySchemaResponse resource.IdentitySchemaResponse
		resourceUnderTest.IdentitySchema(ctx, resource.IdentitySchemaRequest{}, &identitySchemaResponse)
		result.Errors = append(result.Errors, automationImportErrors(identitySchemaResponse.Diagnostics)...)
		identity := &tfsdk.ResourceIdentity{Schema: identitySchemaResponse.IdentitySchema}
		result.Errors = append(result.Errors, automationImportErrors(identity.Set(ctx, IdentityModel{AutomationID: types.StringValue("automation-1")}))...)
		request = resource.ImportStateRequest{Identity: identity}
	}

	state := tfsdk.State{Schema: resourceSchema()}
	result.Errors = append(result.Errors, automationImportErrors(state.Set(ctx, emptyAutomationModel()))...)
	response := resource.ImportStateResponse{State: state}
	if len(result.Errors) == 0 {
		resourceUnderTest.ImportState(ctx, request, &response)
		result.Errors = append(result.Errors, automationImportErrors(response.Diagnostics)...)
	}

	var imported Model
	result.Errors = append(result.Errors, automationImportErrors(response.State.Get(ctx, &imported))...)
	if !imported.ID.IsNull() && !imported.ID.IsUnknown() {
		result.ID = imported.ID.ValueString()
	}
	return result
}

func automationImportErrors(diags diag.Diagnostics) []string {
	var result []string
	for _, diagnostic := range diags.Errors() {
		result = append(result, diagnostic.Summary()+": "+diagnostic.Detail())
	}
	return result
}

func emptyAutomationModel() Model {
	return Model{
		ID:            types.StringNull(),
		Agent:         types.StringNull(),
		Name:          types.StringNull(),
		Description:   types.StringNull(),
		CodexSettings: types.ObjectNull(codexSettingsAttributeTypes),
		Triggers:      types.ListNull(types.ObjectType{AttrTypes: triggerAttributeTypes}),
		Action:        types.ObjectNull(actionAttributeTypes),
		Executor:      types.ObjectNull(subjectAttributeTypes),
		Disabled:      types.BoolNull(),
		WebhookURL:    types.StringNull(),
		Creator:       types.ObjectNull(subjectAttributeTypes),
		CreatedAt:     types.StringNull(),
		UpdatedAt:     types.StringNull(),
	}
}
