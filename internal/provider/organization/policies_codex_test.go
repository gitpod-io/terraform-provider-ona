// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestCodexModelPolicyFromMap(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Policy *v1.CodexModelPolicy
		Errors []string
	}
	tests := []struct {
		Name     string
		Value    types.Map
		Expected Expectation
	}{
		{
			Name:  "null_is_unmanaged",
			Value: types.MapNull(types.StringType),
		},
		{
			Name:  "unknown_is_unmanaged",
			Value: types.MapUnknown(types.StringType),
		},
		{
			Name:  "empty_map_clears_policy",
			Value: stringMapValue(map[string]string{}),
			Expected: Expectation{Policy: &v1.CodexModelPolicy{
				ModelStates: map[string]v1.CodexModelPolicyState{},
			}},
		},
		{
			Name:  "allowed",
			Value: stringMapValue(map[string]string{"CODEX_OPEN_AI_MODEL_GPT_5_5": "allowed"}),
			Expected: Expectation{Policy: &v1.CodexModelPolicy{ModelStates: map[string]v1.CodexModelPolicyState{
				"CODEX_OPEN_AI_MODEL_GPT_5_5": v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_ALLOWED,
			}}},
		},
		{
			Name:  "disabled",
			Value: stringMapValue(map[string]string{"CODEX_OPEN_AI_MODEL_GPT_5_6_SOL": "disabled"}),
			Expected: Expectation{Policy: &v1.CodexModelPolicy{ModelStates: map[string]v1.CodexModelPolicyState{
				"CODEX_OPEN_AI_MODEL_GPT_5_6_SOL": v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_DISABLED,
			}}},
		},
		{
			Name: "mixed",
			Value: stringMapValue(map[string]string{
				"CODEX_OPEN_AI_MODEL_GPT_5_5":     "allowed",
				"CODEX_OPEN_AI_MODEL_GPT_5_6_SOL": "disabled",
			}),
			Expected: Expectation{Policy: &v1.CodexModelPolicy{ModelStates: map[string]v1.CodexModelPolicyState{
				"CODEX_OPEN_AI_MODEL_GPT_5_5":     v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_ALLOWED,
				"CODEX_OPEN_AI_MODEL_GPT_5_6_SOL": v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_DISABLED,
			}}},
		},
		{
			Name:  "invalid_state",
			Value: stringMapValue(map[string]string{"CODEX_OPEN_AI_MODEL_GPT_5_5": "unspecified"}),
			Expected: Expectation{
				Policy: &v1.CodexModelPolicy{ModelStates: map[string]v1.CodexModelPolicyState{}},
				Errors: []string{"Invalid Codex Model State"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			policy := codexModelPolicyFromMap(t.Context(), tc.Value, &diags)
			got := Expectation{Policy: policy, Errors: diagSummaries(diags)}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform(), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("codexModelPolicyFromMap() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCodexModelStatesToMap(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		State  string
		Values map[string]string
		Errors []string
	}
	tests := []struct {
		Name              string
		Policy            *v1.CodexModelPolicy
		Prior             types.Map
		PopulateUnmanaged bool
		Expected          Expectation
	}{
		{
			Name:     "null_remains_unmanaged",
			Policy:   disabledCodexPolicy("CODEX_OPEN_AI_MODEL_GPT_5_5"),
			Prior:    types.MapNull(types.StringType),
			Expected: Expectation{State: "null"},
		},
		{
			Name:     "unknown_remains_unmanaged",
			Policy:   disabledCodexPolicy("CODEX_OPEN_AI_MODEL_GPT_5_5"),
			Prior:    types.MapUnknown(types.StringType),
			Expected: Expectation{State: "unknown"},
		},
		{
			Name:              "import_populates_disabled",
			Policy:            disabledCodexPolicy("CODEX_OPEN_AI_MODEL_GPT_5_5"),
			Prior:             types.MapNull(types.StringType),
			PopulateUnmanaged: true,
			Expected: Expectation{State: "known", Values: map[string]string{
				"CODEX_OPEN_AI_MODEL_GPT_5_5": "disabled",
			}},
		},
		{
			Name:   "canonicalized_allowed_is_retained",
			Policy: &v1.CodexModelPolicy{},
			Prior:  stringMapValue(map[string]string{"CODEX_OPEN_AI_MODEL_GPT_5_5": "allowed"}),
			Expected: Expectation{State: "known", Values: map[string]string{
				"CODEX_OPEN_AI_MODEL_GPT_5_5": "allowed",
			}},
		},
		{
			Name:   "removed_disabled_entry_is_not_retained",
			Policy: &v1.CodexModelPolicy{},
			Prior:  stringMapValue(map[string]string{"CODEX_OPEN_AI_MODEL_GPT_5_5": "disabled"}),
			Expected: Expectation{
				State:  "known",
				Values: map[string]string{},
			},
		},
		{
			Name:   "remote_disabled_wins_over_prior_allowed",
			Policy: disabledCodexPolicy("CODEX_OPEN_AI_MODEL_GPT_5_5"),
			Prior:  stringMapValue(map[string]string{"CODEX_OPEN_AI_MODEL_GPT_5_5": "allowed"}),
			Expected: Expectation{State: "known", Values: map[string]string{
				"CODEX_OPEN_AI_MODEL_GPT_5_5": "disabled",
			}},
		},
		{
			Name: "explicit_allowed_and_unspecified_render_allowed",
			Policy: &v1.CodexModelPolicy{ModelStates: map[string]v1.CodexModelPolicyState{
				"CODEX_OPEN_AI_MODEL_GPT_5_5":     v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_ALLOWED,
				"CODEX_OPEN_AI_MODEL_GPT_5_6_SOL": v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_UNSPECIFIED,
			}},
			Prior:             types.MapNull(types.StringType),
			PopulateUnmanaged: true,
			Expected: Expectation{State: "known", Values: map[string]string{
				"CODEX_OPEN_AI_MODEL_GPT_5_5":     "allowed",
				"CODEX_OPEN_AI_MODEL_GPT_5_6_SOL": "allowed",
			}},
		},
		{
			Name: "unknown_remote_model_and_state_report_errors",
			Policy: &v1.CodexModelPolicy{ModelStates: map[string]v1.CodexModelPolicyState{
				"CODEX_OPEN_AI_MODEL_FUTURE":  v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_DISABLED,
				"CODEX_OPEN_AI_MODEL_GPT_5_5": v1.CodexModelPolicyState(99),
			}},
			Prior:             types.MapNull(types.StringType),
			PopulateUnmanaged: true,
			Expected: Expectation{
				State:  "known",
				Values: map[string]string{},
				Errors: []string{"Unknown Codex Model State", "Unknown Codex Model"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			value := codexModelStatesToMap(t.Context(), tc.Policy, tc.Prior, tc.PopulateUnmanaged, &diags)
			state, values := stringMapSnapshot(t.Context(), value, &diags)
			got := Expectation{State: state, Values: values, Errors: diagSummaries(diags)}

			if diff := cmp.Diff(tc.Expected, got, cmpopts.EquateEmpty(), cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
				t.Errorf("codexModelStatesToMap() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateCodexModelStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Value    types.Map
		Expected []string
	}{
		{Name: "null", Value: types.MapNull(types.StringType)},
		{Name: "unknown", Value: types.MapUnknown(types.StringType)},
		{Name: "empty", Value: stringMapValue(map[string]string{})},
		{Name: "valid", Value: stringMapValue(map[string]string{"CODEX_OPEN_AI_MODEL_GPT_5_5": "allowed"})},
		{
			Name:     "unspecified_model",
			Value:    stringMapValue(map[string]string{"CODEX_OPEN_AI_MODEL_UNSPECIFIED": "disabled"}),
			Expected: []string{"Invalid Codex Model"},
		},
		{
			Name:     "unknown_model",
			Value:    stringMapValue(map[string]string{"CODEX_OPEN_AI_MODEL_FUTURE": "disabled"}),
			Expected: []string{"Invalid Codex Model"},
		},
		{
			Name:     "invalid_state",
			Value:    stringMapValue(map[string]string{"CODEX_OPEN_AI_MODEL_GPT_5_5": "unspecified"}),
			Expected: []string{"Invalid Codex Model State"},
		},
		{
			Name: "unknown_element_is_deferred",
			Value: types.MapValueMust(types.StringType, map[string]attr.Value{
				"CODEX_OPEN_AI_MODEL_GPT_5_5": types.StringUnknown(),
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateCodexModelStates(path.Root("agent_policy").AtName("codex_model_states"), tc.Value, &diags)
			got := diagSummaries(diags)

			if diff := cmp.Diff(tc.Expected, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("validateCodexModelStates() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCodexModelPolicyBaselineAndRestore(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Policy *v1.CodexModelPolicy
	}
	tests := []struct {
		Name     string
		Policy   *v1.CodexModelPolicy
		Expected Expectation
	}{
		{
			Name: "nil_baseline_restores_effective_default",
			Expected: Expectation{
				Policy: &v1.CodexModelPolicy{},
			},
		},
		{
			Name:   "empty_baseline_remains_present",
			Policy: &v1.CodexModelPolicy{},
			Expected: Expectation{
				Policy: &v1.CodexModelPolicy{},
			},
		},
		{
			Name:   "populated_baseline_is_restored",
			Policy: disabledCodexPolicy("CODEX_OPEN_AI_MODEL_GPT_5_4"),
			Expected: Expectation{
				Policy: disabledCodexPolicy("CODEX_OPEN_AI_MODEL_GPT_5_4"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			original := &v1.OrganizationPolicies{AgentPolicy: &v1.AgentPolicy{CodexModelPolicy: tc.Policy}}
			snapshot := managedPoliciesSnapshot(original)
			if len(tc.Policy.GetModelStates()) > 0 {
				original.AgentPolicy.CodexModelPolicy.ModelStates["CODEX_OPEN_AI_MODEL_GPT_5_5"] = v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_DISABLED
			}
			restored := restorePoliciesRequest("org-1", snapshot, &v1.OrganizationPolicies{}).GetAgentPolicy()
			got := Expectation{Policy: restored.GetCodexModelPolicy()}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("Codex model policy baseline mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func stringMapValue(values map[string]string) types.Map {
	return types.MapValueMust(types.StringType, stringMapElements(values))
}

func stringMapElements(values map[string]string) map[string]attr.Value {
	result := make(map[string]attr.Value, len(values))
	for key, value := range values {
		result[key] = types.StringValue(value)
	}
	return result
}

func stringMapSnapshot(ctx context.Context, value types.Map, diags *diag.Diagnostics) (string, map[string]string) {
	if value.IsNull() {
		return "null", nil
	}
	if value.IsUnknown() {
		return "unknown", nil
	}
	var result map[string]string
	diags.Append(value.ElementsAs(ctx, &result, false)...)
	return "known", result
}

func disabledCodexPolicy(model string) *v1.CodexModelPolicy {
	return &v1.CodexModelPolicy{ModelStates: map[string]v1.CodexModelPolicyState{
		model: v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_DISABLED,
	}}
}
