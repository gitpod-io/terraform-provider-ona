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

func TestAccTermsOfServiceQuery(t *testing.T) {
	server := newOrganizationCommunicationsAPIServer(t)
	t.Cleanup(server.Close)
	version := &v1.TermsOfServiceVersion{Id: "terms-version-1", Version: 1, Markdown: "# Terms"}
	server.service.terms = &v1.TermsOfService{OrganizationId: organizationCommunicationsOrgID, Enabled: true, CurrentVersion: version}
	server.service.versions = []*v1.TermsOfServiceVersion{version}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query: true, Config: termsOfServiceQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_terms_of_service.all", 1),
			querycheck.ExpectIdentity("ona_terms_of_service.all", map[string]knownvalue.Check{
				"organization_id": knownvalue.StringExact(organizationCommunicationsOrgID),
			}),
			querycheck.ExpectResourceKnownValues("ona_terms_of_service.all", queryfilter.ByDisplayName(knownvalue.StringExact(organizationCommunicationsOrgID)), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(organizationCommunicationsOrgID)},
				{Path: tfjsonpath.New("enabled"), KnownValue: knownvalue.Bool(true)},
				{Path: tfjsonpath.New("markdown"), KnownValue: knownvalue.StringExact("# Terms")},
			}),
		},
	}))
}

func termsOfServiceQueryConfig() string {
	return `
list "ona_terms_of_service" "all" {
  provider         = ona
  include_resource = true
  config { organization_id = "org-1" }
}
`
}
