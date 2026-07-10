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

func TestValidateServiceAccountValidUntil(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Err string
	}

	now := time.Date(2026, time.July, 9, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		Name     string
		Input    types.String
		Expected Expectation
	}{
		{
			Name:     "accepts_future_rfc3339_timestamp",
			Input:    types.StringValue("2026-07-10T12:00:00Z"),
			Expected: Expectation{},
		},
		{
			Name:     "ignores_unknown_value",
			Input:    types.StringUnknown(),
			Expected: Expectation{},
		},
		{
			Name:  "rejects_invalid_timestamp",
			Input: types.StringValue("tomorrow"),
			Expected: Expectation{
				Err: "Invalid Service Account Expiration",
			},
		},
		{
			Name:  "rejects_past_timestamp",
			Input: types.StringValue("2026-07-08T12:00:00Z"),
			Expected: Expectation{
				Err: "Invalid Service Account Expiration",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateServiceAccountValidUntil(tc.Input, now, &diags)

			var got Expectation
			if diags.HasError() {
				got.Err = diags[0].Summary()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateServiceAccountValidUntil() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
