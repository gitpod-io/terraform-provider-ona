// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestImportStateSeedsEquivalentProjectID(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	identitySchemaResponse := resource.IdentitySchemaResponse{}
	(&Resource{}).IdentitySchema(ctx, resource.IdentitySchemaRequest{}, &identitySchemaResponse)

	identity := &tfsdk.ResourceIdentity{Schema: identitySchemaResponse.IdentitySchema}
	if diags := identity.Set(ctx, IdentityModel{ID: types.StringValue("project-1")}); diags.HasError() {
		t.Fatalf("setting structured import identity: %v", diags)
	}

	tests := map[string]resource.ImportStateRequest{
		"legacy string ID": {ID: "project-1"},
		"structured identity": {
			Identity: identity,
		},
	}

	for name, request := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := tfsdk.State{Schema: resourceSchema()}
			if diags := state.Set(ctx, emptyProjectModel()); diags.HasError() {
				t.Fatalf("initializing import state: %v", diags)
			}

			response := resource.ImportStateResponse{State: state}
			(&Resource{}).ImportState(ctx, request, &response)
			if response.Diagnostics.HasError() {
				t.Fatalf("importing project: %v", response.Diagnostics)
			}

			var imported ProjectModel
			if diags := response.State.Get(ctx, &imported); diags.HasError() {
				t.Fatalf("reading refreshable import state: %v", diags)
			}
			if got := imported.ID.ValueString(); got != "project-1" {
				t.Fatalf("expected imported project ID project-1, got %q", got)
			}
		})
	}
}

func emptyProjectModel() ProjectModel {
	return ProjectModel{
		ID:                   types.StringNull(),
		Name:                 types.StringNull(),
		RepositoryCloneURL:   types.StringNull(),
		Branch:               types.StringNull(),
		InsightsEnabled:      types.BoolNull(),
		DevcontainerFilePath: types.StringNull(),
		AutomationsFilePath:  types.StringNull(),
		CreatedAt:            types.StringNull(),
		Creator:              types.ObjectNull(subjectObjectAttributeTypes),
	}
}
