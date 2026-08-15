// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
	"time"

	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

			_, baseURL, err := newManagementPlane(tc.Host, tc.Token, "test-agent", managementclient.RateLimitRetryConfig{
				MaxRetries:    defaultRateLimitMaxRetries,
				MaxRetryDelay: defaultRateLimitMaxRetryDelay,
			})
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

func TestConfiguredRateLimitMaxRetries(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Value int64
		Err   string
	}
	tests := []struct {
		Name     string
		Value    types.Int64
		Expected Expectation
	}{
		{Name: "null_uses_default", Value: types.Int64Null(), Expected: Expectation{Value: 5}},
		{Name: "explicit_zero_disables_retries", Value: types.Int64Value(0), Expected: Expectation{Value: 0}},
		{Name: "explicit_positive_value", Value: types.Int64Value(12), Expected: Expectation{Value: 12}},
		{Name: "unknown_is_rejected", Value: types.Int64Unknown(), Expected: Expectation{Err: "rate_limit_max_retries must be known"}},
		{Name: "negative_is_rejected", Value: types.Int64Value(-1), Expected: Expectation{Err: "rate_limit_max_retries must be greater than or equal to 0"}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			value, err := configuredRateLimitMaxRetries(tc.Value)
			got := Expectation{Value: value}
			if err != nil {
				got.Err = err.Error()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("configuredRateLimitMaxRetries() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConfiguredRateLimitMaxRetryDelay(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Value time.Duration
		Err   string
	}
	tests := []struct {
		Name     string
		Value    types.String
		Expected Expectation
	}{
		{Name: "null_uses_default", Value: types.StringNull(), Expected: Expectation{Value: 30 * time.Second}},
		{Name: "explicit_positive_value", Value: types.StringValue("1m30s"), Expected: Expectation{Value: 90 * time.Second}},
		{Name: "unknown_is_rejected", Value: types.StringUnknown(), Expected: Expectation{Err: "rate_limit_max_retry_delay must be known"}},
		{Name: "malformed_is_rejected", Value: types.StringValue("invalid"), Expected: Expectation{Err: "rate_limit_max_retry_delay must be a valid Go duration: time: invalid duration \"invalid\""}},
		{Name: "zero_is_rejected", Value: types.StringValue("0s"), Expected: Expectation{Err: "rate_limit_max_retry_delay must be greater than 0"}},
		{Name: "negative_is_rejected", Value: types.StringValue("-1s"), Expected: Expectation{Err: "rate_limit_max_retry_delay must be greater than 0"}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			value, err := configuredRateLimitMaxRetryDelay(tc.Value)
			got := Expectation{Value: value}
			if err != nil {
				got.Err = err.Error()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("configuredRateLimitMaxRetryDelay() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPositiveDurationValidator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name        string
		Value       types.String
		ExpectedErr bool
	}{
		{Name: "null_is_allowed", Value: types.StringNull()},
		{Name: "unknown_is_deferred", Value: types.StringUnknown()},
		{Name: "positive_is_allowed", Value: types.StringValue("30s")},
		{Name: "malformed_is_rejected", Value: types.StringValue("invalid"), ExpectedErr: true},
		{Name: "zero_is_rejected", Value: types.StringValue("0s"), ExpectedErr: true},
		{Name: "negative_is_rejected", Value: types.StringValue("-1s"), ExpectedErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var resp validator.StringResponse
			positiveDurationValidator{}.ValidateString(t.Context(), validator.StringRequest{
				ConfigValue: tc.Value,
				Path:        path.Root("rate_limit_max_retry_delay"),
			}, &resp)
			got := resp.Diagnostics.HasError()
			if diff := cmp.Diff(tc.ExpectedErr, got); diff != "" {
				t.Errorf("positiveDurationValidator.ValidateString() error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
