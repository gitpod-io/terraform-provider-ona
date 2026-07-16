// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createWorkflowRequest(ctx context.Context, data Model) (*v1.CreateWorkflowRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateModel(ctx, data, true, &diags)
	if diags.HasError() {
		return nil, diags
	}
	triggers := triggersFromModel(ctx, data.Triggers, &diags)
	action := actionFromModel(ctx, data.Action, &diags)
	executor := subjectFromObject(ctx, data.Executor, &diags)
	if diags.HasError() {
		return nil, diags
	}
	return &v1.CreateWorkflowRequest{
		Name:        data.Name.ValueString(),
		Description: optionalString(data.Description),
		Triggers:    triggers,
		Action:      action,
		Executor:    executor,
	}, diags
}

func updateWorkflowRequest(ctx context.Context, data Model) (*v1.UpdateWorkflowRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateModel(ctx, data, true, &diags)
	if diags.HasError() {
		return nil, diags
	}
	name := data.Name.ValueString()
	description := optionalString(data.Description)
	disabled := data.Disabled.ValueBool()
	request := &v1.UpdateWorkflowRequest{
		WorkflowId:  data.ID.ValueString(),
		Name:        &name,
		Description: &description,
		Triggers:    triggersFromModel(ctx, data.Triggers, &diags),
		Action:      actionFromModel(ctx, data.Action, &diags),
		Executor:    subjectFromObject(ctx, data.Executor, &diags),
		Disabled:    &disabled,
	}
	if diags.HasError() {
		return nil, diags
	}
	return request, diags
}

func triggersFromModel(ctx context.Context, value types.List, diags *diag.Diagnostics) []*v1.WorkflowTrigger {
	var models []TriggerModel
	diags.Append(value.ElementsAs(ctx, &models, false)...)
	result := make([]*v1.WorkflowTrigger, 0, len(models))
	for _, model := range models {
		trigger := &v1.WorkflowTrigger{}
		switch {
		case !model.Manual.IsNull():
			trigger.Trigger = &v1.WorkflowTrigger_Manual_{Manual: &v1.WorkflowTrigger_Manual{}}
		case !model.Time.IsNull():
			var value TimeTriggerModel
			diags.Append(model.Time.As(ctx, &value, basetypes.ObjectAsOptions{})...)
			trigger.Trigger = &v1.WorkflowTrigger_Time_{Time: &v1.WorkflowTrigger_Time{CronExpression: value.CronExpression.ValueString()}}
		case !model.PullRequest.IsNull():
			var value PullRequestTriggerModel
			diags.Append(model.PullRequest.As(ctx, &value, basetypes.ObjectAsOptions{})...)
			var events []string
			diags.Append(value.Events.ElementsAs(ctx, &events, false)...)
			pullRequest := &v1.WorkflowTrigger_PullRequest{Events: make([]v1.WorkflowTrigger_PullRequestEvent, 0, len(events))}
			for _, event := range events {
				pullRequest.Events = append(pullRequest.Events, pullRequestEventFromString(event))
			}
			pullRequest.WebhookId = optionalStringPointer(value.WebhookID)
			pullRequest.IntegrationId = optionalStringPointer(value.IntegrationID)
			trigger.Trigger = &v1.WorkflowTrigger_PullRequest_{PullRequest: pullRequest}
		}
		trigger.Context = contextFromObject(ctx, model.Context, diags)
		result = append(result, trigger)
	}
	return result
}

