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

func TestAccRunnerPolicyQuery(t *testing.T) {
	server := newRunnerAPIServer(t, map[string]*v1.Runner{
		"runner-1": newTestRunner("runner-1", "Shared Runner"),
	})
	t.Cleanup(server.Close)
	server.service.policies = map[string]*v1.RunnerPolicy{
		"runner-1/group-1": {GroupId: "group-1", Role: v1.RunnerRole_RUNNER_ROLE_USER},
		"runner-1/group-2": {GroupId: "group-2", Role: v1.RunnerRole_RUNNER_ROLE_ADMIN},
	}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: runnerPolicyQueryConfig("runner-1"),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_runner_policy.all", 1),
			querycheck.ExpectIdentity("ona_runner_policy.all", map[string]knownvalue.Check{
				"runner_id": knownvalue.StringExact("runner-1"),
				"group_id":  knownvalue.StringExact("group-1"),
			}),
			querycheck.ExpectResourceKnownValues(
				"ona_runner_policy.all",
				queryfilter.ByDisplayName(knownvalue.StringExact("runner-1 / group-1")),
				[]querycheck.KnownValueCheck{
					{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("runner-1/group-1")},
					{Path: tfjsonpath.New("runner_id"), KnownValue: knownvalue.StringExact("runner-1")},
					{Path: tfjsonpath.New("group_id"), KnownValue: knownvalue.StringExact("group-1")},
					{Path: tfjsonpath.New("role"), KnownValue: knownvalue.StringExact("user")},
				},
			),
		},
	}))
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

func runnerPolicyQueryConfig(runnerID string) string {
	return fmt.Sprintf(`
list "ona_runner_policy" "all" {
  provider         = ona
  include_resource = true

  config {
    runner_ids = [%q]
  }
}
`, runnerID)
}
