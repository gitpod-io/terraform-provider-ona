// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccWarmPoolQuery(t *testing.T) {
	server := newWarmPoolAPIServer(t)
	t.Cleanup(server.Close)

	server.service.put(server.service.newWarmPool("warm-pool-2", "project-2", "class-2", 0, 1))
	server.service.put(server.service.newWarmPool("warm-pool-1", "project-1", "class-1", 1, 2))

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: warmPoolQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_warm_pool.all", 1),
			querycheck.ExpectIdentity("ona_warm_pool.all", map[string]knownvalue.Check{
				"id": knownvalue.StringExact("warm-pool-1"),
			}),
			querycheck.ExpectResourceKnownValues(
				"ona_warm_pool.all",
				queryfilter.ByDisplayName(knownvalue.StringExact("warm-pool-1")),
				[]querycheck.KnownValueCheck{
					{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("warm-pool-1")},
					{Path: tfjsonpath.New("project_id"), KnownValue: knownvalue.StringExact("project-1")},
					{Path: tfjsonpath.New("environment_class_id"), KnownValue: knownvalue.StringExact("class-1")},
					{Path: tfjsonpath.New("min_size"), KnownValue: knownvalue.Int32Exact(1)},
					{Path: tfjsonpath.New("max_size"), KnownValue: knownvalue.Int32Exact(2)},
				},
			),
		},
	}))
}

func warmPoolQueryConfig() string {
	return `
list "ona_warm_pool" "all" {
  provider         = ona
  include_resource = true
  limit            = 1

  config {
    project_ids = ["project-1"]
  }
}
`
}
