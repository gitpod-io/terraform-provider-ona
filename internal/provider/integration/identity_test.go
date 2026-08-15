// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestImportStateSeedsEquivalentIntegrationID(t *testing.T) {
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

			got := runIntegrationImport(t.Context(), tc.Structured)
			expected := integrationImportExpectation{ID: "integration-1"}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("ImportState() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type integrationImportExpectation struct {
	ID     string
	Errors []string
}

func runIntegrationImport(ctx context.Context, structured bool) integrationImportExpectation {
	resourceUnderTest := &Resource{}
	request := resource.ImportStateRequest{ID: "integration-1"}
	var result integrationImportExpectation

	if structured {
		var identitySchemaResponse resource.IdentitySchemaResponse
		resourceUnderTest.IdentitySchema(ctx, resource.IdentitySchemaRequest{}, &identitySchemaResponse)
		result.Errors = append(result.Errors, integrationImportErrors(identitySchemaResponse.Diagnostics)...)
		identity := &tfsdk.ResourceIdentity{Schema: identitySchemaResponse.IdentitySchema}
		result.Errors = append(result.Errors, integrationImportErrors(identity.Set(ctx, IdentityModel{IntegrationID: types.StringValue("integration-1")}))...)
		request = resource.ImportStateRequest{Identity: identity}
	}

	state := tfsdk.State{Schema: resourceSchema()}
	result.Errors = append(result.Errors, integrationImportErrors(state.Set(ctx, emptyIntegrationModel()))...)
	response := resource.ImportStateResponse{State: state}
	if len(result.Errors) == 0 {
		resourceUnderTest.ImportState(ctx, request, &response)
		result.Errors = append(result.Errors, integrationImportErrors(response.Diagnostics)...)
	}

	var imported Model
	result.Errors = append(result.Errors, integrationImportErrors(response.State.Get(ctx, &imported))...)
	if !imported.ID.IsNull() && !imported.ID.IsUnknown() {
		result.ID = imported.ID.ValueString()
	}
	return result
}

func integrationImportErrors(diags diag.Diagnostics) []string {
	var result []string
	for _, diagnostic := range diags.Errors() {
		result = append(result, diagnostic.Summary()+": "+diagnostic.Detail())
	}
	return result
}

func emptyIntegrationModel() Model {
	return Model{
		ID:                      types.StringNull(),
		OrganizationID:          types.StringNull(),
		IntegrationDefinitionID: types.StringNull(),
		RunnerID:                types.StringNull(),
		Enabled:                 types.BoolNull(),
		Capabilities:            types.ObjectNull(capabilitiesAttributeTypes),
		Auth:                    types.ObjectNull(authResourceAttributeTypes),
		Credentials:             types.ObjectNull(credentialsAttributeTypes),
		Host:                    types.StringNull(),
		Name:                    types.StringNull(),
		Description:             types.StringNull(),
		IconURL:                 types.StringNull(),
		Categories:              types.SetNull(types.StringType),
		ExternalInstallation:    types.ObjectNull(externalInstallationAttributeTypes),
	}
}
