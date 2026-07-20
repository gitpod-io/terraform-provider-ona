// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Triggers    types.List   `tfsdk:"triggers"`
	Action      types.Object `tfsdk:"action"`
	Executor    types.Object `tfsdk:"executor"`
	Disabled    types.Bool   `tfsdk:"disabled"`
	WebhookURL  types.String `tfsdk:"webhook_url"`
	Creator     types.Object `tfsdk:"creator"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

type TriggerModel struct {
	Manual      types.Object `tfsdk:"manual"`
	Time        types.Object `tfsdk:"time"`
	PullRequest types.Object `tfsdk:"pull_request"`
	Context     types.Object `tfsdk:"context"`
}

type TimeTriggerModel struct {
	CronExpression types.String `tfsdk:"cron_expression"`
}

type PullRequestTriggerModel struct {
	Events        types.Set    `tfsdk:"events"`
	WebhookID     types.String `tfsdk:"webhook_id"`
	IntegrationID types.String `tfsdk:"integration_id"`
}

type ContextModel struct {
	Projects     types.Object `tfsdk:"projects"`
	Repositories types.Object `tfsdk:"repositories"`
	Agent        types.Object `tfsdk:"agent"`
	FromTrigger  types.Object `tfsdk:"from_trigger"`
}

type ProjectsContextModel struct {
	ProjectIDs types.Set `tfsdk:"project_ids"`
}

type RepositoriesContextModel struct {
	RepositoryURLs     types.Set    `tfsdk:"repository_urls"`
	RepositorySelector types.Object `tfsdk:"repository_selector"`
	EnvironmentClassID types.String `tfsdk:"environment_class_id"`
}

type RepositorySelectorModel struct {
	RepositorySearchString types.String `tfsdk:"repo_search_string"`
	SCMHost                types.String `tfsdk:"scm_host"`
}

type AgentContextModel struct {
	Prompt types.String `tfsdk:"prompt"`
}

type ActionModel struct {
	Limits types.Object `tfsdk:"limits"`
	Steps  types.List   `tfsdk:"steps"`
}

type LimitsModel struct {
	MaxParallel types.Int32  `tfsdk:"max_parallel"`
	MaxTotal    types.Int32  `tfsdk:"max_total"`
	MaxTime     types.String `tfsdk:"max_time"`
}

type StepModel struct {
	Task        types.Object `tfsdk:"task"`
	Agent       types.Object `tfsdk:"agent"`
	PullRequest types.Object `tfsdk:"pull_request"`
}

type TaskStepModel struct {
	Command types.String `tfsdk:"command"`
}

type AgentStepModel struct {
	Prompt types.String `tfsdk:"prompt"`
}

type PullRequestStepModel struct {
	Title       types.String `tfsdk:"title"`
	Description types.String `tfsdk:"description"`
	Branch      types.String `tfsdk:"branch"`
	Draft       types.Bool   `tfsdk:"draft"`
}

type SubjectModel struct {
	ID        types.String `tfsdk:"id"`
	Principal types.String `tfsdk:"principal"`
}

type CollectionModel struct {
	ID                      types.String   `tfsdk:"id"`
	AutomationIDs           types.Set      `tfsdk:"automation_ids"`
	Search                  types.String   `tfsdk:"search"`
	CreatorIDs              types.Set      `tfsdk:"creator_ids"`
	StatusPhases            types.Set      `tfsdk:"status_phases"`
	HasFailedExecutionSince types.String   `tfsdk:"has_failed_execution_since"`
	Disabled                types.Bool     `tfsdk:"disabled"`
	Automations             []SummaryModel `tfsdk:"automations"`
}

type SummaryModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Disabled    types.Bool   `tfsdk:"disabled"`
	Deleting    types.Bool   `tfsdk:"deleting"`
	Executor    types.Object `tfsdk:"executor"`
	Creator     types.Object `tfsdk:"creator"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	WebhookURL  types.String `tfsdk:"webhook_url"`
}

var emptyAttributeTypes = map[string]attr.Type{}

var timeTriggerAttributeTypes = map[string]attr.Type{
	"cron_expression": types.StringType,
}

var pullRequestTriggerAttributeTypes = map[string]attr.Type{
	"events":         types.SetType{ElemType: types.StringType},
	"webhook_id":     types.StringType,
	"integration_id": types.StringType,
}

var projectsContextAttributeTypes = map[string]attr.Type{
	"project_ids": types.SetType{ElemType: types.StringType},
}

var repositorySelectorAttributeTypes = map[string]attr.Type{
	"repo_search_string": types.StringType,
	"scm_host":           types.StringType,
}

var repositoriesContextAttributeTypes = map[string]attr.Type{
	"repository_urls":      types.SetType{ElemType: types.StringType},
	"repository_selector":  types.ObjectType{AttrTypes: repositorySelectorAttributeTypes},
	"environment_class_id": types.StringType,
}

var agentContextAttributeTypes = map[string]attr.Type{
	"prompt": types.StringType,
}

var contextAttributeTypes = map[string]attr.Type{
	"projects":     types.ObjectType{AttrTypes: projectsContextAttributeTypes},
	"repositories": types.ObjectType{AttrTypes: repositoriesContextAttributeTypes},
	"agent":        types.ObjectType{AttrTypes: agentContextAttributeTypes},
	"from_trigger": types.ObjectType{AttrTypes: emptyAttributeTypes},
}

var triggerAttributeTypes = map[string]attr.Type{
	"manual":       types.ObjectType{AttrTypes: emptyAttributeTypes},
	"time":         types.ObjectType{AttrTypes: timeTriggerAttributeTypes},
	"pull_request": types.ObjectType{AttrTypes: pullRequestTriggerAttributeTypes},
	"context":      types.ObjectType{AttrTypes: contextAttributeTypes},
}

var limitsAttributeTypes = map[string]attr.Type{
	"max_parallel": types.Int32Type,
	"max_total":    types.Int32Type,
	"max_time":     types.StringType,
}

var taskStepAttributeTypes = map[string]attr.Type{
	"command": types.StringType,
}

var agentStepAttributeTypes = map[string]attr.Type{
	"prompt": types.StringType,
}

var pullRequestStepAttributeTypes = map[string]attr.Type{
	"title":       types.StringType,
	"description": types.StringType,
	"branch":      types.StringType,
	"draft":       types.BoolType,
}

var stepAttributeTypes = map[string]attr.Type{
	"task":         types.ObjectType{AttrTypes: taskStepAttributeTypes},
	"agent":        types.ObjectType{AttrTypes: agentStepAttributeTypes},
	"pull_request": types.ObjectType{AttrTypes: pullRequestStepAttributeTypes},
}

var actionAttributeTypes = map[string]attr.Type{
	"limits": types.ObjectType{AttrTypes: limitsAttributeTypes},
	"steps":  types.ListType{ElemType: types.ObjectType{AttrTypes: stepAttributeTypes}},
}

var subjectAttributeTypes = map[string]attr.Type{
	"id":        types.StringType,
	"principal": types.StringType,
}