func contextFromObject(ctx context.Context, value types.Object, diags *diag.Diagnostics) *v1.WorkflowTriggerContext {
	var model ContextModel
	diags.Append(value.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	result := &v1.WorkflowTriggerContext{}
	switch {
	case !model.Projects.IsNull():
		var projects ProjectsContextModel
		diags.Append(model.Projects.As(ctx, &projects, basetypes.ObjectAsOptions{})...)
		var ids []string
		diags.Append(projects.ProjectIDs.ElementsAs(ctx, &ids, false)...)
		result.Context = &v1.WorkflowTriggerContext_Projects_{Projects: &v1.WorkflowTriggerContext_Projects{ProjectIds: ids}}
	case !model.Repositories.IsNull():
		var repositories RepositoriesContextModel
		diags.Append(model.Repositories.As(ctx, &repositories, basetypes.ObjectAsOptions{})...)
		remote := &v1.WorkflowTriggerContext_Repositories{EnvironmentClassId: repositories.EnvironmentClassID.ValueString()}
		if !repositories.RepositoryURLs.IsNull() {
			var urls []string
			diags.Append(repositories.RepositoryURLs.ElementsAs(ctx, &urls, false)...)
			remote.RepositorySelector = &v1.WorkflowTriggerContext_Repositories_RepositoryUrls{RepositoryUrls: &v1.WorkflowTriggerContext_Repositories_RepositoryURLs{RepoUrls: urls}}
		} else {
			var selector RepositorySelectorModel
			diags.Append(repositories.RepositorySelector.As(ctx, &selector, basetypes.ObjectAsOptions{})...)
			remote.RepositorySelector = &v1.WorkflowTriggerContext_Repositories_RepoSelector{RepoSelector: &v1.WorkflowTriggerContext_Repositories_RepositorySelector{
				RepoSearchString: selector.RepositorySearchString.ValueString(),
				ScmHost:          selector.SCMHost.ValueString(),
			}}
		}
		result.Context = &v1.WorkflowTriggerContext_Repositories_{Repositories: remote}
	case !model.Agent.IsNull():
		var agent AgentContextModel
		diags.Append(model.Agent.As(ctx, &agent, basetypes.ObjectAsOptions{})...)
		result.Context = &v1.WorkflowTriggerContext_Agent_{Agent: &v1.WorkflowTriggerContext_Agent{Prompt: agent.Prompt.ValueString()}}
	case !model.FromTrigger.IsNull():
		result.Context = &v1.WorkflowTriggerContext_FromTrigger_{FromTrigger: &v1.WorkflowTriggerContext_FromTrigger{}}
	}
	return result
}

func actionFromModel(ctx context.Context, value types.Object, diags *diag.Diagnostics) *v1.WorkflowAction {
	var model ActionModel
	diags.Append(value.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	var limits LimitsModel
	diags.Append(model.Limits.As(ctx, &limits, basetypes.ObjectAsOptions{})...)
	remoteLimits := &v1.WorkflowAction_Limits{MaxParallel: limits.MaxParallel.ValueInt32(), MaxTotal: limits.MaxTotal.ValueInt32()}
	if !limits.MaxTime.IsNull() {
		duration, err := parseDuration(limits.MaxTime.ValueString())
		if err != nil {
			diags.AddError("Invalid Workflow Action Maximum Time", err.Error())
		} else {
			remoteLimits.PerExecution = &v1.WorkflowAction_Limits_PerExecution{MaxTime: durationpb.New(duration)}
		}
	}
	var steps []StepModel
	diags.Append(model.Steps.ElementsAs(ctx, &steps, false)...)
	remoteSteps := make([]*v1.WorkflowStep, 0, len(steps))
	for _, step := range steps {
		remote := &v1.WorkflowStep{}
		switch {
		case !step.Task.IsNull():
			var task TaskStepModel
			diags.Append(step.Task.As(ctx, &task, basetypes.ObjectAsOptions{})...)
			remote.Step = &v1.WorkflowStep_Task_{Task: &v1.WorkflowStep_Task{Command: task.Command.ValueString()}}
		case !step.Agent.IsNull():
			var agent AgentStepModel
			diags.Append(step.Agent.As(ctx, &agent, basetypes.ObjectAsOptions{})...)
			remote.Step = &v1.WorkflowStep_Agent_{Agent: &v1.WorkflowStep_Agent{Prompt: agent.Prompt.ValueString()}}
		case !step.PullRequest.IsNull():
			var pullRequest PullRequestStepModel
			diags.Append(step.PullRequest.As(ctx, &pullRequest, basetypes.ObjectAsOptions{})...)
			remote.Step = &v1.WorkflowStep_PullRequest_{PullRequest: &v1.WorkflowStep_PullRequest{
				Title: pullRequest.Title.ValueString(), Description: optionalString(pullRequest.Description), Branch: pullRequest.Branch.ValueString(), Draft: pullRequest.Draft.ValueBool(),
			}}
		}
		remoteSteps = append(remoteSteps, remote)
	}
	return &v1.WorkflowAction{Limits: remoteLimits, Steps: remoteSteps}
}

func populateModel(ctx context.Context, data *Model, workflow *v1.Workflow, diags *diag.Diagnostics) {
	if workflow == nil {
		diags.AddError("Unable to Read Ona Workflow", "The Ona API returned an empty workflow.")
		return
	}
	if reason := unsupportedWorkflowReason(workflow); reason != "" {
		diags.AddError("Unsupported Ona Workflow", reason)
		return
	}
	metadata, spec := workflow.GetMetadata(), workflow.GetSpec()
	if metadata == nil || spec == nil || spec.GetAction() == nil {
		diags.AddError("Unable to Read Ona Workflow", "The Ona API returned incomplete workflow metadata or configuration.")
		return
	}
	data.ID = stringValue(workflow.GetId())
	data.Name = stringValue(metadata.GetName())
	data.Description = optionalStringValue(metadata.GetDescription())
	data.Triggers = triggersToList(ctx, spec.GetTriggers(), diags)
	data.Action = actionToObject(ctx, spec.GetAction(), diags)
	data.Executor = subjectObject(metadata.GetExecutor(), diags)
	data.Disabled = types.BoolValue(spec.GetDisabled())
	data.WebhookURL = stringValue(workflow.GetWebhookUrl())
	data.Creator = subjectObject(metadata.GetCreator(), diags)
	data.CreatedAt = timestampValue(metadata.GetCreatedAt(), diags)
	data.UpdatedAt = timestampValue(metadata.GetUpdatedAt(), diags)
}

func triggersToList(ctx context.Context, remote []*v1.WorkflowTrigger, diags *diag.Diagnostics) types.List {
	models := make([]TriggerModel, 0, len(remote))
	for _, trigger := range remote {
		if trigger == nil {
			diags.AddError("Unable to Read Ona Workflow", "The Ona API returned an empty workflow trigger.")
			continue
		}
		model := TriggerModel{
			Manual: types.ObjectNull(emptyAttributeTypes), Time: types.ObjectNull(timeTriggerAttributeTypes), PullRequest: types.ObjectNull(pullRequestTriggerAttributeTypes),
		}
		switch value := trigger.GetTrigger().(type) {
		case *v1.WorkflowTrigger_Manual_:
			model.Manual = types.ObjectValueMust(emptyAttributeTypes, map[string]attr.Value{})
		case *v1.WorkflowTrigger_Time_:
			model.Time = objectValueFrom(ctx, timeTriggerAttributeTypes, TimeTriggerModel{CronExpression: stringValue(value.Time.GetCronExpression())}, diags)
		case *v1.WorkflowTrigger_PullRequest_:
			events := make([]string, 0, len(value.PullRequest.GetEvents()))
			for _, event := range value.PullRequest.GetEvents() {
				name, ok := pullRequestEventToString(event)
				if !ok {
					diags.AddError("Unable to Read Ona Workflow", fmt.Sprintf("The Ona API returned unsupported pull-request event %q.", event.String()))
					continue
				}
				events = append(events, name)
			}
			eventSet, eventDiags := types.SetValueFrom(ctx, types.StringType, events)
			diags.Append(eventDiags...)
			model.PullRequest = objectValueFrom(ctx, pullRequestTriggerAttributeTypes, PullRequestTriggerModel{
				Events: eventSet, WebhookID: optionalStringValue(value.PullRequest.GetWebhookId()), IntegrationID: optionalStringValue(value.PullRequest.GetIntegrationId()),
			}, diags)
		default:
			diags.AddError("Unable to Read Ona Workflow", "The Ona API returned an unsupported workflow trigger type.")
		}
		model.Context = contextToObject(ctx, trigger.GetContext(), diags)
		models = append(models, model)
	}
	value, valueDiags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: triggerAttributeTypes}, models)
	diags.Append(valueDiags...)
	return value
}

