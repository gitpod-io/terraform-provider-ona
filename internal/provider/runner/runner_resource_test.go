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

func TestMetricsPasswordSchema(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Sensitive bool
		WriteOnly bool
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
	password, ok := metrics.Attributes["password"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("password schema has type %T, want resourceschema.StringAttribute", metrics.Attributes["password"])
	}

	expected := Expectation{Sensitive: true, WriteOnly: true}
	got := Expectation{Sensitive: password.Sensitive, WriteOnly: password.WriteOnly}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("metrics password schema mismatch (-want +got):\n%s", diff)
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
