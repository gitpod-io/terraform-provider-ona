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

func TestAccOrganizationPoliciesQuery(t *testing.T) {
	server := newPolicyAPIServer(t)
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: organizationPoliciesQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_organization_policies.all", 1),
			querycheck.ExpectIdentity("ona_organization_policies.all", map[string]knownvalue.Check{
				"organization_id": knownvalue.StringExact("org-1"),
			}),
			querycheck.ExpectResourceKnownValues(
				"ona_organization_policies.all",
				queryfilter.ByDisplayName(knownvalue.StringExact("org-1")),
				[]querycheck.KnownValueCheck{
					{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("org-1")},
					{Path: tfjsonpath.New("members_create_projects"), KnownValue: knownvalue.Bool(true)},
				},
			),
		},
	}))
}

func organizationPoliciesQueryConfig() string {
	return `
list "ona_organization_policies" "all" {
  provider         = ona
  include_resource = true

  config {
    organization_id = "org-1"
  }
}
`
}
