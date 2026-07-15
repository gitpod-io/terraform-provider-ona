// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"sort"
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
			Name: "v3_sorts_custom_claim_fields",
			Input: OIDCConfigModel{
				CustomClaimFields: stringSet(t, []string{"project_id", "creator_email"}),
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
			Name: "rejects_unsupported_custom_claim_fields",
			Input: OIDCConfigModel{
				CustomClaimFields: stringSet(t, []string{"not_a_claim"}),
			},
			Expected: Expectation{
				Err: "Unsupported OIDC Custom Claim Fields",
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

func TestPopulateOIDCConfigModel(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		CustomClaimFields []string
		Err               string
	}

	tests := []struct {
		Name     string
		Input    *v1.OIDCConfig
		Expected Expectation
	}{
		{
			Name: "v3_custom_claim_fields",
			Input: &v1.OIDCConfig{
				Version: &v1.OIDCConfig_V3{V3: &v1.OIDCConfigV3{
					ExtraSubFields: []string{"project_id", "creator_email"},
				}},
			},
			Expected: Expectation{CustomClaimFields: []string{"creator_email", "project_id"}},
		},
		{
			Name: "rejects_v2",
			Input: &v1.OIDCConfig{
				Version: &v1.OIDCConfig_V2{V2: &v1.OIDCConfigV2{}},
			},
			Expected: Expectation{Err: "Unsupported Ona OIDC Config Version"},
		},
		{
			Name:     "rejects_missing_config",
			Input:    nil,
			Expected: Expectation{Err: "Missing Ona OIDC Config"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			var model OIDCConfigModel
			var diags diag.Diagnostics
			populateOIDCConfigModel(t.Context(), &model, "org-1", tc.Input, OIDCConfigModel{}, &diags)
			if diags.HasError() {
				got.Err = diags[0].Summary()
			} else {
				diags.Append(model.CustomClaimFields.ElementsAs(t.Context(), &got.CustomClaimFields, false)...)
				if diags.HasError() {
					got.Err = diags[0].Summary()
				}
				sort.Strings(got.CustomClaimFields)
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("populateOIDCConfigModel() mismatch (-want +got):\n%s", diff)
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
