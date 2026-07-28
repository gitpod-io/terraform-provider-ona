// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package tfvalue

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestSplitImportID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Parts             []string
		DiagnosticSummary string
		DiagnosticDetail  string
	}
	tests := []struct {
		Name     string
		ID       string
		Count    int
		Expected Expectation
	}{
		{Name: "valid", ID: "organization/team/mode", Count: 3, Expected: Expectation{Parts: []string{"organization", "team", "mode"}}},
		{Name: "incorrect_part_count", ID: "organization/team", Count: 3, Expected: Expectation{DiagnosticSummary: "Invalid Import ID", DiagnosticDetail: "Expected import ID format: organization_id/team_id/mode."}},
		{Name: "blank_part", ID: "organization/ /mode", Count: 3, Expected: Expectation{DiagnosticSummary: "Invalid Import ID", DiagnosticDetail: "Expected import ID format: organization_id/team_id/mode."}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			parts, diags := SplitImportID(tc.ID, tc.Count, "organization_id/team_id/mode")
			got := Expectation{Parts: parts}
			if len(diags) > 0 {
				got.DiagnosticSummary = diags[0].Summary()
				got.DiagnosticDetail = diags[0].Detail()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("SplitImportID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSetImportString(t *testing.T) {
	t.Parallel()

	schema := resourceschema.Schema{Attributes: map[string]resourceschema.Attribute{
		"id": resourceschema.StringAttribute{Optional: true},
	}}
	state := tfsdk.State{
		Raw: tftypes.NewValue(
			tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String}},
			map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, nil)},
		),
		Schema: schema,
	}
	resp := resource.ImportStateResponse{State: state}

	SetImportString(t.Context(), &resp, "id", "value")

	type Expectation struct {
		Value       types.String
		Diagnostics int
	}
	var value types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(t.Context(), path.Root("id"), &value)...)
	got := Expectation{Value: value, Diagnostics: len(resp.Diagnostics)}
	expected := Expectation{Value: types.StringValue("value")}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("SetImportString() mismatch (-want +got):\n%s", diff)
	}
}
