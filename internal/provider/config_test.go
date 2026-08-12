// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// not parallel: each case sets process-wide ONA_HOST and ONA_TOKEN fallbacks.
func TestNewManagementPlane(t *testing.T) {
	type Expectation struct {
		BaseURL string
		Err     string
	}

	tests := []struct {
		Name     string
		Host     string
		Token    string
		EnvHost  string
		EnvToken string
		Expected Expectation
	}{
		{Name: "adds_https_and_api_path", Host: "example.com/tenant/", Token: "token", Expected: Expectation{BaseURL: "https://example.com/tenant/api"}},
		{Name: "trims_host_whitespace", Host: " https://example.com/ ", Token: "token", Expected: Expectation{BaseURL: "https://example.com/api"}},
		{Name: "uses_environment_fallbacks", EnvHost: "https://example.com/custom", EnvToken: "token", Expected: Expectation{BaseURL: "https://example.com/custom/api"}},
		{Name: "uses_default_host", EnvToken: "token", Expected: Expectation{BaseURL: "https://app.gitpod.io/api"}},
		{Name: "strips_query_and_fragment", Host: "https://example.com/path?query=value#fragment", Token: "token", Expected: Expectation{BaseURL: "https://example.com/path/api"}},
		{Name: "rejects_missing_token", Expected: Expectation{Err: "missing Ona token: set provider token or ONA_TOKEN"}},
		{Name: "rejects_invalid_scheme", Host: "ftp://example.com", Token: "token", Expected: Expectation{Err: "invalid Ona host \"ftp://example.com\""}},
		{Name: "rejects_missing_host", Host: "https://", Token: "token", Expected: Expectation{Err: "invalid Ona host \"https://\""}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Setenv("ONA_HOST", tc.EnvHost)
			t.Setenv("ONA_TOKEN", tc.EnvToken)

			_, baseURL, err := newManagementPlane(tc.Host, tc.Token, "test-agent")
			got := Expectation{BaseURL: baseURL}
			if err != nil {
				got.Err = err.Error()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("newManagementPlane() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
