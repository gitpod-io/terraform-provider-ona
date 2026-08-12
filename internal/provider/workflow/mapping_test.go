// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	"testing"
	"time"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCreateWorkflowRequest(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Request *v1.CreateWorkflowRequest
		Errors  []string
	}
	tests := []struct {
		Name     string
		Mutate   func(*Model)
		Expected Expectation
	}{
		{
			Name: "maps_core_workflow",
			Expected: Expectation{Request: &v1.CreateWorkflowRequest{
				Name: "Nightly checks", Description: "Runs checks", Executor: &v1.Subject{Id: testServiceAccountID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT},
				AgentId: codexAgentID,
				Triggers: []*v1.WorkflowTrigger{{
					Trigger: &v1.WorkflowTrigger_Manual_{Manual: &v1.WorkflowTrigger_Manual{}},
					Context: &v1.WorkflowTriggerContext{Context: &v1.WorkflowTriggerContext_Projects_{Projects: &v1.WorkflowTriggerContext_Projects{ProjectIds: []string{testProjectID}}}},
				}},
				Action: &v1.WorkflowAction{
					Limits: &v1.WorkflowAction_Limits{MaxParallel: 2, MaxTotal: 10},
					Steps:  []*v1.WorkflowStep{{Step: &v1.WorkflowStep_Task_{Task: &v1.WorkflowStep_Task{Command: "make test"}}}},
				},
			}},
		},
		{
			Name: "maps_full_codex_settings",
			Mutate: func(model *Model) {
				model.CodexSettings = mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
					Model: types.StringValue("gpt-5.6-sol"), ReasoningEffort: types.StringValue("high"), ServiceTier: types.StringValue("fast"),
				})
			},
			Expected: Expectation{Request: &v1.CreateWorkflowRequest{
				Name: "Nightly checks", Description: "Runs checks", Executor: &v1.Subject{Id: testServiceAccountID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}, AgentId: codexAgentID,
				CodexSettings: &v1.CodexSettings{
					Model: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_SOL, ReasoningEffort: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_HIGH, ServiceTier: v1.CodexServiceTier_CODEX_SERVICE_TIER_FAST,
				},
				Triggers: []*v1.WorkflowTrigger{{
					Trigger: &v1.WorkflowTrigger_Manual_{Manual: &v1.WorkflowTrigger_Manual{}},
					Context: &v1.WorkflowTriggerContext{Context: &v1.WorkflowTriggerContext_Projects_{Projects: &v1.WorkflowTriggerContext_Projects{ProjectIds: []string{testProjectID}}}},
				}},
				Action: &v1.WorkflowAction{
					Limits: &v1.WorkflowAction_Limits{MaxParallel: 2, MaxTotal: 10},
					Steps:  []*v1.WorkflowStep{{Step: &v1.WorkflowStep_Task_{Task: &v1.WorkflowStep_Task{Command: "make test"}}}},
				},
			}},
		},
		{
			Name: "maps_empty_codex_settings",
			Mutate: func(model *Model) {
				model.CodexSettings = mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
					Model: types.StringNull(), ReasoningEffort: types.StringNull(), ServiceTier: types.StringNull(),
				})
			},
			Expected: Expectation{Request: &v1.CreateWorkflowRequest{
				Name: "Nightly checks", Description: "Runs checks", Executor: &v1.Subject{Id: testServiceAccountID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}, AgentId: codexAgentID,
				CodexSettings: &v1.CodexSettings{},
				Triggers: []*v1.WorkflowTrigger{{
					Trigger: &v1.WorkflowTrigger_Manual_{Manual: &v1.WorkflowTrigger_Manual{}},
					Context: &v1.WorkflowTriggerContext{Context: &v1.WorkflowTriggerContext_Projects_{Projects: &v1.WorkflowTriggerContext_Projects{ProjectIds: []string{testProjectID}}}},
				}},
				Action: &v1.WorkflowAction{
					Limits: &v1.WorkflowAction_Limits{MaxParallel: 2, MaxTotal: 10},
					Steps:  []*v1.WorkflowStep{{Step: &v1.WorkflowStep_Task_{Task: &v1.WorkflowStep_Task{Command: "make test"}}}},
				},
			}},
		},
		{
			Name: "rejects_deprecated_mini_model",
			Mutate: func(model *Model) {
				model.CodexSettings = mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
					Model: types.StringValue("gpt-5.4-mini"), ReasoningEffort: types.StringNull(), ServiceTier: types.StringNull(),
				})
			},
			Expected: Expectation{Errors: []string{"Unsupported Value"}},
		},
		{
			Name: "rejects_ona_agent",
			Mutate: func(model *Model) {
				model.Agent = types.StringValue(agentOna)
			},
			Expected: Expectation{Errors: []string{"Unsupported Automation Agent"}},
		},
		{
			Name: "rejects_parallel_above_total",
			Mutate: func(model *Model) {
				var action ActionModel
				mustObjectAs(t, model.Action, &action)
				var limits LimitsModel
				mustObjectAs(t, action.Limits, &limits)
				limits.MaxParallel = types.Int32Value(11)
				action.Limits = mustObjectValue(t, limitsAttributeTypes, limits)
				model.Action = mustObjectValue(t, actionAttributeTypes, action)
			},
			Expected: Expectation{Errors: []string{"Invalid Automation Action Limits"}},
		},
		{
			Name: "rejects_missing_pull_request_source",
			Mutate: func(model *Model) {
				pullRequest := PullRequestTriggerModel{
					Events: mustSetValue(t, types.StringType, []string{"opened"}), WebhookID: types.StringNull(), IntegrationID: types.StringNull(),
				}
				context := ContextModel{
					Projects: types.ObjectNull(projectsContextAttributeTypes), Repositories: types.ObjectNull(repositoriesContextAttributeTypes), Agent: types.ObjectNull(agentContextAttributeTypes),
					FromTrigger: types.ObjectValueMust(emptyAttributeTypes, map[string]attr.Value{}),
				}
				trigger := TriggerModel{
					Manual: types.ObjectNull(emptyAttributeTypes), Time: types.ObjectNull(timeTriggerAttributeTypes),
					PullRequest: mustObjectValue(t, pullRequestTriggerAttributeTypes, pullRequest), Context: mustObjectValue(t, contextAttributeTypes, context),
				}
				model.Triggers = mustListValue(t, types.ObjectType{AttrTypes: triggerAttributeTypes}, []TriggerModel{trigger})
			},
			Expected: Expectation{Errors: []string{"Missing Pull-Request Event Source"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			model := testWorkflowModel(t)
			if tc.Mutate != nil {
				tc.Mutate(&model)
			}
			request, diags := createWorkflowRequest(t.Context(), model)
			got := Expectation{Request: request, Errors: diagnosticSummaries(diags)}
			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("createWorkflowRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateWorkflowRequestCodexSettings(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Settings   *v1.CodexSettings
		AgentIDSet bool
		Errors     []string
	}
	tests := []struct {
		Name     string
		Settings types.Object
		Expected Expectation
	}{
		{
			Name:     "omission_clears_to_defaults",
			Settings: types.ObjectNull(codexSettingsAttributeTypes),
			Expected: Expectation{Settings: &v1.CodexSettings{}, AgentIDSet: true},
		},
		{
			Name: "empty_object_selects_defaults",
			Settings: mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
				Model: types.StringNull(), ReasoningEffort: types.StringNull(), ServiceTier: types.StringNull(),
			}),
			Expected: Expectation{Settings: &v1.CodexSettings{}, AgentIDSet: true},
		},
		{
			Name: "partial_object_maps_configured_value",
			Settings: mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
				Model: types.StringNull(), ReasoningEffort: types.StringValue("xhigh"), ServiceTier: types.StringNull(),
			}),
			Expected: Expectation{Settings: &v1.CodexSettings{ReasoningEffort: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_EXTRA_HIGH}, AgentIDSet: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			model := testWorkflowModel(t)
			model.ID = types.StringValue(testWorkflowID)
			model.CodexSettings = tc.Settings
			request, diags := updateWorkflowRequest(t.Context(), model)
			got := Expectation{Errors: diagnosticSummaries(diags)}
			if request != nil {
				got.Settings = request.GetCodexSettings()
				got.AgentIDSet = request.AgentId != nil
			}
			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("updateWorkflowRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateWorkflowRequestRejectsOnaAgent(t *testing.T) {
	t.Parallel()

	model := testWorkflowModel(t)
	model.ID = types.StringValue(testWorkflowID)
	model.Agent = types.StringValue(agentOna)
	request, diags := updateWorkflowRequest(t.Context(), model)

	type Expectation struct {
		Request *v1.UpdateWorkflowRequest
		Errors  []string
	}
	expected := Expectation{Errors: []string{"Unsupported Automation Agent"}}
	got := Expectation{Request: request, Errors: diagnosticSummaries(diags)}
	if diff := cmp.Diff(expected, got, protocmp.Transform()); diff != "" {
		t.Errorf("updateWorkflowRequest() mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateWorkflowRequestPartialCodexSettings(t *testing.T) {
	t.Parallel()

	model := testWorkflowModel(t)
	model.CodexSettings = mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
		Model: types.StringNull(), ReasoningEffort: types.StringValue("low"), ServiceTier: types.StringNull(),
	})
	request, diags := createWorkflowRequest(t.Context(), model)
	type Expectation struct {
		AgentID  string
		Settings *v1.CodexSettings
		Errors   []string
	}
	got := Expectation{Errors: diagnosticSummaries(diags)}
	if request != nil {
		got.AgentID = request.GetAgentId()
		got.Settings = request.GetCodexSettings()
	}
	expected := Expectation{
		AgentID: codexAgentID,
		Settings: &v1.CodexSettings{
			ReasoningEffort: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_LOW,
		},
	}
	if diff := cmp.Diff(expected, got, protocmp.Transform()); diff != "" {
		t.Errorf("createWorkflowRequest() mismatch (-want +got):\n%s", diff)
	}
}

func TestCodexModelMapping(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		FromRemote v1.CodexOpenAIModel
		FromOK     bool
		ToLocal    string
		ToOK       bool
	}
	tests := []struct {
		Name     string
		Local    string
		Remote   v1.CodexOpenAIModel
		Expected Expectation
	}{
		{Name: "gpt_5_5", Local: "gpt-5.5", Remote: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_5, Expected: Expectation{FromRemote: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_5, FromOK: true, ToLocal: "gpt-5.5", ToOK: true}},
		{Name: "gpt_5_4", Local: "gpt-5.4", Remote: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_4, Expected: Expectation{FromRemote: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_4, FromOK: true, ToLocal: "gpt-5.4", ToOK: true}},
		{Name: "deprecated_gpt_5_3_codex_is_read_only", Local: "gpt-5.3-codex", Remote: codexOpenAIModelGPT53Codex, Expected: Expectation{ToLocal: "gpt-5.3-codex", ToOK: true}},
		{Name: "deprecated_gpt_5_3_codex_spark_is_read_only", Local: "gpt-5.3-codex-spark", Remote: codexOpenAIModelGPT53CodexSpark, Expected: Expectation{ToLocal: "gpt-5.3-codex-spark", ToOK: true}},
		{Name: "deprecated_gpt_5_2_is_read_only", Local: "gpt-5.2", Remote: codexOpenAIModelGPT52, Expected: Expectation{ToLocal: "gpt-5.2", ToOK: true}},
		{Name: "gpt_5_6_sol", Local: "gpt-5.6-sol", Remote: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_SOL, Expected: Expectation{FromRemote: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_SOL, FromOK: true, ToLocal: "gpt-5.6-sol", ToOK: true}},
		{Name: "gpt_5_6_terra", Local: "gpt-5.6-terra", Remote: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_TERRA, Expected: Expectation{FromRemote: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_TERRA, FromOK: true, ToLocal: "gpt-5.6-terra", ToOK: true}},
		{Name: "gpt_5_6_luna", Local: "gpt-5.6-luna", Remote: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_LUNA, Expected: Expectation{FromRemote: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_LUNA, FromOK: true, ToLocal: "gpt-5.6-luna", ToOK: true}},
		{Name: "deprecated_mini_canonicalizes_on_read", Local: "gpt-5.4-mini", Remote: codexOpenAIModelGPT54Mini, Expected: Expectation{ToLocal: "gpt-5.4", ToOK: true}},
		{Name: "unsupported", Local: "future", Remote: v1.CodexOpenAIModel(99), Expected: Expectation{}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			remote, fromOK := codexModelFromString(tc.Local)
			local, toOK := codexModelToString(tc.Remote)
			got := Expectation{FromRemote: remote, FromOK: fromOK, ToLocal: local, ToOK: toOK}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("Codex model mapping mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCodexReasoningEffortMapping(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		FromRemote v1.CodexReasoningEffort
		FromOK     bool
		ToLocal    string
		ToOK       bool
	}
	tests := []struct {
		Name     string
		Local    string
		Remote   v1.CodexReasoningEffort
		Expected Expectation
	}{
		{Name: "low", Local: "low", Remote: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_LOW, Expected: Expectation{FromRemote: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_LOW, FromOK: true, ToLocal: "low", ToOK: true}},
		{Name: "medium", Local: "medium", Remote: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_MEDIUM, Expected: Expectation{FromRemote: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_MEDIUM, FromOK: true, ToLocal: "medium", ToOK: true}},
		{Name: "high", Local: "high", Remote: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_HIGH, Expected: Expectation{FromRemote: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_HIGH, FromOK: true, ToLocal: "high", ToOK: true}},
		{Name: "xhigh", Local: "xhigh", Remote: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_EXTRA_HIGH, Expected: Expectation{FromRemote: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_EXTRA_HIGH, FromOK: true, ToLocal: "xhigh", ToOK: true}},
		{Name: "max_is_read_only", Local: "max", Remote: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_MAX, Expected: Expectation{ToLocal: "max", ToOK: true}},
		{Name: "ultra_is_read_only", Local: "ultra", Remote: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_ULTRA, Expected: Expectation{ToLocal: "ultra", ToOK: true}},
		{Name: "unsupported", Local: "future", Remote: v1.CodexReasoningEffort(99), Expected: Expectation{}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			remote, fromOK := codexReasoningEffortFromString(tc.Local)
			local, toOK := codexReasoningEffortToString(tc.Remote)
			got := Expectation{FromRemote: remote, FromOK: fromOK, ToLocal: local, ToOK: toOK}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("Codex reasoning-effort mapping mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCodexServiceTierMapping(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Remote v1.CodexServiceTier
		Local  string
		OK     bool
	}
	tests := []struct {
		Name     string
		Local    string
		Remote   v1.CodexServiceTier
		Expected Expectation
	}{
		{Name: "fast", Local: "fast", Remote: v1.CodexServiceTier_CODEX_SERVICE_TIER_FAST, Expected: Expectation{Remote: v1.CodexServiceTier_CODEX_SERVICE_TIER_FAST, Local: "fast", OK: true}},
		{Name: "unsupported", Local: "future", Remote: v1.CodexServiceTier(99), Expected: Expectation{}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			remote, fromOK := codexServiceTierFromString(tc.Local)
			local, toOK := codexServiceTierToString(tc.Remote)
			got := Expectation{Remote: remote, Local: local, OK: fromOK && toOK}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("Codex service-tier mapping mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCodexSettingsToObject(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Null            bool
		Model           string
		ReasoningEffort string
		ServiceTier     string
		Errors          []string
	}
	tests := []struct {
		Name     string
		Remote   *v1.CodexSettings
		Expected Expectation
	}{
		{Name: "nil_is_null", Expected: Expectation{Null: true}},
		{Name: "empty_is_default_object", Remote: &v1.CodexSettings{}, Expected: Expectation{}},
		{
			Name:     "maps_full_settings",
			Remote:   &v1.CodexSettings{Model: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_TERRA, ReasoningEffort: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_MEDIUM, ServiceTier: v1.CodexServiceTier_CODEX_SERVICE_TIER_FAST},
			Expected: Expectation{Model: "gpt-5.6-terra", ReasoningEffort: "medium", ServiceTier: "fast"},
		},
		{Name: "canonicalizes_mini", Remote: &v1.CodexSettings{Model: codexOpenAIModelGPT54Mini}, Expected: Expectation{Model: "gpt-5.4"}},
		{Name: "rejects_unknown_remote_model", Remote: &v1.CodexSettings{Model: v1.CodexOpenAIModel(99)}, Expected: Expectation{Null: true, Errors: []string{"Unsupported Remote Codex Model"}}},
		{Name: "rejects_unknown_remote_reasoning_effort", Remote: &v1.CodexSettings{ReasoningEffort: v1.CodexReasoningEffort(99)}, Expected: Expectation{Null: true, Errors: []string{"Unsupported Remote Codex Reasoning Effort"}}},
		{Name: "rejects_unknown_remote_service_tier", Remote: &v1.CodexSettings{ServiceTier: v1.CodexServiceTier(99)}, Expected: Expectation{Null: true, Errors: []string{"Unsupported Remote Codex Service Tier"}}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var diags diag.Diagnostics
			value := codexSettingsToObject(t.Context(), tc.Remote, &diags)
			got := Expectation{Null: value.IsNull(), Errors: diagnosticSummaries(diags)}
			if !value.IsNull() {
				var model CodexSettingsModel
				mustObjectAs(t, value, &model)
				if !model.Model.IsNull() {
					got.Model = model.Model.ValueString()
				}
				if !model.ReasoningEffort.IsNull() {
					got.ReasoningEffort = model.ReasoningEffort.ValueString()
				}
				if !model.ServiceTier.IsNull() {
					got.ServiceTier = model.ServiceTier.ValueString()
				}
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("codexSettingsToObject() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPreserveDefaultCodexSettings(t *testing.T) {
	t.Parallel()

	empty := mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{Model: types.StringNull(), ReasoningEffort: types.StringNull(), ServiceTier: types.StringNull()})
	nonDefault := mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{Model: types.StringValue("gpt-5.5"), ReasoningEffort: types.StringNull(), ServiceTier: types.StringNull()})
	tests := []struct {
		Name     string
		Target   types.Object
		Source   types.Object
		Expected codexSettingsSnapshot
	}{
		{Name: "preserves_null_over_remote_empty", Target: empty, Source: types.ObjectNull(codexSettingsAttributeTypes), Expected: codexSettingsSnapshot{Null: true}},
		{Name: "preserves_empty_over_remote_null", Target: types.ObjectNull(codexSettingsAttributeTypes), Source: empty, Expected: codexSettingsSnapshot{}},
		{Name: "does_not_preserve_stale_non_default", Target: empty, Source: nonDefault, Expected: codexSettingsSnapshot{}},
		{Name: "keeps_remote_non_default", Target: nonDefault, Source: empty, Expected: codexSettingsSnapshot{Model: "gpt-5.5"}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			target := tc.Target
			preserveDefaultCodexSettings(&target, tc.Source)
			got := snapshotCodexSettings(t, target)
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("preserveDefaultCodexSettings() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWorkflowUsesCodex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Workflow *v1.Workflow
		Expected bool
	}{
		{Name: "codex", Workflow: &v1.Workflow{Spec: &v1.Workflow_Spec{AgentId: codexAgentID}}, Expected: true},
		{Name: "unpinned", Workflow: &v1.Workflow{Spec: &v1.Workflow_Spec{}}, Expected: true},
		{Name: "other_agent", Workflow: &v1.Workflow{Spec: &v1.Workflow_Spec{AgentId: "00000000-0000-0000-0000-000000009999"}}},
		{Name: "missing_spec", Workflow: &v1.Workflow{}},
		{Name: "nil_workflow"},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			got := workflowUsesCodex(tc.Workflow)
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("workflowUsesCodex() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateCodexSettings(t *testing.T) {
	t.Parallel()

	empty := func() CodexSettingsModel {
		return CodexSettingsModel{Model: types.StringNull(), ReasoningEffort: types.StringNull(), ServiceTier: types.StringNull()}
	}
	tests := []struct {
		Name         string
		Settings     types.Object
		RequireKnown bool
		Expected     []string
	}{
		{Name: "accepts_null", Settings: types.ObjectNull(codexSettingsAttributeTypes)},
		{Name: "accepts_empty", Settings: mustObjectValue(t, codexSettingsAttributeTypes, empty())},
		{Name: "defers_unknown_object", Settings: types.ObjectUnknown(codexSettingsAttributeTypes)},
		{Name: "rejects_unknown_object_before_apply", Settings: types.ObjectUnknown(codexSettingsAttributeTypes), RequireKnown: true, Expected: []string{"Unknown Codex Settings"}},
		{
			Name: "defers_unknown_child",
			Settings: mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
				Model: types.StringUnknown(), ReasoningEffort: types.StringNull(), ServiceTier: types.StringNull(),
			}),
		},
		{
			Name: "rejects_unknown_child_before_apply",
			Settings: mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
				Model: types.StringUnknown(), ReasoningEffort: types.StringNull(), ServiceTier: types.StringNull(),
			}),
			RequireKnown: true,
			Expected:     []string{"Unknown String Value"},
		},
		{
			Name: "rejects_invalid_model",
			Settings: mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
				Model: types.StringValue("future"), ReasoningEffort: types.StringNull(), ServiceTier: types.StringNull(),
			}),
			Expected: []string{"Unsupported Value"},
		},
		{
			Name: "rejects_deprecated_model",
			Settings: mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
				Model: types.StringValue("gpt-5.3-codex"), ReasoningEffort: types.StringNull(), ServiceTier: types.StringNull(),
			}),
			Expected: []string{"Unsupported Value"},
		},
		{
			Name: "rejects_invalid_reasoning_effort",
			Settings: mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
				Model: types.StringNull(), ReasoningEffort: types.StringValue("extreme"), ServiceTier: types.StringNull(),
			}),
			Expected: []string{"Unsupported Value"},
		},
		{
			Name: "rejects_feature_flagged_reasoning_effort",
			Settings: mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
				Model: types.StringNull(), ReasoningEffort: types.StringValue("ultra"), ServiceTier: types.StringNull(),
			}),
			Expected: []string{"Unsupported Value"},
		},
		{
			Name: "rejects_invalid_service_tier",
			Settings: mustObjectValue(t, codexSettingsAttributeTypes, CodexSettingsModel{
				Model: types.StringNull(), ReasoningEffort: types.StringNull(), ServiceTier: types.StringValue("slow"),
			}),
			Expected: []string{"Unsupported Value"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var diags diag.Diagnostics
			validateCodexSettings(t.Context(), tc.Settings, tc.RequireKnown, &diags)
			got := diagnosticSummaries(diags)
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateCodexSettings() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUnsupportedWorkflowReason(t *testing.T) {
	t.Parallel()

	type Expectation struct{ Reason string }
	tests := []struct {
		Name     string
		Workflow *v1.Workflow
		Expected Expectation
	}{
		{Name: "supported", Workflow: testRemoteWorkflow(), Expected: Expectation{}},
		{
			Name: "codex_agent",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.AgentId = codexAgentID
				return workflow
			}(),
			Expected: Expectation{},
		},
		{
			Name: "ona_agent",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.AgentId = "00000000-0000-0000-0000-000000007100"
				return workflow
			}(),
			Expected: Expectation{},
		},
		{
			Name: "report_action",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.Report = &v1.WorkflowAction{}
				return workflow
			}(),
			Expected: Expectation{Reason: "The automation configures a report action, which is not supported by ona_automation. Remove the report before importing it."},
		},
		{
			Name: "accepts_unknown_agent",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.AgentId = "00000000-0000-0000-0000-000000000099"
				return workflow
			}(),
			Expected: Expectation{},
		},
		{
			Name: "accepts_codex_settings",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.CodexSettings = &v1.CodexSettings{}
				return workflow
			}(),
			Expected: Expectation{},
		},
		{
			Name: "report_step",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.Action.Steps = append(workflow.Spec.Action.Steps, &v1.WorkflowStep{Step: &v1.WorkflowStep_Report_{Report: &v1.WorkflowStep_Report{}}})
				return workflow
			}(),
			Expected: Expectation{Reason: "The automation contains a report step, which is not supported by ona_automation. Remove the report step before importing it."},
		},
		{
			Name: "legacy_pull_request",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.Triggers[0].Trigger = &v1.WorkflowTrigger_PullRequest_{PullRequest: &v1.WorkflowTrigger_PullRequest{Events: []v1.WorkflowTrigger_PullRequestEvent{v1.WorkflowTrigger_PULL_REQUEST_EVENT_OPENED}}}
				return workflow
			}(),
			Expected: Expectation{Reason: "The automation contains a legacy pull-request trigger without a webhook or integration ID. The current create API cannot reproduce that trigger."},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			got := Expectation{Reason: unsupportedWorkflowReason(tc.Workflow)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("unsupportedWorkflowReason() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPopulateModelRoundTrip(t *testing.T) {
	t.Parallel()

	integrationID := "00000000-0000-0000-0000-000000000040"
	remote := &v1.Workflow{
		Id: testWorkflowID,
		Metadata: &v1.Workflow_Metadata{
			Name: "Core variants", Description: "All supported variants", Creator: &v1.Subject{Id: testServiceAccountID, Principal: v1.Principal_PRINCIPAL_USER},
			Executor:  &v1.Subject{Id: testServiceAccountID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT},
			CreatedAt: timestamppb.New(time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)), UpdatedAt: timestamppb.New(time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC)),
		},
		Spec: &v1.Workflow_Spec{
			AgentId: codexAgentID,
			CodexSettings: &v1.CodexSettings{
				Model: v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_5, ReasoningEffort: v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_HIGH, ServiceTier: v1.CodexServiceTier_CODEX_SERVICE_TIER_FAST,
			},
			Triggers: []*v1.WorkflowTrigger{
				{
					Trigger: &v1.WorkflowTrigger_Time_{Time: &v1.WorkflowTrigger_Time{CronExpression: "0 9 * * 1-5"}},
					Context: &v1.WorkflowTriggerContext{Context: &v1.WorkflowTriggerContext_Repositories_{Repositories: &v1.WorkflowTriggerContext_Repositories{
						RepositorySelector: &v1.WorkflowTriggerContext_Repositories_RepoSelector{RepoSelector: &v1.WorkflowTriggerContext_Repositories_RepositorySelector{RepoSearchString: "org:ona", ScmHost: "github.com"}},
						EnvironmentClassId: testEnvironmentClassID,
					}}},
				},
				{
					Trigger: &v1.WorkflowTrigger_Manual_{Manual: &v1.WorkflowTrigger_Manual{}},
					Context: &v1.WorkflowTriggerContext{Context: &v1.WorkflowTriggerContext_Repositories_{Repositories: &v1.WorkflowTriggerContext_Repositories{
						RepositorySelector: &v1.WorkflowTriggerContext_Repositories_RepositoryUrls{RepositoryUrls: &v1.WorkflowTriggerContext_Repositories_RepositoryURLs{RepoUrls: []string{"https://github.com/ona/repo"}}},
						EnvironmentClassId: testEnvironmentClassID,
					}}},
				},
				{
					Trigger: &v1.WorkflowTrigger_PullRequest_{PullRequest: &v1.WorkflowTrigger_PullRequest{Events: []v1.WorkflowTrigger_PullRequestEvent{v1.WorkflowTrigger_PULL_REQUEST_EVENT_OPENED}, IntegrationId: &integrationID}},
					Context: &v1.WorkflowTriggerContext{Context: &v1.WorkflowTriggerContext_FromTrigger_{FromTrigger: &v1.WorkflowTriggerContext_FromTrigger{}}},
				},
				{
					Trigger: &v1.WorkflowTrigger_Manual_{Manual: &v1.WorkflowTrigger_Manual{}},
					Context: &v1.WorkflowTriggerContext{Context: &v1.WorkflowTriggerContext_Agent_{Agent: &v1.WorkflowTriggerContext_Agent{Prompt: "Choose repositories"}}},
				},
			},
			Action: &v1.WorkflowAction{
				Limits: &v1.WorkflowAction_Limits{MaxParallel: 3, MaxTotal: 20, PerExecution: &v1.WorkflowAction_Limits_PerExecution{MaxTime: durationpb.New(time.Hour)}},
				Steps: []*v1.WorkflowStep{
					{Step: &v1.WorkflowStep_Agent_{Agent: &v1.WorkflowStep_Agent{Prompt: "Fix checks"}}},
					{Step: &v1.WorkflowStep_PullRequest_{PullRequest: &v1.WorkflowStep_PullRequest{Title: "Fix checks", Branch: "ona/fix", Draft: true}}},
				},
			},
		},
		WebhookUrl: "https://example.com/workflows/test/webhooks",
	}

	var model Model
	var diags diag.Diagnostics
	populateModel(t.Context(), &model, remote, &diags)
	request, requestDiags := createWorkflowRequest(t.Context(), model)
	diags.Append(requestDiags...)
	type Expectation struct {
		Request       *v1.CreateWorkflowRequest
		ObservedAgent string
		Errors        []string
	}
	expected := Expectation{Request: &v1.CreateWorkflowRequest{
		Name: "Core variants", Description: "All supported variants", Triggers: remote.GetSpec().GetTriggers(), Action: remote.GetSpec().GetAction(), Executor: remote.GetMetadata().GetExecutor(), AgentId: codexAgentID,
		CodexSettings: remote.GetSpec().GetCodexSettings(),
	}, ObservedAgent: agentCodex}
	got := Expectation{Request: request, ObservedAgent: model.Agent.ValueString(), Errors: diagnosticSummaries(diags)}
	if diff := cmp.Diff(expected, got, protocmp.Transform()); diff != "" {
		t.Errorf("populateModel()/createWorkflowRequest() mismatch (-want +got):\n%s", diff)
	}
}

