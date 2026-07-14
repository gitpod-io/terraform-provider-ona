// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccRunnerQuery(t *testing.T) {
	managedRunner := newTestRunner("runner-2", "Managed Runner")
	managedRunner.Kind = v1.RunnerKind_RUNNER_KIND_LOCAL_CONFIGURATION
	gcpRunner := newTestRunner("runner-3", "GCP Runner")
	gcpRunner.Provider = v1.RunnerProvider_RUNNER_PROVIDER_GCP
	gcpRunner.Creator.Id = "creator-2"
	gcpRunner.RunnerManagerId = "runner-manager-1"
	devAgentRunner := newTestRunner("runner-4", "Dev Agent Runner")
	devAgentRunner.Provider = v1.RunnerProvider_RUNNER_PROVIDER_DEV_AGENT

	server := newRunnerAPIServer(t, map[string]*v1.Runner{
		"runner-1": newTestRunner("runner-1", "AWS Runner"),
		"runner-2": managedRunner,
		"runner-3": gcpRunner,
		"runner-4": devAgentRunner,
	})
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: runnerQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_runner.all", 1),
			querycheck.ExpectIdentity("ona_runner.all", map[string]knownvalue.Check{
				"runner_id": knownvalue.StringExact("runner-3"),
			}),
			querycheck.ExpectResourceDisplayName(
				"ona_runner.all",
				queryfilter.ByDisplayName(knownvalue.StringExact("GCP Runner")),
				knownvalue.StringExact("GCP Runner"),
			),
			querycheck.ExpectResourceKnownValues(
				"ona_runner.all",
				queryfilter.ByDisplayName(knownvalue.StringExact("GCP Runner")),
				[]querycheck.KnownValueCheck{
					{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("runner-3")},
					{Path: tfjsonpath.New("runner_id"), KnownValue: knownvalue.StringExact("runner-3")},
					{Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact("GCP Runner")},
					{Path: tfjsonpath.New("runner_provider"), KnownValue: knownvalue.StringExact("gcp")},
					{Path: tfjsonpath.New("runner_manager_id"), KnownValue: knownvalue.Null()},
				},
			),
		},
	}))

	if got := server.service.tokenCount(); got != 0 {
		t.Fatalf("runner query created %d token values", got)
	}
}

func runnerQueryConfig() string {
	return `
list "ona_runner" "all" {
  provider         = ona
  include_resource = true
  limit            = 1

  config {
    creator_ids      = ["creator-2"]
    runner_providers = ["gcp"]
  }
}
`
}
