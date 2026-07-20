// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	"testing"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
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
			Expected: Expectation{Errors: []string{"Invalid Workflow Action Limits"}},
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
			Name: "report_action",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.Report = &v1.WorkflowAction{}
				return workflow
			}(),
			Expected: Expectation{Reason: "The workflow configures a report action, which is not supported by ona_automation. Remove the report before importing it."},
		},
		{
			Name: "agent_settings",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.AgentId = "00000000-0000-0000-0000-000000000099"
				return workflow
			}(),
			Expected: Expectation{Reason: "The workflow configures workflow-level agent or Codex settings, which are not supported by ona_automation. Remove those settings before importing it."},
		},
		{
			Name: "codex_settings",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.CodexSettings = &v1.CodexSettings{}
				return workflow
			}(),
			Expected: Expectation{Reason: "The workflow configures workflow-level agent or Codex settings, which are not supported by ona_automation. Remove those settings before importing it."},
		},
		{
			Name: "report_step",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.Action.Steps = append(workflow.Spec.Action.Steps, &v1.WorkflowStep{Step: &v1.WorkflowStep_Report_{Report: &v1.WorkflowStep_Report{}}})
				return workflow
			}(),
			Expected: Expectation{Reason: "The workflow contains a report step, which is not supported by ona_automation. Remove the report step before importing it."},
		},
		{
			Name: "legacy_pull_request",
			Workflow: func() *v1.Workflow {
				workflow := testRemoteWorkflow()
				workflow.Spec.Triggers[0].Trigger = &v1.WorkflowTrigger_PullRequest_{PullRequest: &v1.WorkflowTrigger_PullRequest{Events: []v1.WorkflowTrigger_PullRequestEvent{v1.WorkflowTrigger_PULL_REQUEST_EVENT_OPENED}}}
				return workflow
			}(),
			Expected: Expectation{Reason: "The workflow contains a legacy pull-request trigger without a webhook or integration ID. The current create API cannot reproduce that trigger."},
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
		Request *v1.CreateWorkflowRequest
		Errors  []string
	}
	expected := Expectation{Request: &v1.CreateWorkflowRequest{
		Name: "Core variants", Description: "All supported variants", Triggers: remote.GetSpec().GetTriggers(), Action: remote.GetSpec().GetAction(), Executor: remote.GetMetadata().GetExecutor(),
	}}
	got := Expectation{Request: request, Errors: diagnosticSummaries(diags)}
	if diff := cmp.Diff(expected, got, protocmp.Transform()); diff != "" {
		t.Errorf("populateModel()/createWorkflowRequest() mismatch (-want +got):\n%s", diff)
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
			Expected:     Expectation{Errors: []string{"Unknown Workflow Trigger"}},
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
			Expected:     Expectation{Errors: []string{"Unknown Workflow Step"}},
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
		ID: types.StringNull(), Name: types.StringValue("Nightly checks"), Description: types.StringValue("Runs checks"),
		Triggers: mustListValue(t, types.ObjectType{AttrTypes: triggerAttributeTypes}, []TriggerModel{trigger}), Action: mustObjectValue(t, actionAttributeTypes, action),
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

func boolPointer(value bool) *bool { return &value }

const (
	testWorkflowID         = "00000000-0000-0000-0000-000000000001"
	testProjectID          = "00000000-0000-0000-0000-000000000010"
	testServiceAccountID   = "00000000-0000-0000-0000-000000000020"
	testEnvironmentClassID = "00000000-0000-0000-0000-000000000030"
)