func TestContextToObjectKeepsEmptyProjectIDsKnown(t *testing.T) {
	t.Parallel()

	remote := &v1.WorkflowTriggerContext{Context: &v1.WorkflowTriggerContext_Projects_{Projects: &v1.WorkflowTriggerContext_Projects{}}}
	var diags diag.Diagnostics
	value := contextToObject(t.Context(), remote, &diags)
	var contextModel ContextModel
	diags.Append(value.As(t.Context(), &contextModel, basetypes.ObjectAsOptions{})...)
	var projectsModel ProjectsContextModel
	if !contextModel.Projects.IsNull() && !contextModel.Projects.IsUnknown() {
		diags.Append(contextModel.Projects.As(t.Context(), &projectsModel, basetypes.ObjectAsOptions{})...)
	}

	got := struct {
		Null    bool
		Unknown bool
		Count   int
		Errors  []string
	}{
		Null:    projectsModel.ProjectIDs.IsNull(),
		Unknown: projectsModel.ProjectIDs.IsUnknown(),
		Count:   len(projectsModel.ProjectIDs.Elements()),
		Errors:  diagnosticSummaries(diags),
	}
	expected := struct {
		Null    bool
		Unknown bool
		Count   int
		Errors  []string
	}{}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("contextToObject() mismatch (-want +got):\n%s", diff)
	}
}

