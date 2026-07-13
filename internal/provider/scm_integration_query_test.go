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

func TestAccSCMIntegrationQuery(t *testing.T) {
	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{
		Id:       "scm-1",
		RunnerId: "runner-1",
		ScmId:    "github",
		Host:     "github.com",
		Oauth:    &v1.SCMIntegrationOAuthConfig{ClientId: "client-1"},
	}
	server.service.scmIntegrations["scm-2"] = &v1.SCMIntegration{
		Id:       "scm-2",
		RunnerId: "runner-2",
		ScmId:    "gitlab",
		Host:     "gitlab.com",
		Pat:      true,
	}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: scmIntegrationQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_scm_integration.all", 1),
			querycheck.ExpectIdentity("ona_scm_integration.all", map[string]knownvalue.Check{
				"id": knownvalue.StringExact("scm-1"),
			}),
			querycheck.ExpectResourceKnownValues(
				"ona_scm_integration.all",
				queryfilter.ByDisplayName(knownvalue.StringExact("github.com (github)")),
				[]querycheck.KnownValueCheck{
					{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("scm-1")},
					{Path: tfjsonpath.New("runner_id"), KnownValue: knownvalue.StringExact("runner-1")},
					{Path: tfjsonpath.New("oauth_client_secret"), KnownValue: knownvalue.Null()},
					{Path: tfjsonpath.New("oauth_client_secret_version"), KnownValue: knownvalue.Null()},
				},
			),
		},
	}))

	if got := server.service.scmSecretUpdateCount(); got != 0 {
		t.Fatalf("SCM integration query wrote %d OAuth secret values", got)
	}
}

func scmIntegrationQueryConfig() string {
	return `
list "ona_scm_integration" "all" {
  provider         = ona
  include_resource = true

  config {
    runner_ids = ["runner-1"]
  }
}
`
}
