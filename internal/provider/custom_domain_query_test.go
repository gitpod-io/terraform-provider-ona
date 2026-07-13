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

func TestAccCustomDomainQuery(t *testing.T) {
	server := newCustomDomainAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed("ona.example.com", v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_AWS, "123456789012")

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query: true, Config: customDomainQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_custom_domain.all", 1),
			querycheck.ExpectIdentity("ona_custom_domain.all", map[string]knownvalue.Check{"organization_id": knownvalue.StringExact(customDomainOrgID)}),
			querycheck.ExpectResourceKnownValues("ona_custom_domain.all", queryfilter.ByDisplayName(knownvalue.StringExact("ona.example.com")), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("domain_name"), KnownValue: knownvalue.StringExact("ona.example.com")},
				{Path: tfjsonpath.New("cloud_provider"), KnownValue: knownvalue.StringExact("aws")},
				{Path: tfjsonpath.New("cloud_account_id"), KnownValue: knownvalue.StringExact("123456789012")},
			}),
		},
	}))
}

func customDomainQueryConfig() string {
	return `
list "ona_custom_domain" "all" {
  provider         = ona
  include_resource = true
  config { organization_id = "11111111-1111-4111-8111-111111111111" }
}
`
}
