// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestReplaceWhenCommandRemoved(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		RequiresReplace bool
	}
	tests := []struct {
		Name       string
		StateRaw   tftypes.Value
		PlanRaw    tftypes.Value
		StateValue types.String
		PlanValue  types.String
		Expected   Expectation
	}{
		{Name: "create_does_not_replace", StateRaw: tftypes.NewValue(tftypes.String, nil), PlanRaw: tftypes.NewValue(tftypes.String, "new"), StateValue: types.StringNull(), PlanValue: types.StringValue("new")},
		{Name: "destroy_does_not_replace", StateRaw: tftypes.NewValue(tftypes.String, "old"), PlanRaw: tftypes.NewValue(tftypes.String, nil), StateValue: types.StringValue("old"), PlanValue: types.StringNull()},
		{Name: "addition_updates", StateRaw: tftypes.NewValue(tftypes.String, ""), PlanRaw: tftypes.NewValue(tftypes.String, "new"), StateValue: types.StringNull(), PlanValue: types.StringValue("new")},
		{Name: "rename_updates", StateRaw: tftypes.NewValue(tftypes.String, "old"), PlanRaw: tftypes.NewValue(tftypes.String, "new"), StateValue: types.StringValue("old"), PlanValue: types.StringValue("new")},
		{Name: "unknown_does_not_replace", StateRaw: tftypes.NewValue(tftypes.String, "old"), PlanRaw: tftypes.NewValue(tftypes.String, tftypes.UnknownValue), StateValue: types.StringValue("old"), PlanValue: types.StringUnknown()},
		{Name: "removal_replaces", StateRaw: tftypes.NewValue(tftypes.String, "old"), PlanRaw: tftypes.NewValue(tftypes.String, ""), StateValue: types.StringValue("old"), PlanValue: types.StringNull(), Expected: Expectation{RequiresReplace: true}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			request := planmodifier.StringRequest{
				State:      tfsdk.State{Raw: tc.StateRaw},
				Plan:       tfsdk.Plan{Raw: tc.PlanRaw},
				StateValue: tc.StateValue,
				PlanValue:  tc.PlanValue,
			}
			var response planmodifier.StringResponse
			replaceWhenCommandRemoved{}.PlanModifyString(t.Context(), request, &response)
			got := Expectation{RequiresReplace: response.RequiresReplace}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("PlanModifyString() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
