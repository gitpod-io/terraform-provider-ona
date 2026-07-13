// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package listutil

import (
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestStringList(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	value, diags := types.ListValueFrom(ctx, types.StringType, []string{"runner-1", "runner-2"})
	if diags.HasError() {
		t.Fatalf("create list value: %v", diags)
	}

	got, diags := StringList(ctx, value)
	if diags.HasError() {
		t.Fatalf("convert list value: %v", diags)
	}
	if len(got) != 2 || got[0] != "runner-1" || got[1] != "runner-2" {
		t.Fatalf("StringList() = %#v, want [runner-1 runner-2]", got)
	}
}

func TestStringListNullAndUnknown(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]types.List{
		"null":    types.ListNull(types.StringType),
		"unknown": types.ListUnknown(types.StringType),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, diags := StringList(t.Context(), value)
			if diags.HasError() {
				t.Fatalf("StringList() diagnostics: %v", diags)
			}
			if got != nil {
				t.Fatalf("StringList() = %#v, want nil", got)
			}
		})
	}
}

func TestLimitHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		limit    int64
		emitted  int64
		capacity bool
		pageSize int32
	}{
		{name: "unlimited", limit: 0, emitted: 500, capacity: true, pageSize: DefaultPageSize},
		{name: "remaining_below_page", limit: 25, emitted: 5, capacity: true, pageSize: 20},
		{name: "remaining_above_page", limit: 250, emitted: 5, capacity: true, pageSize: DefaultPageSize},
		{name: "limit_met", limit: 5, emitted: 5, capacity: false, pageSize: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := HasCapacity(tc.limit, tc.emitted); got != tc.capacity {
				t.Errorf("HasCapacity(%d, %d) = %t, want %t", tc.limit, tc.emitted, got, tc.capacity)
			}
			if got := PageSize(tc.limit, tc.emitted); got != tc.pageSize {
				t.Errorf("PageSize(%d, %d) = %d, want %d", tc.limit, tc.emitted, got, tc.pageSize)
			}
		})
	}
}

func TestError(t *testing.T) {
	t.Parallel()

	result := Error("Unable to List", errors.New("API failed"))
	if !result.Diagnostics.HasError() {
		t.Fatal("Error() did not return an error diagnostic")
	}
}
