// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package tfvalue

import (
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// IsKnownString reports whether value is known, non-null, and non-empty.
func IsKnownString(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueString() != ""
}

// IsKnownBool reports whether value is known and non-null.
func IsKnownBool(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown()
}

// PreserveString returns planned when it is known and current otherwise.
func PreserveString(current, planned types.String) types.String {
	if planned.IsNull() || planned.IsUnknown() {
		return current
	}
	return planned
}

// PreserveBool returns planned when it is known and current otherwise.
func PreserveBool(current, planned types.Bool) types.Bool {
	if planned.IsNull() || planned.IsUnknown() {
		return current
	}
	return planned
}

// OptionalStringValue maps an empty Go string to Terraform null.
func OptionalStringValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

// TimestampRFC3339Value maps a valid protobuf timestamp to second-precision
// UTC RFC 3339 and maps nil or invalid timestamps to Terraform null.
func TimestampRFC3339Value(value *timestamppb.Timestamp) types.String {
	if value == nil || !value.IsValid() {
		return types.StringNull()
	}
	return types.StringValue(value.AsTime().UTC().Format(time.RFC3339))
}
