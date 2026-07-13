// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"testing"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	updatedAt := time.Date(2026, 7, 10, 12, 30, 0, 0, time.UTC)
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
			UpdatedAt:                          types.StringValue("2026-07-10T12:30:00Z"),
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
		UpdatedAt:                          timestamppb.New(updatedAt),
	}, SCIMConfigurationModel{
		TokenExpiresIn: types.StringValue("8760h"),
	})
	got := Expectation{Result: data}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("populateSCIMConfigurationModel() mismatch (-want +got):\n%s", diff)
	}
}
