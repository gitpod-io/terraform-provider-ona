// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package security

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPolicyResourceUpgradeStateV0(t *testing.T) {
	ctx := t.Context()
	upgrader := (&PolicyResource{}).UpgradeState(ctx)[0]
	priorState := tfsdk.State{Schema: *upgrader.PriorSchema}
	prior := policyModelV0{
		ID:             types.StringValue("policy-1"),
		OrganizationID: types.StringValue("org-1"),
		Name:           types.StringValue("baseline"),
		CreatedAt:      types.StringValue("2026-07-01T00:00:00Z"),
		UpdatedAt:      types.StringValue("2026-07-02T00:00:00Z"),
		Spec: &policySpecModelV0{
			Ports: &portPolicyModelV0{
				DefaultEffect: types.StringValue(effectAllow),
				Rules: []portRuleModelV0{{
					RangeFrom: types.Int64Value(22),
					RangeTo:   types.Int64Value(22),
					Effect:    types.StringValue(effectBlock),
				}},
			},
			Executables: &ExecutablePolicyModel{
				DefaultEffect: types.StringValue(effectAllow),
				Rules: []ExecutableRuleModel{{
					Path:   types.StringValue("/usr/bin/nc"),
					Effect: types.StringValue(effectAudit),
				}},
			},
			Files: &filePolicyModelV0{
				DefaultEffect:  types.StringValue(effectAllow),
				DefaultActions: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("read")}),
			},
			BlockDevices: &blockDevicePolicyModelV0{DefaultEffect: types.StringValue(effectBlock)},
			Data:         &dataPolicyModelV0{DefaultEffect: types.StringValue(effectAudit)},
		},
	}
	if diags := priorState.Set(ctx, &prior); diags.HasError() {
		t.Fatalf("setting prior state: %v", diags)
	}

	currentSchema := policyResourceSchema()
	response := resource.UpgradeStateResponse{State: tfsdk.State{Schema: currentSchema}}
	upgrader.StateUpgrader(ctx, resource.UpgradeStateRequest{State: &priorState}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("upgrading state: %v", response.Diagnostics)
	}

	var upgraded PolicyModel
	if diags := response.State.Get(ctx, &upgraded); diags.HasError() {
		t.Fatalf("reading upgraded state: %v", diags)
	}

	type stateSnapshot struct {
		ID                      string
		OrganizationID          string
		Name                    string
		CreatedAt               string
		UpdatedAt               string
		ExecutableDefaultEffect string
		ExecutableRulePath      string
		ExecutableRuleEffect    string
	}
	want := stateSnapshot{
		ID:                      "policy-1",
		OrganizationID:          "org-1",
		Name:                    "baseline",
		CreatedAt:               "2026-07-01T00:00:00Z",
		UpdatedAt:               "2026-07-02T00:00:00Z",
		ExecutableDefaultEffect: effectAllow,
		ExecutableRulePath:      "/usr/bin/nc",
		ExecutableRuleEffect:    effectAudit,
	}
	got := stateSnapshot{
		ID:                      upgraded.ID.ValueString(),
		OrganizationID:          upgraded.OrganizationID.ValueString(),
		Name:                    upgraded.Name.ValueString(),
		CreatedAt:               upgraded.CreatedAt.ValueString(),
		UpdatedAt:               upgraded.UpdatedAt.ValueString(),
		ExecutableDefaultEffect: upgraded.Spec.Executables.DefaultEffect.ValueString(),
		ExecutableRulePath:      upgraded.Spec.Executables.Rules[0].Path.ValueString(),
		ExecutableRuleEffect:    upgraded.Spec.Executables.Rules[0].Effect.ValueString(),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("upgraded state mismatch (-want +got):\n%s", diff)
	}
}