func contextToObject(ctx context.Context, remote *v1.WorkflowTriggerContext, diags *diag.Diagnostics) types.Object {
	model := ContextModel{
		Projects: types.ObjectNull(projectsContextAttributeTypes), Repositories: types.ObjectNull(repositoriesContextAttributeTypes), Agent: types.ObjectNull(agentContextAttributeTypes), FromTrigger: types.ObjectNull(emptyAttributeTypes),
	}
	if remote == nil {
		diags.AddError("Unable to Read Ona Workflow", "The Ona API returned an empty workflow trigger context.")
		return types.ObjectNull(contextAttributeTypes)
	}
	switch value := remote.GetContext().(type) {
	case *v1.WorkflowTriggerContext_Projects_:
		ids, idDiags := types.SetValueFrom(ctx, types.StringType, value.Projects.GetProjectIds())
		diags.Append(idDiags...)
		model.Projects = objectValueFrom(ctx, projectsContextAttributeTypes, ProjectsContextModel{ProjectIDs: ids}, diags)
	case *v1.WorkflowTriggerContext_Repositories_:
		repositories := RepositoriesContextModel{
			RepositoryURLs: types.SetNull(types.StringType), RepositorySelector: types.ObjectNull(repositorySelectorAttributeTypes), EnvironmentClassID: stringValue(value.Repositories.GetEnvironmentClassId()),
		}
		switch selector := value.Repositories.GetRepositorySelector().(type) {
		case *v1.WorkflowTriggerContext_Repositories_RepositoryUrls:
			urls, urlDiags := types.SetValueFrom(ctx, types.StringType, selector.RepositoryUrls.GetRepoUrls())
			diags.Append(urlDiags...)
			repositories.RepositoryURLs = urls
		case *v1.WorkflowTriggerContext_Repositories_RepoSelector:
			repositories.RepositorySelector = objectValueFrom(ctx, repositorySelectorAttributeTypes, RepositorySelectorModel{
				RepositorySearchString: stringValue(selector.RepoSelector.GetRepoSearchString()), SCMHost: stringValue(selector.RepoSelector.GetScmHost()),
			}, diags)
		default:
			diags.AddError("Unable to Read Ona Workflow", "The Ona API returned a repository context without a selector.")
		}
		model.Repositories = objectValueFrom(ctx, repositoriesContextAttributeTypes, repositories, diags)
	case *v1.WorkflowTriggerContext_Agent_:
		model.Agent = objectValueFrom(ctx, agentContextAttributeTypes, AgentContextModel{Prompt: stringValue(value.Agent.GetPrompt())}, diags)
	case *v1.WorkflowTriggerContext_FromTrigger_:
		model.FromTrigger = types.ObjectValueMust(emptyAttributeTypes, map[string]attr.Value{})
	default:
		diags.AddError("Unable to Read Ona Workflow", "The Ona API returned an unsupported trigger context type.")
	}
	return objectValueFrom(ctx, contextAttributeTypes, model, diags)
}

