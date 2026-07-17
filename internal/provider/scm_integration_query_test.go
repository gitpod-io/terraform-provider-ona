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

func TestAccSCMIntegrationQuery(t *testing.T) {
	server := newSCMIntegrationQueryAPIServer(t)
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: scmIntegrationQueryConfig(""),
		QueryResultChecks: []querycheck.QueryResultCheck{
			expectSCMIntegrationQueryResults{
				Expected: []scmIntegrationQueryResult{
					expectedOAuthSCMIntegrationQueryResult(),
					expectedPATSCMIntegrationQueryResult(),
				},
			},
		},
	}))

	if got := server.service.scmSecretUpdateCount(); got != 0 {
		t.Fatalf("SCM integration query wrote %d OAuth secret values", got)
	}
}

func TestAccSCMIntegrationQueryFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Config   string
		Expected []scmIntegrationQueryResult
	}{
		{
			Name: "host",
			Config: scmIntegrationQueryConfig(`
hosts = ["github.com"]
`),
			Expected: []scmIntegrationQueryResult{
				expectedOAuthSCMIntegrationQueryResult(),
			},
		},
		{
			Name: "auth_mode",
			Config: scmIntegrationQueryConfig(`
auth_modes = ["pat"]
`),
			Expected: []scmIntegrationQueryResult{
				expectedPATSCMIntegrationQueryResult(),
			},
		},
		{
			Name: "provider",
			Config: scmIntegrationQueryConfig(`
providers = ["gitlab"]
`),
			Expected: []scmIntegrationQueryResult{
				expectedPATSCMIntegrationQueryResult(),
			},
		},
		{
			Name: "runner_id",
			Config: scmIntegrationQueryConfig(`
runner_ids = ["runner-1"]
`),
			Expected: []scmIntegrationQueryResult{
				expectedOAuthSCMIntegrationQueryResult(),
			},
		},
		{
			Name: "combined_filters",
			Config: scmIntegrationQueryConfig(`
hosts      = ["github.com"]
auth_modes = ["oauth"]
providers  = ["github"]
runner_ids = ["runner-1"]
`),
			Expected: []scmIntegrationQueryResult{
				expectedOAuthSCMIntegrationQueryResult(),
			},
		},
		{
			Name: "non_matching_filters",
			Config: scmIntegrationQueryConfig(`
hosts      = ["github.com"]
auth_modes = ["pat"]
`),
			Expected: []scmIntegrationQueryResult{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newSCMIntegrationQueryAPIServer(t)
			t.Cleanup(server.Close)

			testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
				Query:  true,
				Config: tc.Config,
				QueryResultChecks: []querycheck.QueryResultCheck{
					expectSCMIntegrationQueryResults{Expected: tc.Expected},
				},
			}))

			if got := server.service.scmSecretUpdateCount(); got != 0 {
				t.Fatalf("SCM integration query wrote %d OAuth secret values", got)
			}
		})
	}
}

func TestAccSCMIntegrationQueryRejectsInvalidAuthMode(t *testing.T) {
	server := newSCMIntegrationQueryAPIServer(t)
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query: true,
		Config: scmIntegrationQueryConfig(`
auth_modes = ["basic"]
`),
		ExpectError: regexp.MustCompile("Invalid SCM Authentication Mode"),
	}))
}

func newSCMIntegrationQueryAPIServer(t *testing.T) *runnerConfigurationAPIServer {
	t.Helper()

	server := newRunnerConfigurationAPIServer(t)
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
	return server
}

func scmIntegrationQueryConfig(config string) string {
	config = strings.TrimSpace(config)
	if config == "" {
		return `
list "ona_scm_integration" "all" {
  provider         = ona
  include_resource = true
}
`
	}

	return fmt.Sprintf(`
list "ona_scm_integration" "all" {
  provider         = ona
  include_resource = true

  config {
%s
  }
}
`, indentSCMIntegrationQueryConfig(config))
}

func indentSCMIntegrationQueryConfig(config string) string {
	lines := strings.Split(config, "\n")
	for i := range lines {
		lines[i] = "    " + strings.TrimSpace(lines[i])
	}
	return strings.Join(lines, "\n")
}

func expectedOAuthSCMIntegrationQueryResult() scmIntegrationQueryResult {
	return scmIntegrationQueryResult{
		Address:                              "list.ona_scm_integration.all",
		DisplayName:                          "github.com (github)",
		ID:                                   "scm-1",
		RunnerID:                             "runner-1",
		SCMID:                                "github",
		Host:                                 "github.com",
		AuthMode:                             "oauth",
		OAuthClientID:                        "client-1",
		OAuthClientSecret:                    nil,
		OAuthClientSecretVersion:             nil,
		GeneratedConfigHasOAuthClientSecret:  false,
		GeneratedConfigHasOAuthSecretVersion: false,
	}
}

func expectedPATSCMIntegrationQueryResult() scmIntegrationQueryResult {
	return scmIntegrationQueryResult{
		Address:                              "list.ona_scm_integration.all",
		DisplayName:                          "gitlab.com (gitlab)",
		ID:                                   "scm-2",
		RunnerID:                             "runner-2",
		SCMID:                                "gitlab",
		Host:                                 "gitlab.com",
		AuthMode:                             "pat",
		OAuthClientID:                        nil,
		OAuthClientSecret:                    nil,
		OAuthClientSecretVersion:             nil,
		GeneratedConfigHasOAuthClientSecret:  false,
		GeneratedConfigHasOAuthSecretVersion: false,
	}
}

type expectSCMIntegrationQueryResults struct {
	Expected []scmIntegrationQueryResult
}

type scmIntegrationQueryResult struct {
	Address                              string
	DisplayName                          string
	ID                                   string
	RunnerID                             string
	SCMID                                string
	Host                                 string
	AuthMode                             string
	OAuthClientID                        any
	OAuthClientSecret                    any
	OAuthClientSecretVersion             any
	GeneratedConfigHasOAuthClientSecret  bool
	GeneratedConfigHasOAuthSecretVersion bool
}

func (e expectSCMIntegrationQueryResults) CheckQuery(_ context.Context, req querycheck.CheckQueryRequest, resp *querycheck.CheckQueryResponse) {
	got := make([]scmIntegrationQueryResult, 0, len(req.Query))
	for _, result := range req.Query {
		got = append(got, scmIntegrationQueryResult{
			Address:                              result.Address,
			DisplayName:                          result.DisplayName,
			ID:                                   stringMapValue(result.Identity, "id"),
			RunnerID:                             stringMapValue(result.ResourceObject, "runner_id"),
			SCMID:                                stringMapValue(result.ResourceObject, "scm_id"),
			Host:                                 stringMapValue(result.ResourceObject, "host"),
			AuthMode:                             stringMapValue(result.ResourceObject, "auth_mode"),
			OAuthClientID:                        result.ResourceObject["oauth_client_id"],
			OAuthClientSecret:                    result.ResourceObject["oauth_client_secret"],
			OAuthClientSecretVersion:             result.ResourceObject["oauth_client_secret_version"],
			GeneratedConfigHasOAuthClientSecret:  strings.Contains(result.Config, "oauth_client_secret"),
			GeneratedConfigHasOAuthSecretVersion: strings.Contains(result.Config, "oauth_client_secret_version"),
		})
	}
	sort.Slice(got, func(i, j int) bool {
		return got[i].ID < got[j].ID
	})

	if diff := cmp.Diff(e.Expected, got); diff != "" {
		resp.Error = fmt.Errorf("SCM integration query results mismatch (-want +got):\n%s", diff)
	}
}
