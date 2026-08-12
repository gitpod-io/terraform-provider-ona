// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"errors"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAutomationQuery(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)

	codex := testAPIWorkflow(workflowAccID1, "Nightly Checks")
	codex.Spec.AgentId = workflowCodexAgentID
	codex.Spec.Disabled = true
	ona := testAPIWorkflow(workflowAccID2, "Nightly Checks")
	ona.Spec.AgentId = workflowOnaAgentID
	ona.Spec.Triggers[0].GetContext().GetProjects().ProjectIds = nil
	report := testAPIWorkflow(workflowAccID3, "Weekly Report")
	report.Spec.Report = &v1.WorkflowAction{}
	legacy := testAPIWorkflow(automationQueryLegacyID, "Legacy Pull Request")
	legacy.Spec.Triggers = []*v1.WorkflowTrigger{{
		Trigger: &v1.WorkflowTrigger_PullRequest_{PullRequest: &v1.WorkflowTrigger_PullRequest{}},
		Context: &v1.WorkflowTriggerContext{Context: &v1.WorkflowTriggerContext_FromTrigger_{FromTrigger: &v1.WorkflowTriggerContext_FromTrigger{}}},
	}}
	deleting := testAPIWorkflow(automationQueryDeletingID, "Deleting")
	deleting.Spec.Deleting = true
	for _, workflow := range []*v1.Workflow{codex, ona, report, legacy, deleting} {
		server.service.seed(workflow)
	}
	server.service.listPageSize = 1

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: automationQueryConfig(true, 0),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_automation.all", 2),
			querycheck.ExpectIdentity("ona_automation.all", automationQueryIdentity(workflowAccID1)),
			querycheck.ExpectIdentity("ona_automation.all", automationQueryIdentity(workflowAccID2)),
			querycheck.ExpectNoIdentity("ona_automation.all", automationQueryIdentity(workflowAccID3)),
			querycheck.ExpectNoIdentity("ona_automation.all", automationQueryIdentity(automationQueryLegacyID)),
			querycheck.ExpectNoIdentity("ona_automation.all", automationQueryIdentity(automationQueryDeletingID)),
			querycheck.ExpectResourceDisplayName("ona_automation.all", queryfilter.ByResourceIdentity(automationQueryIdentity(workflowAccID2)), knownvalue.StringExact("nightly_checks")),
			querycheck.ExpectResourceDisplayName("ona_automation.all", queryfilter.ByResourceIdentity(automationQueryIdentity(workflowAccID1)), knownvalue.StringExact("nightly_checks_2")),
			querycheck.ExpectResourceKnownValues("ona_automation.all", queryfilter.ByResourceIdentity(automationQueryIdentity(workflowAccID1)), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(workflowAccID1)},
				{Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact("Nightly Checks")},
				{Path: tfjsonpath.New("agent"), KnownValue: knownvalue.StringExact("codex")},
				{Path: tfjsonpath.New("disabled"), KnownValue: knownvalue.Bool(true)},
			}),
			querycheck.ExpectResourceKnownValues("ona_automation.all", queryfilter.ByResourceIdentity(automationQueryIdentity(workflowAccID2)), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("agent"), KnownValue: knownvalue.StringExact("ona")},
			}),
		},
	}))

	filter, calls := server.service.listStats()
	got := struct {
		Calls       int
		FilterEmpty bool
	}{Calls: calls, FilterEmpty: filter != nil && len(filter.GetWorkflowIds()) == 0 && filter.GetSearch() == "" && len(filter.GetCreatorIds()) == 0 && len(filter.GetStatusPhases()) == 0 && filter.GetHasFailedExecutionSince() == nil && filter.Disabled == nil}
	expected := struct {
		Calls       int
		FilterEmpty bool
	}{Calls: 5, FilterEmpty: true}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ListWorkflows requests mismatch (-want +got):\n%s", diff)
	}
}

func TestAccAutomationQueryLimitContinuesPastExcludedAutomations(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)
	valid := testAPIWorkflow(workflowAccID1, "Importable")
	valid.Spec.AgentId = workflowCodexAgentID
	report := testAPIWorkflow(workflowAccID2, "Excluded")
	report.Spec.Report = &v1.WorkflowAction{}
	server.service.seed(valid)
	server.service.seed(report)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: automationQueryConfig(false, 1),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_automation.all", 1),
			querycheck.ExpectIdentity("ona_automation.all", automationQueryIdentity(workflowAccID1)),
		},
	}))

	server.service.mu.Lock()
	got := append([]int32(nil), server.service.listPageSizes...)
	server.service.mu.Unlock()
	if diff := cmp.Diff([]int32{1, 1}, got); diff != "" {
		t.Errorf("ListWorkflows page sizes mismatch (-want +got):\n%s", diff)
	}
}

func TestAccAutomationQueryReportsListError(t *testing.T) {
	t.Parallel()

	server := newWorkflowAPIServer(t)
	t.Cleanup(server.Close)
	server.service.listErr = connect.NewError(connect.CodePermissionDenied, errors.New("automation read denied"))

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      automationQueryConfig(false, 0),
		ExpectError: regexp.MustCompile("Unable to List Ona Automations"),
	}))
}

func automationQueryIdentity(id string) map[string]knownvalue.Check {
	return map[string]knownvalue.Check{"automation_id": knownvalue.StringExact(id)}
}

func automationQueryConfig(includeResource bool, limit int) string {
	includeResourceLine := ""
	if includeResource {
		includeResourceLine = "  include_resource = true\n"
	}
	limitLine := ""
	if limit > 0 {
		limitLine = "  limit            = 1\n"
	}
	return `
list "ona_automation" "all" {
  provider = ona
` + includeResourceLine + limitLine + `}
`
}

const (
	automationQueryLegacyID   = "00000000-0000-0000-0000-000000000004"
	automationQueryDeletingID = "00000000-0000-0000-0000-000000000005"
)
