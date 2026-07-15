// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccRunnerPolicyQueryWithRunnerIDs(t *testing.T) {
	server := newRunnerPolicyQueryAPIServer(t)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: runnerPolicyQueryConfig("runner-1"),
		QueryResultChecks: runnerPolicyQueryResultChecks(
			runnerPolicyQueryExpectation{RunnerID: "runner-1", GroupID: "group-1"},
		),
	}))
}

func TestAccRunnerPolicyQueryWithoutRunnerIDs(t *testing.T) {
	server := newRunnerPolicyQueryAPIServer(t)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: runnerPolicyQueryConfig(),
		QueryResultChecks: runnerPolicyQueryResultChecks(
			runnerPolicyQueryExpectation{RunnerID: "runner-1", GroupID: "group-1"},
			runnerPolicyQueryExpectation{RunnerID: "runner-2", GroupID: "group-2"},
		),
	}))
}

func newRunnerPolicyQueryAPIServer(t *testing.T) *runnerAPIServer {
	t.Helper()

	server := newRunnerAPIServer(t, map[string]*v1.Runner{
		"runner-1": newTestRunner("runner-1", "Shared Runner"),
		"runner-2": newTestRunner("runner-2", "Secondary Runner"),
	})
	t.Cleanup(server.Close)
	server.service.policies = map[string]*v1.RunnerPolicy{
		"runner-1/group-1":     {GroupId: "group-1", Role: v1.RunnerRole_RUNNER_ROLE_USER},
		"runner-1/group-admin": {GroupId: "group-admin", Role: v1.RunnerRole_RUNNER_ROLE_ADMIN},
		"runner-2/group-2":     {GroupId: "group-2", Role: v1.RunnerRole_RUNNER_ROLE_USER},
	}
	return server
}

type runnerPolicyQueryExpectation struct {
	RunnerID string
	GroupID  string
}

func runnerPolicyQueryResultChecks(expected ...runnerPolicyQueryExpectation) []querycheck.QueryResultCheck {
	checks := []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_runner_policy.all", len(expected)),
	}
	for _, policy := range expected {
		checks = append(checks,
			querycheck.ExpectIdentity("ona_runner_policy.all", map[string]knownvalue.Check{
				"runner_id": knownvalue.StringExact(policy.RunnerID),
				"group_id":  knownvalue.StringExact(policy.GroupID),
			}),
			querycheck.ExpectResourceKnownValues(
				"ona_runner_policy.all",
				queryfilter.ByDisplayName(knownvalue.StringExact(fmt.Sprintf("%s / %s", policy.RunnerID, policy.GroupID))),
				[]querycheck.KnownValueCheck{
					{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(fmt.Sprintf("%s/%s", policy.RunnerID, policy.GroupID))},
					{Path: tfjsonpath.New("runner_id"), KnownValue: knownvalue.StringExact(policy.RunnerID)},
					{Path: tfjsonpath.New("group_id"), KnownValue: knownvalue.StringExact(policy.GroupID)},
					{Path: tfjsonpath.New("role"), KnownValue: knownvalue.StringExact("user")},
				},
			),
		)
	}
	return checks
}

func TestAccRunnerPolicyQueryRejectsUnknownRunner(t *testing.T) {
	server := newRunnerAPIServer(t, map[string]*v1.Runner{
		"runner-1": newTestRunner("runner-1", "Shared Runner"),
	})
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      runnerPolicyQueryConfig("missing-runner"),
		ExpectError: regexp.MustCompile("Unable to List Ona Runner Policies"),
	}))
}

func runnerPolicyQueryConfig(runnerIDs ...string) string {
	if len(runnerIDs) == 0 {
		return `
list "ona_runner_policy" "all" {
  provider         = ona
  include_resource = true
}
`
	}

	return fmt.Sprintf(`
list "ona_runner_policy" "all" {
  provider         = ona
  include_resource = true

  config {
    runner_ids = [%q]
  }
}
`, runnerIDs[0])
}