func actionToObject(ctx context.Context, remote *v1.WorkflowAction, diags *diag.Diagnostics) types.Object {
	if remote == nil || remote.GetLimits() == nil {
		diags.AddError("Unable to Read Ona Workflow", "The Ona API returned an action without limits.")
		return types.ObjectNull(actionAttributeTypes)
	}
	maxTime := types.StringNull()
	if remote.GetLimits().GetPerExecution() != nil && remote.GetLimits().GetPerExecution().GetMaxTime() != nil {
		if err := remote.GetLimits().GetPerExecution().GetMaxTime().CheckValid(); err != nil {
			diags.AddError("Unable to Read Ona Workflow", fmt.Sprintf("The Ona API returned an invalid maximum time: %v", err))
		} else {
			maxTime = types.StringValue(remote.GetLimits().GetPerExecution().GetMaxTime().AsDuration().String())
		}
	}
	limits := objectValueFrom(ctx, limitsAttributeTypes, LimitsModel{
		MaxParallel: types.Int32Value(remote.GetLimits().GetMaxParallel()), MaxTotal: types.Int32Value(remote.GetLimits().GetMaxTotal()), MaxTime: maxTime,
	}, diags)
	steps := make([]StepModel, 0, len(remote.GetSteps()))
	for _, remoteStep := range remote.GetSteps() {
		model := StepModel{Task: types.ObjectNull(taskStepAttributeTypes), Agent: types.ObjectNull(agentStepAttributeTypes), PullRequest: types.ObjectNull(pullRequestStepAttributeTypes)}
		if remoteStep == nil {
			diags.AddError("Unable to Read Ona Workflow", "The Ona API returned an empty workflow step.")
			continue
		}
		switch value := remoteStep.GetStep().(type) {
		case *v1.WorkflowStep_Task_:
			model.Task = objectValueFrom(ctx, taskStepAttributeTypes, TaskStepModel{Command: stringValue(value.Task.GetCommand())}, diags)
		case *v1.WorkflowStep_Agent_:
			model.Agent = objectValueFrom(ctx, agentStepAttributeTypes, AgentStepModel{Prompt: stringValue(value.Agent.GetPrompt())}, diags)
		case *v1.WorkflowStep_PullRequest_:
			model.PullRequest = objectValueFrom(ctx, pullRequestStepAttributeTypes, PullRequestStepModel{
				Title: stringValue(value.PullRequest.GetTitle()), Description: stringValue(value.PullRequest.GetDescription()), Branch: stringValue(value.PullRequest.GetBranch()), Draft: types.BoolValue(value.PullRequest.GetDraft()),
			}, diags)
		default:
			diags.AddError("Unable to Read Ona Workflow", "The Ona API returned an unsupported workflow step type.")
		}
		steps = append(steps, model)
	}
	stepList, stepDiags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: stepAttributeTypes}, steps)
	diags.Append(stepDiags...)
	return objectValueFrom(ctx, actionAttributeTypes, ActionModel{Limits: limits, Steps: stepList}, diags)
}

