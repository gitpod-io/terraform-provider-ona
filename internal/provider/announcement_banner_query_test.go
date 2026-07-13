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

func TestAccAnnouncementBannerQuery(t *testing.T) {
	server := newOrganizationCommunicationsAPIServer(t)
	t.Cleanup(server.Close)
	server.service.banner = &v1.AnnouncementBanner{
		OrganizationId: organizationCommunicationsOrgID,
		Enabled:        true,
		Message:        "Scheduled maintenance",
	}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: announcementBannerQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_announcement_banner.all", 1),
			querycheck.ExpectIdentity("ona_announcement_banner.all", map[string]knownvalue.Check{
				"organization_id": knownvalue.StringExact(organizationCommunicationsOrgID),
			}),
			querycheck.ExpectResourceKnownValues(
				"ona_announcement_banner.all",
				queryfilter.ByDisplayName(knownvalue.StringExact(organizationCommunicationsOrgID)),
				[]querycheck.KnownValueCheck{
					{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(organizationCommunicationsOrgID)},
					{Path: tfjsonpath.New("enabled"), KnownValue: knownvalue.Bool(true)},
					{Path: tfjsonpath.New("message"), KnownValue: knownvalue.StringExact("Scheduled maintenance")},
				},
			),
		},
	}))
}

func announcementBannerQueryConfig() string {
	return `
list "ona_announcement_banner" "all" {
  provider         = ona
  include_resource = true

  config {
    organization_id = "org-1"
  }
}
`
}
