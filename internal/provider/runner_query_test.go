// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/google/go-cmp/cmp"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
)

func TestAccRunnerQuery(t *testing.T) {
	localRunner := newTestRunner("runner-2", "Local Runner")
	localRunner.Kind = v1.RunnerKind_RUNNER_KIND_LOCAL_CONFIGURATION
	gcpRunner := newTestRunner("runner-3", "GCP Runner")
	gcpRunner.Provider = v1.RunnerProvider_RUNNER_PROVIDER_GCP
	gcpRunner.Creator.Id = "creator-2"
	managedRunner := newTestRunner("runner-4", "Managed Runner")
	managedRunner.Provider = v1.RunnerProvider_RUNNER_PROVIDER_MANAGED
	devAgentRunner := newTestRunner("runner-5", "Dev Agent Runner")
	devAgentRunner.Provider = v1.RunnerProvider_RUNNER_PROVIDER_DEV_AGENT

	server := newRunnerAPIServer(t, map[string]*v1.Runner{
		"runner-1": newTestRunner("runner-1", "AWS Runner"),
		"runner-2": localRunner,
		"runner-3": gcpRunner,
		"runner-4": managedRunner,
		"runner-5": devAgentRunner,
	})
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: runnerQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			expectRunnerQueryResults{
				Expected: []runnerQueryResult{
					{
						Address:                           "list.ona_runner.all",
						DisplayName:                       "AWS Runner",
						RunnerID:                          "runner-1",
						Name:                              "AWS Runner",
						RunnerProvider:                    "aws_ec2",
						RunnerManagerID:                   nil,
						GeneratedConfigHasRunnerManagerID: false,
					},
					{
						Address:                           "list.ona_runner.all",
						DisplayName:                       "GCP Runner",
						RunnerID:                          "runner-3",
						Name:                              "GCP Runner",
						RunnerProvider:                    "gcp",
						RunnerManagerID:                   nil,
						GeneratedConfigHasRunnerManagerID: false,
					},
				},
			},
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
}
`
}

type expectRunnerQueryResults struct {
	Expected []runnerQueryResult
}

type runnerQueryResult struct {
	Address                           string
	DisplayName                       string
	RunnerID                          string
	Name                              string
	RunnerProvider                    string
	RunnerManagerID                   any
	GeneratedConfigHasRunnerManagerID bool
}

func (e expectRunnerQueryResults) CheckQuery(_ context.Context, req querycheck.CheckQueryRequest, resp *querycheck.CheckQueryResponse) {
	got := make([]runnerQueryResult, 0, len(req.Query))
	for _, result := range req.Query {
		got = append(got, runnerQueryResult{
			Address:                           result.Address,
			DisplayName:                       result.DisplayName,
			RunnerID:                          stringMapValue(result.Identity, "runner_id"),
			Name:                              stringMapValue(result.ResourceObject, "name"),
			RunnerProvider:                    stringMapValue(result.ResourceObject, "runner_provider"),
			RunnerManagerID:                   result.ResourceObject["runner_manager_id"],
			GeneratedConfigHasRunnerManagerID: strings.Contains(result.Config, "runner_manager_id"),
		})
	}
	sort.Slice(got, func(i, j int) bool {
		return got[i].RunnerID < got[j].RunnerID
	})

	if diff := cmp.Diff(e.Expected, got); diff != "" {
		resp.Error = fmt.Errorf("runner query results mismatch (-want +got):\n%s", diff)
	}
}

func stringMapValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}
