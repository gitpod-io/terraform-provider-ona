// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package serviceaccount

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestValidateServiceAccountTokenValidFor(t *testing.T) {
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
			Name:     "accepts_go_duration",
			Input:    types.StringValue("2160h"),
			Expected: Expectation{},
		},
		{
			Name:     "ignores_omitted_duration",
			Input:    types.StringNull(),
			Expected: Expectation{},
		},
		{
			Name:  "rejects_invalid_duration",
			Input: types.StringValue("90 days"),
			Expected: Expectation{
				Err: "Invalid Service Account Token Validity",
			},
		},
		{
			Name:  "rejects_negative_duration",
			Input: types.StringValue("-1h"),
			Expected: Expectation{
				Err: "Invalid Service Account Token Validity",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateServiceAccountTokenValidFor(tc.Input, &diags)

			var got Expectation
			if diags.HasError() {
				got.Err = diags[0].Summary()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateServiceAccountTokenValidFor() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestServiceAccountTokenValidFor(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result time.Duration
		Err    string
	}

	tests := []struct {
		Name     string
		Input    types.String
		Expected Expectation
	}{
		{
			Name:  "parses_duration",
			Input: types.StringValue("720h"),
			Expected: Expectation{
				Result: 720 * time.Hour,
			},
		},
		{
			Name:  "rejects_negative_duration",
			Input: types.StringValue("-1s"),
			Expected: Expectation{
				Err: "valid_for must not be negative",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, err := serviceAccountTokenValidFor(tc.Input)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("serviceAccountTokenValidFor() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