func unsupportedWorkflowReason(workflow *v1.Workflow) string {
	spec := workflow.GetSpec()
	if spec == nil {
		return "The workflow has no specification and cannot be managed by ona_automation."
	}
	if spec.GetReport() != nil {
		return "The workflow configures a report action, which is not supported by ona_automation. Remove the report before importing it."
	}
	if spec.GetAgentId() != "" || spec.GetCodexSettings() != nil {
		return "The workflow configures workflow-level agent or Codex settings, which are not supported by ona_automation. Remove those settings before importing it."
	}
	for _, step := range spec.GetAction().GetSteps() {
		if step.GetReport() != nil {
			return "The workflow contains a report step, which is not supported by ona_automation. Remove the report step before importing it."
		}
	}
	for _, trigger := range spec.GetTriggers() {
		if pullRequest := trigger.GetPullRequest(); pullRequest != nil && pullRequest.GetWebhookId() == "" && pullRequest.GetIntegrationId() == "" {
			return "The workflow contains a legacy pull-request trigger without a webhook or integration ID. The current create API cannot reproduce that trigger."
		}
	}
	return ""
}

func summaryFromWorkflow(workflow *v1.Workflow, diags *diag.Diagnostics) SummaryModel {
	if workflow == nil || workflow.GetMetadata() == nil || workflow.GetSpec() == nil {
		diags.AddError("Unable to List Ona Workflows", "The Ona API returned an incomplete workflow summary.")
		return SummaryModel{}
	}
	metadata, spec := workflow.GetMetadata(), workflow.GetSpec()
	return SummaryModel{
		ID: stringValue(workflow.GetId()), Name: stringValue(metadata.GetName()), Description: stringValue(metadata.GetDescription()),
		Disabled: types.BoolValue(spec.GetDisabled()), Deleting: types.BoolValue(spec.GetDeleting()), Executor: subjectObject(metadata.GetExecutor(), diags), Creator: subjectObject(metadata.GetCreator(), diags),
		CreatedAt: timestampValue(metadata.GetCreatedAt(), diags), UpdatedAt: timestampValue(metadata.GetUpdatedAt(), diags), WebhookURL: stringValue(workflow.GetWebhookUrl()),
	}
}

