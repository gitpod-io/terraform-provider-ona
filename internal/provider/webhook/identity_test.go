// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package webhook

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestImportStateSeedsEquivalentWebhookID(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	identitySchemaResponse := resource.IdentitySchemaResponse{}
	(&Resource{}).IdentitySchema(ctx, resource.IdentitySchemaRequest{}, &identitySchemaResponse)

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

			type Expectation struct {
				ID  string
				Err string
			}
			var got Expectation
			state := tfsdk.State{Schema: resourceSchema()}
			if diags := state.Set(t.Context(), emptyWebhookModel()); diags.HasError() {
				got.Err = fmt.Sprint(diags)
			} else {
				request := resource.ImportStateRequest{ID: "webhook-1"}
				if tc.Structured {
					identity := &tfsdk.ResourceIdentity{Schema: identitySchemaResponse.IdentitySchema}
					if diags := identity.Set(t.Context(), IdentityModel{ID: types.StringValue("webhook-1")}); diags.HasError() {
						got.Err = fmt.Sprint(diags)
					} else {
						request = resource.ImportStateRequest{Identity: identity}
					}
				}

				response := resource.ImportStateResponse{State: state}
				if got.Err == "" {
					(&Resource{}).ImportState(t.Context(), request, &response)
				}
				if response.Diagnostics.HasError() {
					got.Err = fmt.Sprint(response.Diagnostics)
				}

				var imported Model
				diags := response.State.Get(t.Context(), &imported)
				if diags.HasError() {
					got.Err = fmt.Sprint(diags)
				} else {
					got.ID = imported.ID.ValueString()
				}
			}
			expected := Expectation{ID: "webhook-1"}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("ImportState() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func emptyWebhookModel() Model {
	return Model{
		ID:                types.StringNull(),
		Name:              types.StringNull(),
		Description:       types.StringNull(),
		Type:              types.StringNull(),
		Provider:          types.StringNull(),
		RepositoryScopes:  types.SetNull(types.ObjectType{AttrTypes: repositoryScopeAttributeTypes}),
		OrganizationScope: types.ObjectNull(organizationScopeAttributeTypes),
		SecretVersion:     types.StringNull(),
		URL:               types.StringNull(),
		Creator:           types.ObjectNull(creatorAttributeTypes),
		CreatedAt:         types.StringNull(),
	}
}
