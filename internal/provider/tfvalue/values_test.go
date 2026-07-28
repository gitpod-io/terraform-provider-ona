// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package tfvalue

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestIsKnownString(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result bool
	}
	tests := []struct {
		Name     string
		Value    types.String
		Expected Expectation
	}{
		{Name: "null", Value: types.StringNull()},
		{Name: "unknown", Value: types.StringUnknown()},
		{Name: "empty", Value: types.StringValue("")},
		{Name: "known", Value: types.StringValue("value"), Expected: Expectation{Result: true}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			got := Expectation{Result: IsKnownString(tc.Value)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("IsKnownString() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsKnownBool(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result bool
	}
	tests := []struct {
		Name     string
		Value    types.Bool
		Expected Expectation
	}{
		{Name: "null", Value: types.BoolNull()},
		{Name: "unknown", Value: types.BoolUnknown()},
		{Name: "known_false", Value: types.BoolValue(false), Expected: Expectation{Result: true}},
		{Name: "known_true", Value: types.BoolValue(true), Expected: Expectation{Result: true}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			got := Expectation{Result: IsKnownBool(tc.Value)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("IsKnownBool() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPreserveString(t *testing.T) {
	t.Parallel()

	current := types.StringValue("observed")
	type Expectation struct {
		Result types.String
	}
	tests := []struct {
		Name     string
		Planned  types.String
		Expected Expectation
	}{
		{Name: "planned_null", Planned: types.StringNull(), Expected: Expectation{Result: current}},
		{Name: "planned_unknown", Planned: types.StringUnknown(), Expected: Expectation{Result: current}},
		{Name: "planned_empty", Planned: types.StringValue(""), Expected: Expectation{Result: types.StringValue("")}},
		{Name: "planned_known", Planned: types.StringValue("planned"), Expected: Expectation{Result: types.StringValue("planned")}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			got := Expectation{Result: PreserveString(current, tc.Planned)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("PreserveString() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPreserveBool(t *testing.T) {
	t.Parallel()

	current := types.BoolValue(true)
	type Expectation struct {
		Result types.Bool
	}
	tests := []struct {
		Name     string
		Planned  types.Bool
		Expected Expectation
	}{
		{Name: "planned_null", Planned: types.BoolNull(), Expected: Expectation{Result: current}},
		{Name: "planned_unknown", Planned: types.BoolUnknown(), Expected: Expectation{Result: current}},
		{Name: "planned_false", Planned: types.BoolValue(false), Expected: Expectation{Result: types.BoolValue(false)}},
		{Name: "planned_true", Planned: types.BoolValue(true), Expected: Expectation{Result: types.BoolValue(true)}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			got := Expectation{Result: PreserveBool(current, tc.Planned)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("PreserveBool() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestOptionalStringValue(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result types.String
	}
	tests := []struct {
		Name     string
		Value    string
		Expected Expectation
	}{
		{Name: "empty", Expected: Expectation{Result: types.StringNull()}},
		{Name: "non_empty", Value: "value", Expected: Expectation{Result: types.StringValue("value")}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			got := Expectation{Result: OptionalStringValue(tc.Value)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("OptionalStringValue() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTimestampRFC3339Value(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result types.String
	}
	tests := []struct {
		Name     string
		Value    *timestamppb.Timestamp
		Expected Expectation
	}{
		{Name: "nil", Expected: Expectation{Result: types.StringNull()}},
		{Name: "invalid", Value: &timestamppb.Timestamp{Seconds: 253402300800}, Expected: Expectation{Result: types.StringNull()}},
		{Name: "whole_second", Value: timestamppb.New(time.Date(2026, time.July, 28, 12, 34, 56, 0, time.FixedZone("offset", 2*60*60))), Expected: Expectation{Result: types.StringValue("2026-07-28T10:34:56Z")}},
		{Name: "fractional_second", Value: timestamppb.New(time.Date(2026, time.July, 28, 12, 34, 56, 123456789, time.UTC)), Expected: Expectation{Result: types.StringValue("2026-07-28T12:34:56Z")}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			got := Expectation{Result: TimestampRFC3339Value(tc.Value)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("TimestampRFC3339Value() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