func subjectFromObject(ctx context.Context, value types.Object, diags *diag.Diagnostics) *v1.Subject {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var subject SubjectModel
	diags.Append(value.As(ctx, &subject, basetypes.ObjectAsOptions{})...)
	principal, ok := principalFromString(subject.Principal.ValueString())
	if !ok {
		diags.AddError("Invalid Workflow Executor", fmt.Sprintf("Unsupported principal %q.", subject.Principal.ValueString()))
		return nil
	}
	return &v1.Subject{Id: subject.ID.ValueString(), Principal: principal}
}

func subjectObject(subject *v1.Subject, diags *diag.Diagnostics) types.Object {
	if subject == nil {
		return types.ObjectNull(subjectAttributeTypes)
	}
	principal, ok := principalToString(subject.GetPrincipal())
	if !ok {
		diags.AddError("Unable to Read Ona Workflow", fmt.Sprintf("The Ona API returned unsupported subject principal %q.", subject.GetPrincipal().String()))
		return types.ObjectNull(subjectAttributeTypes)
	}
	value, objectDiags := types.ObjectValue(subjectAttributeTypes, map[string]attr.Value{
		"id": stringValue(subject.GetId()), "principal": types.StringValue(principal),
	})
	diags.Append(objectDiags...)
	return value
}

func objectValueFrom(ctx context.Context, attributeTypes map[string]attr.Type, model any, diags *diag.Diagnostics) types.Object {
	value, objectDiags := types.ObjectValueFrom(ctx, attributeTypes, model)
	diags.Append(objectDiags...)
	return value
}

func preservePlannedInputs(ctx context.Context, data *Model, planned Model, diags *diag.Diagnostics) {
	if !planned.Description.IsUnknown() {
		data.Description = planned.Description
	}
	preserveMaxTimeLexeme(ctx, data, planned, diags)
}

func preserveTerraformOnlyState(ctx context.Context, data *Model, prior Model, diags *diag.Diagnostics) {
	if data.Description.ValueString() == "" && !prior.Description.IsUnknown() && (prior.Description.IsNull() || prior.Description.ValueString() == "") {
		data.Description = prior.Description
	}
	preserveMaxTimeLexeme(ctx, data, prior, diags)
}

func preserveMaxTimeLexeme(ctx context.Context, data *Model, source Model, diags *diag.Diagnostics) {
	if data.Action.IsNull() || data.Action.IsUnknown() || source.Action.IsNull() || source.Action.IsUnknown() {
		return
	}
	var targetAction, sourceAction ActionModel
	diags.Append(data.Action.As(ctx, &targetAction, basetypes.ObjectAsOptions{})...)
	diags.Append(source.Action.As(ctx, &sourceAction, basetypes.ObjectAsOptions{})...)
	if diags.HasError() || targetAction.Limits.IsNull() || sourceAction.Limits.IsNull() {
		return
	}
	var targetLimits, sourceLimits LimitsModel
	diags.Append(targetAction.Limits.As(ctx, &targetLimits, basetypes.ObjectAsOptions{})...)
	diags.Append(sourceAction.Limits.As(ctx, &sourceLimits, basetypes.ObjectAsOptions{})...)
	if diags.HasError() || sourceLimits.MaxTime.IsNull() || sourceLimits.MaxTime.IsUnknown() || targetLimits.MaxTime.IsNull() || targetLimits.MaxTime.IsUnknown() {
		return
	}
	sourceDuration, sourceErr := parseDuration(sourceLimits.MaxTime.ValueString())
	targetDuration, targetErr := parseDuration(targetLimits.MaxTime.ValueString())
	if sourceErr != nil || targetErr != nil || sourceDuration != targetDuration {
		return
	}
	targetLimits.MaxTime = sourceLimits.MaxTime
	targetAction.Limits = objectValueFrom(ctx, limitsAttributeTypes, targetLimits, diags)
	data.Action = objectValueFrom(ctx, actionAttributeTypes, targetAction, diags)
}

