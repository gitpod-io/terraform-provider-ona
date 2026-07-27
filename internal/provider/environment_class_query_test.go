// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
)

func TestAccEnvironmentClassQuery(t *testing.T) {
	server := newEnvironmentClassQueryAPIServer(t)
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: environmentClassQueryConfig(""),
		QueryResultChecks: []querycheck.QueryResultCheck{
			expectEnvironmentClassQueryResults{
				Expected: expectedSupportedEnvironmentClassQueryResults(),
			},
		},
	}))
}

func TestAccEnvironmentClassQueryFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Config   string
		Expected []environmentClassQueryResult
	}{
		{
			Name:   "matching_runner_id",
			Config: `runner_ids = ["runner-1"]`,
			Expected: []environmentClassQueryResult{
				expectedLargeEnvironmentClassQueryResult(),
			},
		},
		{
			Name:     "unsupported_runner_provider",
			Config:   `runner_ids = ["runner-3"]`,
			Expected: []environmentClassQueryResult{},
		},
		{
			Name:   "aws_ec2_provider",
			Config: `providers = ["aws_ec2"]`,
			Expected: []environmentClassQueryResult{
				expectedLargeEnvironmentClassQueryResult(),
			},
		},
		{
			Name:   "gcp_provider",
			Config: `providers = ["gcp"]`,
			Expected: []environmentClassQueryResult{
				expectedSmallEnvironmentClassQueryResult(),
				expectedDisabledEnvironmentClassQueryResult(),
			},
		},
		{
			Name:   "enabled_environment_classes",
			Config: "enabled = true",
			Expected: []environmentClassQueryResult{
				expectedLargeEnvironmentClassQueryResult(),
				expectedSmallEnvironmentClassQueryResult(),
			},
		},
		{
			Name:   "disabled_environment_classes",
			Config: "enabled = false",
			Expected: []environmentClassQueryResult{
				expectedDisabledEnvironmentClassQueryResult(),
			},
		},
		{
			Name: "combined_provider_and_enabled_filters",
			Config: `
providers = ["gcp"]
enabled   = true
`,
			Expected: []environmentClassQueryResult{
				expectedSmallEnvironmentClassQueryResult(),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newEnvironmentClassQueryAPIServer(t)
			t.Cleanup(server.Close)

			testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
				Query:  true,
				Config: environmentClassQueryConfig(tc.Config),
				QueryResultChecks: []querycheck.QueryResultCheck{
					expectEnvironmentClassQueryResults{Expected: tc.Expected},
				},
			}))
		})
	}
}

func TestAccEnvironmentClassQueryRejectsInvalidProvider(t *testing.T) {
	server := newEnvironmentClassQueryAPIServer(t)
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query: true,
		Config: environmentClassQueryConfig(`
providers = ["managed"]
`),
		ExpectError: regexp.MustCompile("Invalid Environment Class Provider"),
	}))
}

func TestAccEnvironmentClassQueryDeduplicatesDisplayNames(t *testing.T) {
	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)

	server.runnerService.runners["runner-1"] = newTestRunner("runner-1", "AWS Runner")
	server.service.environmentClasses["class-1"] = &v1.EnvironmentClass{
		Id:          "class-1",
		RunnerId:    "runner-1",
		DisplayName: "Large",
		Enabled:     true,
	}
	server.service.environmentClasses["class-2"] = &v1.EnvironmentClass{
		Id:          "class-2",
		RunnerId:    "runner-1",
		DisplayName: "Large",
		Enabled:     true,
	}
	server.service.environmentClassRunnerProviders["runner-1"] = v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: environmentClassQueryConfig(""),
		QueryResultChecks: []querycheck.QueryResultCheck{
			expectEnvironmentClassQueryResults{
				Expected: []environmentClassQueryResult{
					{
						Address:       "list.ona_environment_class.all",
						DisplayName:   "aws_runner_large",
						IdentityID:    "class-1",
						ResourceID:    "class-1",
						RunnerID:      "runner-1",
						Configuration: map[string]any{},
						Enabled:       true,
					},
					{
						Address:       "list.ona_environment_class.all",
						DisplayName:   "aws_runner_large_2",
						IdentityID:    "class-2",
						ResourceID:    "class-2",
						RunnerID:      "runner-1",
						Configuration: map[string]any{},
						Enabled:       true,
					},
				},
			},
		},
	}))
}

func TestAccEnvironmentClassQueryReportsRunnerListError(t *testing.T) {
	server := newEnvironmentClassQueryAPIServer(t)
	t.Cleanup(server.Close)

	server.runnerService.listErr = fmt.Errorf("runner list failed")

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      environmentClassQueryConfig(""),
		ExpectError: regexp.MustCompile("Unable to List Ona Runners"),
	}))
}

