// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAccGitAuthenticationQuery(t *testing.T) {
	t.Parallel()

	server := newGitAuthenticationQueryAPIServer(t)
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: gitAuthenticationQueryConfig(""),
		QueryResultChecks: []querycheck.QueryResultCheck{
			expectGitAuthenticationQueryResults{Expected: []gitAuthenticationQueryResult{expectedGitAuthenticationQueryResult()}},
		},
	}))
}

func TestAccGitAuthenticationQueryFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Config   string
		Expected []gitAuthenticationQueryResult
	}{
		{Name: "service_account", Config: `service_account_id = "service-account-1"`, Expected: []gitAuthenticationQueryResult{expectedGitAuthenticationQueryResult()}},
		{Name: "scm_integration", Config: `scm_integration_id = "scm-1"`, Expected: []gitAuthenticationQueryResult{expectedGitAuthenticationQueryResult()}},
		{Name: "non_matching_service_account", Config: `service_account_id = "service-account-missing"`, Expected: []gitAuthenticationQueryResult{}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			server := newGitAuthenticationQueryAPIServer(t)
			t.Cleanup(server.Close)
			testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
				Query:  true,
				Config: gitAuthenticationQueryConfig(tc.Config),
				QueryResultChecks: []querycheck.QueryResultCheck{
					expectGitAuthenticationQueryResults{Expected: tc.Expected},
				},
			}))
		})
	}
}

func TestAccGitAuthenticationQueryPaginatesAndBackfillsLimit(t *testing.T) {
	t.Parallel()

	server := newRunnerConfigurationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}
	server.service.hostAuthenticationTokens["git-auth-1-unsupported"] = gitAuthenticationQueryToken("git-auth-1-unsupported", "user-1", v1.Principal_PRINCIPAL_USER, v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT, "scm-1", "runner-1", "github.com")
	server.service.hostAuthenticationTokens["git-auth-2"] = gitAuthenticationQueryToken("git-auth-2", "service-account-1", v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT, "scm-1", "runner-1", "github.com")
	server.service.hostAuthenticationPageSizeLimit = 1

	expected := expectedGitAuthenticationQueryResult()
	expected.ID = "git-auth-2"
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: gitAuthenticationLimitedQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			expectGitAuthenticationQueryResults{Expected: []gitAuthenticationQueryResult{expected}},
		},
	}))

	wantRequests := []*v1.ListHostAuthenticationTokensRequest{
		{Pagination: &v1.PaginationRequest{PageSize: 1}, Filter: &v1.ListHostAuthenticationTokensRequest_Filter{}},
		{Pagination: &v1.PaginationRequest{PageSize: 1, Token: "1"}, Filter: &v1.ListHostAuthenticationTokensRequest_Filter{}},
	}
	if diff := cmp.Diff(wantRequests, server.service.hostAuthenticationListRequestsSnapshot(), protocmp.Transform()); diff != "" {
		t.Errorf("Git authentication list requests mismatch (-want +got):\n%s", diff)
	}
}

func newGitAuthenticationQueryAPIServer(t *testing.T) *runnerConfigurationAPIServer {
	t.Helper()

	server := newRunnerConfigurationAPIServer(t)
	server.service.scmIntegrations["scm-1"] = &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", ScmId: "github", Host: "github.com", Pat: true}
	server.service.scmIntegrations["scm-2"] = &v1.SCMIntegration{Id: "scm-2", RunnerId: "runner-2", ScmId: "gitlab", Host: "gitlab.com"}
	server.service.hostAuthenticationTokens["git-auth-1"] = gitAuthenticationQueryToken("git-auth-1", "service-account-1", v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT, "scm-1", "runner-1", "github.com")
	server.service.hostAuthenticationTokens["git-auth-user"] = gitAuthenticationQueryToken("git-auth-user", "user-1", v1.Principal_PRINCIPAL_USER, v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT, "scm-1", "runner-1", "github.com")
	server.service.hostAuthenticationTokens["git-auth-oauth"] = gitAuthenticationQueryToken("git-auth-oauth", "service-account-1", v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_OAUTH, "scm-1", "runner-1", "github.com")
	server.service.hostAuthenticationTokens["git-auth-unlinked"] = gitAuthenticationQueryToken("git-auth-unlinked", "service-account-1", v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT, "", "runner-1", "github.com")
	server.service.hostAuthenticationTokens["git-auth-non-pat"] = gitAuthenticationQueryToken("git-auth-non-pat", "service-account-2", v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT, "scm-2", "runner-2", "gitlab.com")
	return server
}

