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

func TestAccEnvironmentClassQuery(t *testing.T) {
	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.environmentClasses["class-1"] = &v1.EnvironmentClass{
		Id:            "class-1",
		RunnerId:      "runner-1",
		DisplayName:   "Large",
		Description:   "Large test environment class",
		Configuration: []*v1.FieldValue{{Key: "machine_type", Value: "n2-standard-8"}},
		Enabled:       true,
	}
	server.service.environmentClasses["class-2"] = &v1.EnvironmentClass{
		Id:          "class-2",
		RunnerId:    "runner-2",
		DisplayName: "Small",
		Description: "Small test environment class",
		Enabled:     true,
	}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: environmentClassQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_environment_class.all", 1),
			querycheck.ExpectIdentity("ona_environment_class.all", map[string]knownvalue.Check{
				"id": knownvalue.StringExact("class-1"),
			}),
			querycheck.ExpectResourceKnownValues(
				"ona_environment_class.all",
				queryfilter.ByDisplayName(knownvalue.StringExact("Large")),
				[]querycheck.KnownValueCheck{
					{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("class-1")},
					{Path: tfjsonpath.New("runner_id"), KnownValue: knownvalue.StringExact("runner-1")},
					{Path: tfjsonpath.New("display_name"), KnownValue: knownvalue.StringExact("Large")},
				},
			),
		},
	}))
}

func environmentClassQueryConfig() string {
	return `
list "ona_environment_class" "all" {
  provider         = ona
  include_resource = true

  config {
    runner_ids = ["runner-1"]
  }
}
`
}
