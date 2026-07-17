// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"testing"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestValidateSCIMDurationValue(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Err string
	}

	tests := []struct {
		Name     string
		Input    types.String
		Expected Expectation
	}{
		{
			Name:  "minimum",
			Input: types.StringValue("24h"),
		},
		{
			Name:  "maximum",
			Input: types.StringValue("17520h"),
		},
		{
			Name:  "too_short",
			Input: types.StringValue("23h"),
			Expected: Expectation{
				Err: "Invalid SCIM Token Duration",
			},
		},
		{
			Name:  "invalid",
			Input: types.StringValue("one year"),
			Expected: Expectation{
				Err: "Invalid SCIM Token Duration",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateSCIMDurationValue(tc.Input, path.Root("token_expires_in"), &diags)
			var got Expectation
			if diags.HasError() {
				got.Err = diags[0].Summary()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateSCIMDurationValue() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPopulateSCIMConfigurationModel(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 7, 10, 12, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	expiresAt := time.Date(2027, 7, 10, 12, 0, 0, 0, time.UTC)

	type Expectation struct {
		Result SCIMConfigurationModel
	}

	expected := Expectation{
		Result: SCIMConfigurationModel{
			ID:                                 types.StringValue("scim-1"),
			SSOConfigurationID:                 types.StringValue("sso-1"),
			Name:                               types.StringValue("Example SCIM"),
			Enabled:                            types.BoolValue(true),
			AllowUnverifiedEmailAccountLinking: types.BoolValue(false),
			TokenExpiresIn:                     types.StringValue("8760h"),
			TokenExpiresAt:                     types.StringValue("2027-07-10T12:00:00Z"),
			CreatedAt:                          types.StringValue("2026-07-10T10:00:00Z"),
		},
	}

	var data SCIMConfigurationModel
	populateSCIMConfigurationModel(&data, &v1.SCIMConfiguration{
		Id:                                 "scim-1",
		Name:                               "Example SCIM",
		Enabled:                            true,
		SsoConfigurationId:                 "sso-1",
		AllowUnverifiedEmailAccountLinking: false,
		TokenExpiresAt:                     timestamppb.New(expiresAt),
		CreatedAt:                          timestamppb.New(createdAt),
	}, SCIMConfigurationModel{
		TokenExpiresIn: types.StringValue("8760h"),
	})
	got := Expectation{Result: data}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("populateSCIMConfigurationModel() mismatch (-want +got):\n%s", diff)
	}
}

func TestSCIMConfigurationImportStateSeedsEquivalentRefreshState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resourceUnderTest := &SCIMConfigurationResource{}
	const scimConfigurationID = "scim-1"

	legacy := importSCIMConfigurationState(t, ctx, resourceUnderTest, resource.ImportStateRequest{
		ID: scimConfigurationID,
	})
	structured := importSCIMConfigurationState(t, ctx, resourceUnderTest, resource.ImportStateRequest{
		Identity: newSCIMConfigurationImportIdentity(t, ctx, resourceUnderTest, scimConfigurationID),
	})

	if !legacy.State.Raw.Equal(structured.State.Raw) {
		t.Fatalf("legacy and structured import state differ\nlegacy: %s\nstructured: %s", legacy.State.Raw, structured.State.Raw)
	}
	if got := scimConfigurationStateID(t, ctx, structured.State); got != scimConfigurationID {
		t.Fatalf("structured import seeded state id %q, want %q", got, scimConfigurationID)
	}
}

func importSCIMConfigurationState(t *testing.T, ctx context.Context, resourceUnderTest *SCIMConfigurationResource, req resource.ImportStateRequest) resource.ImportStateResponse {
	t.Helper()

	var schemaResp resource.SchemaResponse
	resourceUnderTest.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema() diagnostics: %v", schemaResp.Diagnostics)
	}

	resp := resource.ImportStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, SCIMConfigurationModel{})...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("empty state diagnostics: %v", resp.Diagnostics)
	}

	resourceUnderTest.ImportState(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ImportState() diagnostics: %v", resp.Diagnostics)
	}

	return resp
}

func newSCIMConfigurationImportIdentity(t *testing.T, ctx context.Context, resourceUnderTest *SCIMConfigurationResource, id string) *tfsdk.ResourceIdentity {
	t.Helper()

	var identitySchemaResp resource.IdentitySchemaResponse
	resourceUnderTest.IdentitySchema(ctx, resource.IdentitySchemaRequest{}, &identitySchemaResp)
	if identitySchemaResp.Diagnostics.HasError() {
		t.Fatalf("IdentitySchema() diagnostics: %v", identitySchemaResp.Diagnostics)
	}

	return &tfsdk.ResourceIdentity{
		Raw: tftypes.NewValue(identitySchemaResp.IdentitySchema.Type().TerraformType(ctx), map[string]tftypes.Value{
			"id": tftypes.NewValue(tftypes.String, id),
		}),
		Schema: identitySchemaResp.IdentitySchema,
	}
}

func scimConfigurationStateID(t *testing.T, ctx context.Context, state tfsdk.State) string {
	t.Helper()

	var id types.String
	diags := state.GetAttribute(ctx, path.Root("id"), &id)
	if diags.HasError() {
		t.Fatalf("state id diagnostics: %v", diags)
	}

	return id.ValueString()
}