func gitAuthenticationQueryToken(id string, subjectID string, principal v1.Principal, source v1.HostAuthenticationTokenSource, integrationID string, runnerID string, host string) *v1.HostAuthenticationToken {
	return &v1.HostAuthenticationToken{
		Id:            id,
		RunnerId:      runnerID,
		Host:          host,
		Source:        source,
		IntegrationId: integrationID,
		Scopes:        []string{"repo"},
		ExpiresAt:     timestamppb.New(time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)),
		Subject:       &v1.Subject{Id: subjectID, Principal: principal},
	}
}

func gitAuthenticationLimitedQueryConfig() string {
	return `
list "ona_git_authentication" "all" {
  provider         = ona
  include_resource = true
  limit            = 1
}
`
}

func gitAuthenticationQueryConfig(config string) string {
	config = strings.TrimSpace(config)
	if config == "" {
		return `
list "ona_git_authentication" "all" {
  provider         = ona
  include_resource = true
}
`
	}
	return fmt.Sprintf(`
list "ona_git_authentication" "all" {
  provider         = ona
  include_resource = true

  config {
    %s
  }
}
`, config)
}

func expectedGitAuthenticationQueryResult() gitAuthenticationQueryResult {
	return gitAuthenticationQueryResult{
		Address:                      "list.ona_git_authentication.all",
		DisplayName:                  "github.com (service-account-1)",
		ID:                           "git-auth-1",
		ServiceAccountID:             "service-account-1",
		SCMIntegrationID:             "scm-1",
		RunnerID:                     "runner-1",
		Host:                         "github.com",
		ExpiresAt:                    "2030-01-02T03:04:05Z",
		Scopes:                       []string{"repo"},
		PersonalAccessToken:          nil,
		PersonalAccessTokenVersion:   nil,
		GeneratedConfigHasPAT:        false,
		GeneratedConfigHasPATVersion: false,
	}
}

type gitAuthenticationQueryResult struct {
	Address                      string
	DisplayName                  string
	ID                           string
	ServiceAccountID             string
	SCMIntegrationID             string
	RunnerID                     string
	Host                         string
	ExpiresAt                    string
	Scopes                       []string
	PersonalAccessToken          any
	PersonalAccessTokenVersion   any
	GeneratedConfigHasPAT        bool
	GeneratedConfigHasPATVersion bool
}

type expectGitAuthenticationQueryResults struct {
	Expected []gitAuthenticationQueryResult
}

func (e expectGitAuthenticationQueryResults) CheckQuery(_ context.Context, req querycheck.CheckQueryRequest, resp *querycheck.CheckQueryResponse) {
	got := make([]gitAuthenticationQueryResult, 0, len(req.Query))
	for _, result := range req.Query {
		got = append(got, gitAuthenticationQueryResult{
			Address:                      result.Address,
			DisplayName:                  result.DisplayName,
			ID:                           stringMapValue(result.Identity, "id"),
			ServiceAccountID:             stringMapValue(result.ResourceObject, "service_account_id"),
			SCMIntegrationID:             stringMapValue(result.ResourceObject, "scm_integration_id"),
			RunnerID:                     stringMapValue(result.ResourceObject, "runner_id"),
			Host:                         stringMapValue(result.ResourceObject, "host"),
			ExpiresAt:                    stringMapValue(result.ResourceObject, "expires_at"),
			Scopes:                       stringSliceMapValue(result.ResourceObject, "scopes"),
			PersonalAccessToken:          result.ResourceObject["personal_access_token"],
			PersonalAccessTokenVersion:   result.ResourceObject["personal_access_token_version"],
			GeneratedConfigHasPAT:        strings.Contains(result.Config, "personal_access_token ="),
			GeneratedConfigHasPATVersion: strings.Contains(result.Config, "personal_access_token_version"),
		})
	}
	sort.Slice(got, func(i, j int) bool { return got[i].ID < got[j].ID })
	if diff := cmp.Diff(e.Expected, got); diff != "" {
		resp.Error = fmt.Errorf("Git authentication query results mismatch (-want +got):\n%s", diff)
	}
}

func stringSliceMapValue(values map[string]any, key string) []string {
	value := values[key]
	switch value := value.(type) {
	case []string:
		return value
	case []any:
		result := make([]string, 0, len(value))
		for _, element := range value {
			stringElement, ok := element.(string)
			if !ok {
				return nil
			}
			result = append(result, stringElement)
		}
		return result
	default:
		return nil
	}
}
