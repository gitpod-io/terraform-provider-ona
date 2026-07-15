// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/robfig/cron/v3"
	"google.golang.org/protobuf/types/known/durationpb"
)

var workflowCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

func validateModel(ctx context.Context, data Model, requireKnown bool, diags *diag.Diagnostics) {
	validateString(data.Name, path.Root("name"), 1, 80, true, requireKnown, diags)
	validateString(data.Description, path.Root("description"), 0, 500, false, requireKnown, diags)
	validateExecutor(ctx, data.Executor, requireKnown, diags)

	if data.Triggers.IsUnknown() {
		unknownRequired(path.Root("triggers"), "Workflow Triggers", requireKnown, diags)
	} else if data.Triggers.IsNull() {
		diags.AddAttributeError(path.Root("triggers"), "Missing Workflow Triggers", "Configure between 1 and 10 workflow triggers.")
	} else {
		elements := data.Triggers.Elements()
		if len(elements) < 1 || len(elements) > 10 {
			diags.AddAttributeError(path.Root("triggers"), "Invalid Workflow Trigger Count", "Configure between 1 and 10 workflow triggers.")
		}
		for i, element := range elements {
			object, ok := element.(types.Object)
			if !ok {
				diags.AddAttributeError(path.Root("triggers").AtListIndex(i), "Invalid Workflow Trigger", "Workflow triggers must be objects.")
				continue
			}
			if object.IsUnknown() {
				unknownRequired(path.Root("triggers").AtListIndex(i), "Workflow Trigger", requireKnown, diags)
				continue
			}
			var trigger TriggerModel
			diags.Append(object.As(ctx, &trigger, basetypes.ObjectAsOptions{})...)
			if !diags.HasError() {
				validateTrigger(ctx, trigger, path.Root("triggers").AtListIndex(i), requireKnown, diags)
			}
		}
	}

	if data.Action.IsUnknown() {
		unknownRequired(path.Root("action"), "Workflow Action", requireKnown, diags)
	} else if data.Action.IsNull() {
		diags.AddAttributeError(path.Root("action"), "Missing Workflow Action", "Configure a workflow action.")
	} else {
		var action ActionModel
		diags.Append(data.Action.As(ctx, &action, basetypes.ObjectAsOptions{})...)
		if !diags.HasError() {
			validateAction(ctx, action, path.Root("action"), requireKnown, diags)
		}
	}
}