func pullRequestEventFromString(value string) v1.WorkflowTrigger_PullRequestEvent {
	switch value {
	case "opened":
		return v1.WorkflowTrigger_PULL_REQUEST_EVENT_OPENED
	case "updated":
		return v1.WorkflowTrigger_PULL_REQUEST_EVENT_UPDATED
	case "approved":
		return v1.WorkflowTrigger_PULL_REQUEST_EVENT_APPROVED
	case "merged":
		return v1.WorkflowTrigger_PULL_REQUEST_EVENT_MERGED
	case "closed":
		return v1.WorkflowTrigger_PULL_REQUEST_EVENT_CLOSED
	case "ready_for_review":
		return v1.WorkflowTrigger_PULL_REQUEST_EVENT_READY_FOR_REVIEW
	case "review_requested":
		return v1.WorkflowTrigger_PULL_REQUEST_EVENT_REVIEW_REQUESTED
	default:
		return v1.WorkflowTrigger_PULL_REQUEST_EVENT_UNSPECIFIED
	}
}

func pullRequestEventToString(value v1.WorkflowTrigger_PullRequestEvent) (string, bool) {
	switch value {
	case v1.WorkflowTrigger_PULL_REQUEST_EVENT_OPENED:
		return "opened", true
	case v1.WorkflowTrigger_PULL_REQUEST_EVENT_UPDATED:
		return "updated", true
	case v1.WorkflowTrigger_PULL_REQUEST_EVENT_APPROVED:
		return "approved", true
	case v1.WorkflowTrigger_PULL_REQUEST_EVENT_MERGED:
		return "merged", true
	case v1.WorkflowTrigger_PULL_REQUEST_EVENT_CLOSED:
		return "closed", true
	case v1.WorkflowTrigger_PULL_REQUEST_EVENT_READY_FOR_REVIEW:
		return "ready_for_review", true
	case v1.WorkflowTrigger_PULL_REQUEST_EVENT_REVIEW_REQUESTED:
		return "review_requested", true
	default:
		return "", false
	}
}

func principalFromString(value string) (v1.Principal, bool) {
	switch value {
	case "user":
		return v1.Principal_PRINCIPAL_USER, true
	case "service_account":
		return v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, true
	default:
		return v1.Principal_PRINCIPAL_UNSPECIFIED, false
	}
}

func principalToString(value v1.Principal) (string, bool) {
	switch value {
	case v1.Principal_PRINCIPAL_USER:
		return "user", true
	case v1.Principal_PRINCIPAL_SERVICE_ACCOUNT:
		return "service_account", true
	case v1.Principal_PRINCIPAL_ACCOUNT:
		return "account", true
	case v1.Principal_PRINCIPAL_RUNNER:
		return "runner", true
	case v1.Principal_PRINCIPAL_ENVIRONMENT:
		return "environment", true
	case v1.Principal_PRINCIPAL_RUNNER_MANAGER:
		return "runner_manager", true
	default:
		return "", false
	}
}

func optionalString(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}

func optionalStringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return nil
	}
	result := value.ValueString()
	return &result
}

func stringValue(value string) types.String { return types.StringValue(value) }

func optionalStringValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func timestampValue(value *timestamppb.Timestamp, diags *diag.Diagnostics) types.String {
	if value == nil {
		return types.StringNull()
	}
	if err := value.CheckValid(); err != nil {
		diags.AddError("Unable to Read Ona Workflow", fmt.Sprintf("The Ona API returned an invalid timestamp: %v", err))
		return types.StringNull()
	}
	return types.StringValue(value.AsTime().UTC().Format(time.RFC3339Nano))
}

func parseDuration(value string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", value, err)
	}
	return duration, nil
}