func newEnvironmentClassQueryAPIServer(t *testing.T) *runnerConfigurationAPIServer {
	t.Helper()

	server := newRunnerConfigurationAPIServer(t)
	server.runnerService.runners["runner-1"] = newTestRunner("runner-1", "AWS Runner")
	gcpRunner := newTestRunner("runner-2", "GCP Runner")
	gcpRunner.Provider = v1.RunnerProvider_RUNNER_PROVIDER_GCP
	server.runnerService.runners["runner-2"] = gcpRunner
	managedRunner := newTestRunner("runner-3", "Managed Runner")
	managedRunner.Provider = v1.RunnerProvider_RUNNER_PROVIDER_MANAGED
	server.runnerService.runners["runner-3"] = managedRunner

	server.service.environmentClasses["class-1"] = &v1.EnvironmentClass{
		Id:            "class-1",
		RunnerId:      "runner-1",
		DisplayName:   "Large",
		Description:   "Large test environment class",
		Configuration: []*v1.FieldValue{{Key: "machine_type", Value: "n2-standard-8"}},
		Enabled:       true,
	}
	server.service.environmentClasses["class-2"] = &v1.EnvironmentClass{
		Id:            "class-2",
		RunnerId:      "runner-2",
		DisplayName:   "Small",
		Description:   "Small test environment class",
		Configuration: []*v1.FieldValue{{Key: "machine_type", Value: "e2-standard-4"}},
		Enabled:       true,
	}
	server.service.environmentClasses["class-3"] = &v1.EnvironmentClass{
		Id:          "class-3",
		RunnerId:    "runner-3",
		DisplayName: "Managed",
		Description: "Unsupported managed runner environment class",
		Enabled:     true,
	}
	server.service.environmentClasses["class-4"] = &v1.EnvironmentClass{
		Id:          "class-4",
		RunnerId:    "runner-2",
		DisplayName: "Disabled",
		Description: "Disabled test environment class",
		Enabled:     false,
	}
	server.service.environmentClassRunnerProviders["runner-1"] = v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2
	server.service.environmentClassRunnerProviders["runner-2"] = v1.RunnerProvider_RUNNER_PROVIDER_GCP
	server.service.environmentClassRunnerProviders["runner-3"] = v1.RunnerProvider_RUNNER_PROVIDER_MANAGED
	return server
}

func environmentClassQueryConfig(config string) string {
	config = strings.TrimSpace(config)
	if config == "" {
		return `
list "ona_environment_class" "all" {
  provider         = ona
  include_resource = true
}
`
	}

	return fmt.Sprintf(`
list "ona_environment_class" "all" {
  provider         = ona
  include_resource = true

  config {
%s
  }
}
`, indentEnvironmentClassQueryConfig(config))
}

func indentEnvironmentClassQueryConfig(config string) string {
	lines := strings.Split(config, "\n")
	for i := range lines {
		lines[i] = "    " + strings.TrimSpace(lines[i])
	}
	return strings.Join(lines, "\n")
}

func expectedSupportedEnvironmentClassQueryResults() []environmentClassQueryResult {
	return []environmentClassQueryResult{
		expectedLargeEnvironmentClassQueryResult(),
		expectedSmallEnvironmentClassQueryResult(),
		expectedDisabledEnvironmentClassQueryResult(),
	}
}

func expectedLargeEnvironmentClassQueryResult() environmentClassQueryResult {
	return environmentClassQueryResult{
		Address:       "list.ona_environment_class.all",
		DisplayName:   "aws_runner_large",
		IdentityID:    "class-1",
		ResourceID:    "class-1",
		RunnerID:      "runner-1",
		Description:   "Large test environment class",
		Configuration: map[string]any{"machine_type": "n2-standard-8"},
		Enabled:       true,
	}
}

func expectedSmallEnvironmentClassQueryResult() environmentClassQueryResult {
	return environmentClassQueryResult{
		Address:       "list.ona_environment_class.all",
		DisplayName:   "gcp_runner_small",
		IdentityID:    "class-2",
		ResourceID:    "class-2",
		RunnerID:      "runner-2",
		Description:   "Small test environment class",
		Configuration: map[string]any{"machine_type": "e2-standard-4"},
		Enabled:       true,
	}
}

func expectedDisabledEnvironmentClassQueryResult() environmentClassQueryResult {
	return environmentClassQueryResult{
		Address:       "list.ona_environment_class.all",
		DisplayName:   "gcp_runner_disabled",
		IdentityID:    "class-4",
		ResourceID:    "class-4",
		RunnerID:      "runner-2",
		Description:   "Disabled test environment class",
		Configuration: map[string]any{},
		Enabled:       false,
	}
}

type expectEnvironmentClassQueryResults struct {
	Expected []environmentClassQueryResult
}

type environmentClassQueryResult struct {
	Address       string
	DisplayName   string
	IdentityID    string
	ResourceID    string
	RunnerID      string
	Description   string
	Configuration map[string]any
	Enabled       bool
}

func (e expectEnvironmentClassQueryResults) CheckQuery(_ context.Context, req querycheck.CheckQueryRequest, resp *querycheck.CheckQueryResponse) {
	got := make([]environmentClassQueryResult, 0, len(req.Query))
	for _, result := range req.Query {
		configuration, _ := result.ResourceObject["configuration"].(map[string]any)
		enabled, _ := result.ResourceObject["enabled"].(bool)
		got = append(got, environmentClassQueryResult{
			Address:       result.Address,
			DisplayName:   result.DisplayName,
			IdentityID:    stringMapValue(result.Identity, "id"),
			ResourceID:    stringMapValue(result.ResourceObject, "id"),
			RunnerID:      stringMapValue(result.ResourceObject, "runner_id"),
			Description:   stringMapValue(result.ResourceObject, "description"),
			Configuration: configuration,
			Enabled:       enabled,
		})
	}
	sort.Slice(got, func(i, j int) bool {
		return got[i].IdentityID < got[j].IdentityID
	})

	if diff := cmp.Diff(e.Expected, got); diff != "" {
		resp.Error = fmt.Errorf("environment class query results mismatch (-want +got):\n%s", diff)
	}
}