func validateExecutor(ctx context.Context, value types.Object, requireKnown bool, diags *diag.Diagnostics) {
	p := path.Root("executor")
	if value.IsNull() {
		return
	}
	if value.IsUnknown() {
		return
	}
	var executor SubjectModel
	diags.Append(value.As(ctx, &executor, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}
	validateUUIDString(executor.ID, p.AtName("id"), true, requireKnown, diags)
	validateEnumString(executor.Principal, p.AtName("principal"), []string{"user", "service_account"}, true, requireKnown, diags)
}

func validateTrigger(ctx context.Context, trigger TriggerModel, p path.Path, requireKnown bool, diags *diag.Diagnostics) {
	triggerKind, ok := exactlyOneObject(p, "Workflow Trigger Type", map[string]types.Object{
		"manual": trigger.Manual, "time": trigger.Time, "pull_request": trigger.PullRequest,
	}, requireKnown, diags)
	if !ok {
		return
	}

	switch triggerKind {
	case "time":
		var value TimeTriggerModel
		diags.Append(trigger.Time.As(ctx, &value, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return
		}
		validateString(value.CronExpression, p.AtName("time").AtName("cron_expression"), 1, 100, true, requireKnown, diags)
		if !value.CronExpression.IsNull() && !value.CronExpression.IsUnknown() {
			if _, err := workflowCronParser.Parse(value.CronExpression.ValueString()); err != nil {
				diags.AddAttributeError(p.AtName("time").AtName("cron_expression"), "Invalid Workflow Cron Expression", err.Error())
			}
		}
	case "pull_request":
		validatePullRequestTrigger(ctx, trigger.PullRequest, p.AtName("pull_request"), requireKnown, diags)
	}

	if trigger.Context.IsUnknown() {
		unknownRequired(p.AtName("context"), "Workflow Trigger Context", requireKnown, diags)
		return
	}
	if trigger.Context.IsNull() {
		diags.AddAttributeError(p.AtName("context"), "Missing Workflow Trigger Context", "Configure exactly one trigger context.")
		return
	}
	var contextModel ContextModel
	diags.Append(trigger.Context.As(ctx, &contextModel, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}
	contextKind, ok := exactlyOneObject(p.AtName("context"), "Workflow Trigger Context", map[string]types.Object{
		"projects": contextModel.Projects, "repositories": contextModel.Repositories, "agent": contextModel.Agent, "from_trigger": contextModel.FromTrigger,
	}, requireKnown, diags)
	if !ok {
		return
	}
	if contextKind == "from_trigger" && triggerKind != "pull_request" {
		diags.AddAttributeError(p.AtName("context").AtName("from_trigger"), "Invalid From-Trigger Context", "from_trigger is supported only with a pull_request trigger.")
	}
	switch contextKind {
	case "projects":
		var value ProjectsContextModel
		diags.Append(contextModel.Projects.As(ctx, &value, basetypes.ObjectAsOptions{})...)
		if !diags.HasError() {
			validateUUIDSet(value.ProjectIDs, p.AtName("context").AtName("projects").AtName("project_ids"), 0, 500, requireKnown, diags)
		}
	case "repositories":
		validateRepositoriesContext(ctx, contextModel.Repositories, p.AtName("context").AtName("repositories"), requireKnown, diags)
	case "agent":
		var value AgentContextModel
		diags.Append(contextModel.Agent.As(ctx, &value, basetypes.ObjectAsOptions{})...)
		if !diags.HasError() {
			validateString(value.Prompt, p.AtName("context").AtName("agent").AtName("prompt"), 1, 20000, true, requireKnown, diags)
		}
	}
}

func validatePullRequestTrigger(ctx context.Context, value types.Object, p path.Path, requireKnown bool, diags *diag.Diagnostics) {
	var trigger PullRequestTriggerModel
	diags.Append(value.As(ctx, &trigger, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}
	events := validateStringSet(trigger.Events, p.AtName("events"), 1, -1, requireKnown, diags)
	allowed := map[string]struct{}{
		"opened": {}, "updated": {}, "approved": {}, "merged": {}, "closed": {}, "ready_for_review": {}, "review_requested": {},
	}
	for _, event := range events {
		if _, ok := allowed[event]; !ok {
			diags.AddAttributeError(p.AtName("events"), "Invalid Pull-Request Event", fmt.Sprintf("Unsupported event %q.", event))
		}
	}
	validateUUIDString(trigger.WebhookID, p.AtName("webhook_id"), false, requireKnown, diags)
	validateUUIDString(trigger.IntegrationID, p.AtName("integration_id"), false, requireKnown, diags)
	if trigger.WebhookID.IsUnknown() || trigger.IntegrationID.IsUnknown() {
		unknownRequired(p, "Pull-Request Event Source", requireKnown, diags)
		return
	}
	hasWebhook := !trigger.WebhookID.IsNull() && trigger.WebhookID.ValueString() != ""
	hasIntegration := !trigger.IntegrationID.IsNull() && trigger.IntegrationID.ValueString() != ""
	if !hasWebhook && !hasIntegration {
		diags.AddAttributeError(p, "Missing Pull-Request Event Source", "Set webhook_id, integration_id, or both.")
	}
}

func validateRepositoriesContext(ctx context.Context, value types.Object, p path.Path, requireKnown bool, diags *diag.Diagnostics) {
	var repositories RepositoriesContextModel
	diags.Append(value.As(ctx, &repositories, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}
	validateUUIDString(repositories.EnvironmentClassID, p.AtName("environment_class_id"), true, requireKnown, diags)
	hasURLs := !repositories.RepositoryURLs.IsNull() && !repositories.RepositoryURLs.IsUnknown()
	hasSelector := !repositories.RepositorySelector.IsNull() && !repositories.RepositorySelector.IsUnknown()
	unknown := repositories.RepositoryURLs.IsUnknown() || repositories.RepositorySelector.IsUnknown()
	if hasURLs == hasSelector {
		if unknown && !requireKnown {
			return
		}
		diags.AddAttributeError(p, "Invalid Repository Selector", "Configure exactly one of repository_urls or repository_selector.")
		return
	}
	if hasURLs {
		values := validateStringSet(repositories.RepositoryURLs, p.AtName("repository_urls"), 1, 500, requireKnown, diags)
		for _, value := range values {
			if value == "" {
				diags.AddAttributeError(p.AtName("repository_urls"), "Invalid Repository URL", "Repository URLs must not be empty.")
			}
		}
		return
	}
	var selector RepositorySelectorModel
	diags.Append(repositories.RepositorySelector.As(ctx, &selector, basetypes.ObjectAsOptions{})...)
	if !diags.HasError() {
		validateString(selector.RepositorySearchString, p.AtName("repository_selector").AtName("repo_search_string"), 1, -1, true, requireKnown, diags)
		validateString(selector.SCMHost, p.AtName("repository_selector").AtName("scm_host"), 1, -1, true, requireKnown, diags)
	}
}

func validateAction(ctx context.Context, action ActionModel, p path.Path, requireKnown bool, diags *diag.Diagnostics) {
	if action.Limits.IsUnknown() {
		unknownRequired(p.AtName("limits"), "Workflow Action Limits", requireKnown, diags)
	} else if action.Limits.IsNull() {
		diags.AddAttributeError(p.AtName("limits"), "Missing Workflow Action Limits", "Configure workflow action limits.")
	} else {
		var limits LimitsModel
		diags.Append(action.Limits.As(ctx, &limits, basetypes.ObjectAsOptions{})...)
		if !diags.HasError() {
			validateInt32(limits.MaxParallel, p.AtName("limits").AtName("max_parallel"), 1, 25, requireKnown, diags)
			validateInt32(limits.MaxTotal, p.AtName("limits").AtName("max_total"), 1, 100, requireKnown, diags)
			if !limits.MaxParallel.IsNull() && !limits.MaxParallel.IsUnknown() && !limits.MaxTotal.IsNull() && !limits.MaxTotal.IsUnknown() && limits.MaxParallel.ValueInt32() > limits.MaxTotal.ValueInt32() {
				diags.AddAttributeError(p.AtName("limits").AtName("max_parallel"), "Invalid Workflow Action Limits", "max_parallel must not exceed max_total.")
			}
			if !limits.MaxTime.IsNull() && !limits.MaxTime.IsUnknown() {
				duration, err := parseDuration(limits.MaxTime.ValueString())
				if err != nil {
					diags.AddAttributeError(p.AtName("limits").AtName("max_time"), "Invalid Workflow Action Maximum Time", err.Error())
				} else if err := durationpb.New(duration).CheckValid(); err != nil {
					diags.AddAttributeError(p.AtName("limits").AtName("max_time"), "Invalid Workflow Action Maximum Time", err.Error())
				}
			} else if limits.MaxTime.IsUnknown() {
				unknownRequired(p.AtName("limits").AtName("max_time"), "Workflow Action Maximum Time", requireKnown, diags)
			}
		}
	}

	if action.Steps.IsUnknown() {
		unknownRequired(p.AtName("steps"), "Workflow Action Steps", requireKnown, diags)
		return
	}
	if action.Steps.IsNull() {
		diags.AddAttributeError(p.AtName("steps"), "Missing Workflow Action Steps", "Configure between 1 and 50 action steps.")
		return
	}
	elements := action.Steps.Elements()
	if len(elements) < 1 || len(elements) > 50 {
		diags.AddAttributeError(p.AtName("steps"), "Invalid Workflow Step Count", "Configure between 1 and 50 action steps.")
	}
	for i, element := range elements {
		object, ok := element.(types.Object)
		if !ok {
			diags.AddAttributeError(p.AtName("steps").AtListIndex(i), "Invalid Workflow Step", "Workflow steps must be objects.")
			continue
		}
		if object.IsUnknown() {
			unknownRequired(p.AtName("steps").AtListIndex(i), "Workflow Step", requireKnown, diags)
			continue
		}
		var step StepModel
		diags.Append(object.As(ctx, &step, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			continue
		}
		validateStep(ctx, step, p.AtName("steps").AtListIndex(i), requireKnown, diags)
	}
}

func validateStep(ctx context.Context, step StepModel, p path.Path, requireKnown bool, diags *diag.Diagnostics) {
	kind, ok := exactlyOneObject(p, "Workflow Step Type", map[string]types.Object{
		"task": step.Task, "agent": step.Agent, "pull_request": step.PullRequest,
	}, requireKnown, diags)
	if !ok {
		return
	}
	switch kind {
	case "task":
		var value TaskStepModel
		diags.Append(step.Task.As(ctx, &value, basetypes.ObjectAsOptions{})...)
		if !diags.HasError() {
			validateString(value.Command, p.AtName("task").AtName("command"), 1, 20000, true, requireKnown, diags)
		}
	case "agent":
		var value AgentStepModel
		diags.Append(step.Agent.As(ctx, &value, basetypes.ObjectAsOptions{})...)
		if !diags.HasError() {
			validateString(value.Prompt, p.AtName("agent").AtName("prompt"), 1, 20000, true, requireKnown, diags)
		}
	case "pull_request":
		var value PullRequestStepModel
		diags.Append(step.PullRequest.As(ctx, &value, basetypes.ObjectAsOptions{})...)
		if !diags.HasError() {
			validateString(value.Title, p.AtName("pull_request").AtName("title"), 1, 500, true, requireKnown, diags)
			validateString(value.Description, p.AtName("pull_request").AtName("description"), 0, 20000, false, requireKnown, diags)
			validateString(value.Branch, p.AtName("pull_request").AtName("branch"), 1, 255, true, requireKnown, diags)
			if value.Draft.IsUnknown() {
				unknownRequired(p.AtName("pull_request").AtName("draft"), "Pull-Request Draft Setting", requireKnown, diags)
			}
		}
	}
}

func exactlyOneObject(p path.Path, label string, values map[string]types.Object, requireKnown bool, diags *diag.Diagnostics) (string, bool) {
	var selected string
	known, unknown := 0, 0
	for name, value := range values {
		if value.IsUnknown() {
			unknown++
			continue
		}
		if !value.IsNull() {
			known++
			selected = name
		}
	}
	if known == 1 && unknown == 0 {
		return selected, true
	}
	if unknown > 0 && !requireKnown && known <= 1 {
		return "", false
	}
	if unknown > 0 && requireKnown {
		diags.AddAttributeError(p, "Unknown "+label, "All values must be known before apply.")
		return "", false
	}
	diags.AddAttributeError(p, "Invalid "+label, "Configure exactly one supported variant.")
	return "", false
}

func validateString(value types.String, p path.Path, minLength, maxLength int, required, requireKnown bool, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		unknownRequired(p, "String Value", requireKnown, diags)
		return
	}
	if value.IsNull() {
		if required {
			diags.AddAttributeError(p, "Missing Required Value", "Set this required value.")
		}
		return
	}
	length := utf8.RuneCountInString(value.ValueString())
	if length < minLength || maxLength >= 0 && length > maxLength {
		rangeText := fmt.Sprintf("at least %d characters", minLength)
		if maxLength >= 0 {
			rangeText = fmt.Sprintf("between %d and %d characters", minLength, maxLength)
		}
		diags.AddAttributeError(p, "Invalid String Length", "Value must be "+rangeText+".")
	}
}

func validateInt32(value types.Int32, p path.Path, minValue, maxValue int32, requireKnown bool, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		unknownRequired(p, "Integer Value", requireKnown, diags)
		return
	}
	if value.IsNull() {
		diags.AddAttributeError(p, "Missing Required Value", "Set this required integer value.")
		return
	}
	if value.ValueInt32() < minValue || value.ValueInt32() > maxValue {
		diags.AddAttributeError(p, "Integer Out of Range", fmt.Sprintf("Value must be between %d and %d.", minValue, maxValue))
	}
}

func validateUUIDString(value types.String, p path.Path, required, requireKnown bool, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		unknownRequired(p, "UUID", requireKnown, diags)
		return
	}
	if value.IsNull() {
		if required {
			diags.AddAttributeError(p, "Missing UUID", "Set a valid UUID.")
		}
		return
	}
	if value.ValueString() == "" {
		diags.AddAttributeError(p, "Invalid UUID", "Configured UUID values must not be empty.")
		return
	}
	if _, err := uuid.Parse(value.ValueString()); err != nil {
		diags.AddAttributeError(p, "Invalid UUID", err.Error())
	}
}

func validateUUIDSet(value types.Set, p path.Path, minItems, maxItems int, requireKnown bool, diags *diag.Diagnostics) []string {
	values := validateStringSet(value, p, minItems, maxItems, requireKnown, diags)
	for _, item := range values {
		if _, err := uuid.Parse(item); err != nil {
			diags.AddAttributeError(p, "Invalid UUID", fmt.Sprintf("Value %q is not a valid UUID: %v", item, err))
		}
	}
	return values
}

func validateStringSet(value types.Set, p path.Path, minItems, maxItems int, requireKnown bool, diags *diag.Diagnostics) []string {
	if value.IsUnknown() {
		unknownRequired(p, "Set Value", requireKnown, diags)
		return nil
	}
	if value.IsNull() {
		if minItems > 0 {
			diags.AddAttributeError(p, "Missing Required Values", fmt.Sprintf("Configure at least %d value(s).", minItems))
		}
		return nil
	}
	elements := value.Elements()
	if len(elements) < minItems || maxItems >= 0 && len(elements) > maxItems {
		message := fmt.Sprintf("Configure at least %d value(s).", minItems)
		if maxItems >= 0 {
			message = fmt.Sprintf("Configure between %d and %d values.", minItems, maxItems)
		}
		diags.AddAttributeError(p, "Invalid Value Count", message)
	}
	values := make([]string, 0, len(elements))
	for _, element := range elements {
		stringValue, ok := element.(types.String)
		if !ok {
			diags.AddAttributeError(p, "Invalid Set Value", "This set must contain only string values.")
			continue
		}
		if stringValue.IsUnknown() {
			unknownRequired(p, "Set Element", requireKnown, diags)
			continue
		}
		if stringValue.IsNull() {
			diags.AddAttributeError(p, "Invalid Set Value", "Set values must not be null.")
			continue
		}
		values = append(values, stringValue.ValueString())
	}
	return values
}

func validateEnumString(value types.String, p path.Path, allowed []string, required, requireKnown bool, diags *diag.Diagnostics) {
	validateString(value, p, 1, -1, required, requireKnown, diags)
	if value.IsNull() || value.IsUnknown() {
		return
	}
	for _, candidate := range allowed {
		if value.ValueString() == candidate {
			return
		}
	}
	diags.AddAttributeError(p, "Unsupported Value", fmt.Sprintf("Supported values are %v.", allowed))
}

func unknownRequired(p path.Path, label string, requireKnown bool, diags *diag.Diagnostics) {
	if requireKnown {
		diags.AddAttributeError(p, "Unknown "+label, "This value must be known before apply.")
	}
}
