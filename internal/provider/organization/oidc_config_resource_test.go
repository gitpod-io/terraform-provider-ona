// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestOIDCConfigFromModel(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result *v1.OIDCConfig
		Err    string
	}

	tests := []struct {
		Name     string
		Input    OIDCConfigModel
		Expected Expectation
	}{
		{
			Name: "v2",
			Input: OIDCConfigModel{
				Version:        types.StringValue(oidcVersionV2),
				ExtraSubFields: stringSet(t, nil),
			},
			Expected: Expectation{
				Result: &v1.OIDCConfig{
					Version: &v1.OIDCConfig_V2{V2: &v1.OIDCConfigV2{}},
				},
			},
		},
		{
			Name: "v3_sorts_extra_sub_fields",
			Input: OIDCConfigModel{
				Version:        types.StringValue(oidcVersionV3),
				ExtraSubFields: stringSet(t, []string{"creator_email", "project_id"}),
			},
			Expected: Expectation{
				Result: &v1.OIDCConfig{
					Version: &v1.OIDCConfig_V3{
						V3: &v1.OIDCConfigV3{
							ExtraSubFields: []string{"creator_email", "project_id"},
						},
					},
				},
			},
		},
		{
			Name: "rejects_extra_sub_fields_for_v2",
			Input: OIDCConfigModel{
				Version:        types.StringValue(oidcVersionV2),
				ExtraSubFields: stringSet(t, []string{"project_id"}),
			},
			Expected: Expectation{
				Err: "Invalid OIDC V2 Extra Subject Fields",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, diags := oidcConfigFromModel(t.Context(), tc.Input)
			if diags.HasError() {
				got.Err = diags[0].Summary()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("oidcConfigFromModel() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func stringSet(t *testing.T, values []string) types.Set {
	t.Helper()

	result, diags := types.SetValueFrom(t.Context(), types.StringType, values)
	if diags.HasError() {
		t.Fatalf("types.SetValueFrom() returned diagnostics: %s", diags.Errors()[0].Detail())
	}
	return result
}