func TestCollectionFilter(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Filter *v1.ListWorkflowsRequest_Filter
		Errors []string
	}
	tests := []struct {
		Name     string
		Input    CollectionModel
		Expected Expectation
	}{
		{
			Name: "maps_filters",
			Input: CollectionModel{
				AutomationIDs: mustSetValue(t, types.StringType, []string{testWorkflowID}), Search: types.StringValue("checks"), CreatorIDs: mustSetValue(t, types.StringType, []string{testServiceAccountID}),
				StatusPhases: mustSetValue(t, types.StringType, []string{"running", "completed"}), HasFailedExecutionSince: types.StringNull(), Disabled: types.BoolValue(false),
			},
			Expected: Expectation{Filter: &v1.ListWorkflowsRequest_Filter{
				WorkflowIds: []string{testWorkflowID}, Search: "checks", CreatorIds: []string{testServiceAccountID},
				StatusPhases: []v1.WorkflowExecutionPhase{v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_COMPLETED, v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_RUNNING}, Disabled: boolPointer(false),
			}},
		},
		{
			Name: "maps_failed_since",
			Input: CollectionModel{
				AutomationIDs: types.SetNull(types.StringType), Search: types.StringNull(), CreatorIDs: types.SetNull(types.StringType), StatusPhases: types.SetNull(types.StringType),
				HasFailedExecutionSince: types.StringValue("2026-07-15T12:00:00Z"), Disabled: types.BoolNull(),
			},
			Expected: Expectation{Filter: &v1.ListWorkflowsRequest_Filter{HasFailedExecutionSince: timestamppb.New(time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC))}},
		},
		{
			Name: "rejects_incompatible_filters",
			Input: CollectionModel{
				AutomationIDs: types.SetNull(types.StringType), Search: types.StringNull(), CreatorIDs: types.SetNull(types.StringType), StatusPhases: mustSetValue(t, types.StringType, []string{"running"}),
				HasFailedExecutionSince: types.StringValue("2026-07-15T12:00:00Z"), Disabled: types.BoolNull(),
			},
			Expected: Expectation{Errors: []string{"Incompatible Automation Filters"}},
		},
		{
			Name: "rejects_invalid_phase",
			Input: CollectionModel{
				AutomationIDs: types.SetNull(types.StringType), Search: types.StringNull(), CreatorIDs: types.SetNull(types.StringType), StatusPhases: mustSetValue(t, types.StringType, []string{"failed"}),
				HasFailedExecutionSince: types.StringNull(), Disabled: types.BoolNull(),
			},
			Expected: Expectation{Errors: []string{"Invalid Automation Execution Phase"}},
		},
		{
			Name: "rejects_timestamp_outside_protobuf_range",
			Input: CollectionModel{
				AutomationIDs: types.SetNull(types.StringType), Search: types.StringNull(), CreatorIDs: types.SetNull(types.StringType), StatusPhases: types.SetNull(types.StringType),
				HasFailedExecutionSince: types.StringValue("0000-01-01T00:00:00Z"), Disabled: types.BoolNull(),
			},
			Expected: Expectation{Errors: []string{"Invalid Failed-Execution Timestamp"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var diags diag.Diagnostics
			filter := collectionFilter(tc.Input, &diags)
			got := Expectation{Errors: diagnosticSummaries(diags)}
			if !diags.HasError() {
				got.Filter = filter
			}
			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("collectionFilter() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWorkflowExecutionPhaseFromString(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Phase v1.WorkflowExecutionPhase
		OK    bool
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{Name: "running", Input: "running", Expected: Expectation{Phase: v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_RUNNING, OK: true}},
		{Name: "completed", Input: "completed", Expected: Expectation{Phase: v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_COMPLETED, OK: true}},
		{Name: "unsupported", Input: "failed", Expected: Expectation{Phase: v1.WorkflowExecutionPhase_WORKFLOW_EXECUTION_PHASE_UNSPECIFIED}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			phase, ok := workflowExecutionPhaseFromString(tc.Input)
			got := Expectation{Phase: phase, OK: ok}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("workflowExecutionPhaseFromString() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateModelUnknownElements(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Errors []string
	}
	tests := []struct {
		Name         string
		Mutate       func(*Model)
		RequireKnown bool
		Expected     Expectation
	}{
		{
			Name: "defers_unknown_project_id_during_config_validation",
			Mutate: func(model *Model) {
				setProjectIDs(t, model, types.SetValueMust(types.StringType, []attr.Value{types.StringUnknown()}))
			},
		},
		{
			Name: "rejects_unknown_project_id_before_apply",
			Mutate: func(model *Model) {
				setProjectIDs(t, model, types.SetValueMust(types.StringType, []attr.Value{types.StringUnknown()}))
			},
			RequireKnown: true,
			Expected:     Expectation{Errors: []string{"Unknown Set Element"}},
		},
		{
			Name: "defers_unknown_trigger_during_config_validation",
			Mutate: func(model *Model) {
				model.Triggers = types.ListValueMust(types.ObjectType{AttrTypes: triggerAttributeTypes}, []attr.Value{types.ObjectUnknown(triggerAttributeTypes)})
			},
		},
		{
			Name: "rejects_unknown_trigger_before_apply",
			Mutate: func(model *Model) {
				model.Triggers = types.ListValueMust(types.ObjectType{AttrTypes: triggerAttributeTypes}, []attr.Value{types.ObjectUnknown(triggerAttributeTypes)})
			},
			RequireKnown: true,
			Expected:     Expectation{Errors: []string{"Unknown Automation Trigger"}},
		},
		{
			Name: "defers_unknown_step_during_config_validation",
			Mutate: func(model *Model) {
				setActionSteps(t, model, types.ListValueMust(types.ObjectType{AttrTypes: stepAttributeTypes}, []attr.Value{types.ObjectUnknown(stepAttributeTypes)}))
			},
		},
		{
			Name: "rejects_unknown_step_before_apply",
			Mutate: func(model *Model) {
				setActionSteps(t, model, types.ListValueMust(types.ObjectType{AttrTypes: stepAttributeTypes}, []attr.Value{types.ObjectUnknown(stepAttributeTypes)}))
			},
			RequireKnown: true,
			Expected:     Expectation{Errors: []string{"Unknown Automation Step"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			model := testWorkflowModel(t)
			tc.Mutate(&model)
			var diags diag.Diagnostics
			validateModel(t.Context(), model, tc.RequireKnown, &diags)
			got := Expectation{Errors: diagnosticSummaries(diags)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateModel() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func testWorkflowModel(t *testing.T) Model {
	t.Helper()
	context := ContextModel{
		Projects:     mustObjectValue(t, projectsContextAttributeTypes, ProjectsContextModel{ProjectIDs: mustSetValue(t, types.StringType, []string{testProjectID})}),
		Repositories: types.ObjectNull(repositoriesContextAttributeTypes), Agent: types.ObjectNull(agentContextAttributeTypes), FromTrigger: types.ObjectNull(emptyAttributeTypes),
	}
	trigger := TriggerModel{
		Manual: types.ObjectValueMust(emptyAttributeTypes, map[string]attr.Value{}), Time: types.ObjectNull(timeTriggerAttributeTypes), PullRequest: types.ObjectNull(pullRequestTriggerAttributeTypes),
		Context: mustObjectValue(t, contextAttributeTypes, context),
	}
	step := StepModel{
		Task:  mustObjectValue(t, taskStepAttributeTypes, TaskStepModel{Command: types.StringValue("make test")}),
		Agent: types.ObjectNull(agentStepAttributeTypes), PullRequest: types.ObjectNull(pullRequestStepAttributeTypes),
	}
	action := ActionModel{
		Limits: mustObjectValue(t, limitsAttributeTypes, LimitsModel{MaxParallel: types.Int32Value(2), MaxTotal: types.Int32Value(10), MaxTime: types.StringNull()}),
		Steps:  mustListValue(t, types.ObjectType{AttrTypes: stepAttributeTypes}, []StepModel{step}),
	}
	return Model{
		ID: types.StringNull(), Agent: types.StringValue(agentCodex), Name: types.StringValue("Nightly checks"), Description: types.StringValue("Runs checks"),
		CodexSettings: types.ObjectNull(codexSettingsAttributeTypes),
		Triggers:      mustListValue(t, types.ObjectType{AttrTypes: triggerAttributeTypes}, []TriggerModel{trigger}), Action: mustObjectValue(t, actionAttributeTypes, action),
		Executor: mustObjectValue(t, subjectAttributeTypes, SubjectModel{ID: types.StringValue(testServiceAccountID), Principal: types.StringValue("service_account")}),
		Disabled: types.BoolValue(false), WebhookURL: types.StringUnknown(), Creator: types.ObjectUnknown(subjectAttributeTypes), CreatedAt: types.StringUnknown(), UpdatedAt: types.StringUnknown(),
	}
}

func setProjectIDs(t *testing.T, model *Model, projectIDs types.Set) {
	t.Helper()

	var triggers []TriggerModel
	diags := model.Triggers.ElementsAs(t.Context(), &triggers, false)
	if diags.HasError() {
		t.Fatalf("types.List.ElementsAs() diagnostics: %v", diags)
	}
	var contextModel ContextModel
	mustObjectAs(t, triggers[0].Context, &contextModel)
	contextModel.Projects = mustObjectValue(t, projectsContextAttributeTypes, ProjectsContextModel{ProjectIDs: projectIDs})
	triggers[0].Context = mustObjectValue(t, contextAttributeTypes, contextModel)
	model.Triggers = mustListValue(t, types.ObjectType{AttrTypes: triggerAttributeTypes}, triggers)
}

func setActionSteps(t *testing.T, model *Model, steps types.List) {
	t.Helper()

	var action ActionModel
	mustObjectAs(t, model.Action, &action)
	action.Steps = steps
	model.Action = mustObjectValue(t, actionAttributeTypes, action)
}

func testRemoteWorkflow() *v1.Workflow {
	return &v1.Workflow{
		Id:       testWorkflowID,
		Metadata: &v1.Workflow_Metadata{Name: "Nightly checks", Executor: &v1.Subject{Id: testServiceAccountID, Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}},
		Spec: &v1.Workflow_Spec{
			Triggers: []*v1.WorkflowTrigger{{Trigger: &v1.WorkflowTrigger_Manual_{Manual: &v1.WorkflowTrigger_Manual{}}, Context: &v1.WorkflowTriggerContext{Context: &v1.WorkflowTriggerContext_Projects_{Projects: &v1.WorkflowTriggerContext_Projects{ProjectIds: []string{testProjectID}}}}}},
			Action:   &v1.WorkflowAction{Limits: &v1.WorkflowAction_Limits{MaxParallel: 2, MaxTotal: 10}, Steps: []*v1.WorkflowStep{{Step: &v1.WorkflowStep_Task_{Task: &v1.WorkflowStep_Task{Command: "make test"}}}}},
		},
	}
}

func mustObjectValue(t *testing.T, attributeTypes map[string]attr.Type, model any) types.Object {
	t.Helper()
	value, diags := types.ObjectValueFrom(t.Context(), attributeTypes, model)
	if diags.HasError() {
		t.Fatalf("types.ObjectValueFrom() diagnostics: %v", diags)
	}
	return value
}

func mustObjectAs(t *testing.T, value types.Object, target any) {
	t.Helper()
	diags := value.As(t.Context(), target, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("types.Object.As() diagnostics: %v", diags)
	}
}

func mustListValue(t *testing.T, elementType attr.Type, model any) types.List {
	t.Helper()
	value, diags := types.ListValueFrom(t.Context(), elementType, model)
	if diags.HasError() {
		t.Fatalf("types.ListValueFrom() diagnostics: %v", diags)
	}
	return value
}

func mustSetValue(t *testing.T, elementType attr.Type, model any) types.Set {
	t.Helper()
	value, diags := types.SetValueFrom(t.Context(), elementType, model)
	if diags.HasError() {
		t.Fatalf("types.SetValueFrom() diagnostics: %v", diags)
	}
	return value
}

func diagnosticSummaries(diags diag.Diagnostics) []string {
	var result []string
	for _, diagnostic := range diags {
		if diagnostic.Severity() == diag.SeverityError {
			result = append(result, diagnostic.Summary())
		}
	}
	return result
}

func TestImportableWorkflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Input    *v1.Workflow
		Expected bool
	}{
		{Name: "nil_workflow", Expected: false},
		{Name: "missing_specification", Input: &v1.Workflow{}, Expected: false},
		{Name: "deleting", Input: &v1.Workflow{Spec: &v1.Workflow_Spec{Deleting: true, Action: &v1.WorkflowAction{}}}, Expected: false},
		{Name: "report_action", Input: &v1.Workflow{Spec: &v1.Workflow_Spec{Action: &v1.WorkflowAction{}, Report: &v1.WorkflowAction{}}}, Expected: false},
		{Name: "report_step", Input: &v1.Workflow{Spec: &v1.Workflow_Spec{Action: &v1.WorkflowAction{Steps: []*v1.WorkflowStep{{Step: &v1.WorkflowStep_Report_{Report: &v1.WorkflowStep_Report{}}}}}}}, Expected: false},
		{Name: "legacy_pull_request_trigger", Input: &v1.Workflow{Spec: &v1.Workflow_Spec{Action: &v1.WorkflowAction{}, Triggers: []*v1.WorkflowTrigger{{Trigger: &v1.WorkflowTrigger_PullRequest_{PullRequest: &v1.WorkflowTrigger_PullRequest{}}}}}}, Expected: false},
		{Name: "supported", Input: &v1.Workflow{Spec: &v1.Workflow_Spec{Action: &v1.WorkflowAction{}}}, Expected: true},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			if diff := cmp.Diff(tc.Expected, importableWorkflow(tc.Input)); diff != "" {
				t.Errorf("importableWorkflow() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type codexSettingsSnapshot struct {
	Null            bool
	Model           string
	ReasoningEffort string
	ServiceTier     string
}

func snapshotCodexSettings(t *testing.T, value types.Object) codexSettingsSnapshot {
	t.Helper()
	result := codexSettingsSnapshot{Null: value.IsNull()}
	if value.IsNull() {
		return result
	}
	var model CodexSettingsModel
	mustObjectAs(t, value, &model)
	if !model.Model.IsNull() {
		result.Model = model.Model.ValueString()
	}
	if !model.ReasoningEffort.IsNull() {
		result.ReasoningEffort = model.ReasoningEffort.ValueString()
	}
	if !model.ServiceTier.IsNull() {
		result.ServiceTier = model.ServiceTier.ValueString()
	}
	return result
}

func boolPointer(value bool) *bool { return &value }

const (
	testWorkflowID         = "00000000-0000-0000-0000-000000000001"
	testProjectID          = "00000000-0000-0000-0000-000000000010"
	testServiceAccountID   = "00000000-0000-0000-0000-000000000020"
	testEnvironmentClassID = "00000000-0000-0000-0000-000000000030"
)
