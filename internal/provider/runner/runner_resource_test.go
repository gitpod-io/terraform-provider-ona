// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/google/go-cmp/cmp"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCustomMetricsSchema(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		MetricsChildCount     int
		ManagedAttributeCount int
		CustomAttributeCount  int
		PasswordSensitive     bool
		PasswordWriteOnly     bool
	}

	schema := resourceSchema()
	configuration, ok := schema.Blocks["configuration"].(resourceschema.SingleNestedBlock)
	if !ok {
		t.Fatalf("configuration schema has type %T, want resourceschema.SingleNestedBlock", schema.Blocks["configuration"])
	}
	metrics, ok := configuration.Blocks["metrics"].(resourceschema.SingleNestedBlock)
	if !ok {
		t.Fatalf("metrics schema has type %T, want resourceschema.SingleNestedBlock", configuration.Blocks["metrics"])
	}
	managed, ok := metrics.Blocks["managed"].(resourceschema.SingleNestedBlock)
	if !ok {
		t.Fatalf("managed metrics schema has type %T, want resourceschema.SingleNestedBlock", metrics.Blocks["managed"])
	}
	custom, ok := metrics.Blocks["custom"].(resourceschema.SingleNestedBlock)
	if !ok {
		t.Fatalf("custom metrics schema has type %T, want resourceschema.SingleNestedBlock", metrics.Blocks["custom"])
	}
	password, ok := custom.Attributes["password"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("custom metrics password schema has type %T, want resourceschema.StringAttribute", custom.Attributes["password"])
	}

	expected := Expectation{MetricsChildCount: 2, ManagedAttributeCount: 1, CustomAttributeCount: 5, PasswordSensitive: true, PasswordWriteOnly: true}
	got := Expectation{
		MetricsChildCount:     len(metrics.Blocks),
		ManagedAttributeCount: len(managed.Attributes),
		CustomAttributeCount:  len(custom.Attributes),
		PasswordSensitive:     password.Sensitive,
		PasswordWriteOnly:     password.WriteOnly,
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("custom metrics schema mismatch (-want +got):\n%s", diff)
	}
}

func TestUpdateMetricsConfiguration(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Password          string
		DiagnosticSummary string
	}

	tests := []struct {
		Name     string
		Model    *MetricsModel
		Prior    *MetricsModel
		Password types.String
		Expected Expectation
	}{
		{
			Name: "resubmits_password_when_version_changes",
			Model: &MetricsModel{Custom: &CustomMetricsModel{
				PasswordVersion: types.StringValue("2"),
			}},
			Prior: &MetricsModel{Custom: &CustomMetricsModel{
				PasswordVersion: types.StringValue("1"),
			}},
			Password: types.StringValue("rotated-password"),
			Expected: Expectation{
				Password: "rotated-password",
			},
		},
		{
			Name: "requires_password_when_version_changes",
			Model: &MetricsModel{Custom: &CustomMetricsModel{
				PasswordVersion: types.StringValue("2"),
			}},
			Prior: &MetricsModel{Custom: &CustomMetricsModel{
				PasswordVersion: types.StringValue("1"),
			}},
			Password: types.StringNull(),
			Expected: Expectation{
				DiagnosticSummary: "Missing Custom Metrics Password",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			result, diags := updateMetricsConfiguration(tc.Model, tc.Prior, tc.Password)
			got := Expectation{}
			if result != nil && result.Password != nil {
				got.Password = *result.Password
			}
			if diags.HasError() {
				got.DiagnosticSummary = diags[0].Summary()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("updateMetricsConfiguration() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMetricsModelDoesNotStorePassword(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		HasCustom      bool
		PasswordIsNull bool
	}

	model := metricsModel(&v1.MetricsConfiguration{
		Enabled:  true,
		Url:      "https://metrics.example.com/api/v1/write",
		Username: "runner",
		Password: "secret",
	})
	got := Expectation{
		HasCustom:      model != nil && model.Custom != nil,
		PasswordIsNull: model != nil && model.Custom != nil && model.Custom.Password.IsNull(),
	}
	expected := Expectation{
		HasCustom:      true,
		PasswordIsNull: true,
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("metricsModel() mismatch (-want +got):\n%s", diff)
	}
}

func TestEnumMappings(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Provider v1.RunnerProvider
		OK       bool
	}

	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			Name:  "aws_ec2",
			Input: "aws_ec2",
			Expected: Expectation{
				Provider: v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2,
				OK:       true,
			},
		},
		{
			Name:  "gcp",
			Input: "gcp",
			Expected: Expectation{
				Provider: v1.RunnerProvider_RUNNER_PROVIDER_GCP,
				OK:       true,
			},
		},
		{
			Name:  "internal_provider_rejected",
			Input: "managed",
			Expected: Expectation{
				Provider: v1.RunnerProvider_RUNNER_PROVIDER_UNSPECIFIED,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			provider, ok := providerFromString(tc.Input)
			got := Expectation{
				Provider: provider,
				OK:       ok,
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("providerFromString() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseHour(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Hour int
		OK   bool
	}

	tests := []struct {
		Name     string
		Input    types.String
		Expected Expectation
	}{
		{
			Name:  "midnight",
			Input: types.StringValue("00:00"),
			Expected: Expectation{
				Hour: 0,
				OK:   true,
			},
		},
		{
			Name:  "last_hour",
			Input: types.StringValue("23:00"),
			Expected: Expectation{
				Hour: 23,
				OK:   true,
			},
		},
		{
			Name:  "minutes_not_supported",
			Input: types.StringValue("02:30"),
			Expected: Expectation{
				OK: false,
			},
		},
		{
			Name:  "out_of_range",
			Input: types.StringValue("24:00"),
			Expected: Expectation{
				OK: false,
			},
		},
		{
			Name:  "unknown",
			Input: types.StringUnknown(),
			Expected: Expectation{
				OK: false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			hour, ok := parseHour(tc.Input)
			got := Expectation{
				Hour: hour,
				OK:   ok,
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("parseHour() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
