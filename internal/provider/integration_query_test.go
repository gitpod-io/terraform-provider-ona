// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccIntegrationQuery(t *testing.T) {
	t.Parallel()

	server := newIntegrationAPIServer(t)
	t.Cleanup(server.Close)
	definition := testIntegrationDefinition("definition-1", "Example Integration", "example.com")
	server.service.seedDefinition(definition)
	server.service.seedIntegration(&v1.Integration{
		Id:             "integration-query-1",
		OrganizationId: "organization-1",
		Name:           "Example Integration",
		Host:           "custom.example.com",
		Enabled:        true,
		Capabilities:   &v1.IntegrationCapabilities{Mcp: &v1.IntegrationMCPCapability{Url: "https://custom.example.com/mcp"}},
		Auth: &v1.IntegrationAuthentication{Oauth: &v1.IntegrationOAuthConfig{
			ClientId:     "custom-client",
			ClientSecret: "must-not-appear",
		}},
		Categories: []v1.IntegrationCategory{v1.IntegrationCategory_INTEGRATION_CATEGORY_MCP},
	})
	server.service.seedIntegration(&v1.Integration{
		Id:                      "integration-query-2",
		OrganizationId:          "organization-1",
		IntegrationDefinitionId: definition.GetId(),
		RunnerId:                "runner-1",
		Enabled:                 false,
		ExternalInstallation: &v1.IntegrationExternalInstallation{
			Id:          "installation-1",
			AccountName: "example-org",
			AccountType: "Organization",
		},
	})
	server.service.listPageSize = 1

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: integrationQueryConfig(true, 0),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_integration.all", 2),
			querycheck.ExpectIdentity("ona_integration.all", integrationQueryIdentity("integration-query-1")),
			querycheck.ExpectIdentity("ona_integration.all", integrationQueryIdentity("integration-query-2")),
			querycheck.ExpectResourceDisplayName("ona_integration.all", queryfilter.ByResourceIdentity(integrationQueryIdentity("integration-query-1")), knownvalue.StringExact("example_integration")),
			querycheck.ExpectResourceDisplayName("ona_integration.all", queryfilter.ByResourceIdentity(integrationQueryIdentity("integration-query-2")), knownvalue.StringExact("example_integration_2")),
			querycheck.ExpectResourceKnownValues("ona_integration.all", queryfilter.ByResourceIdentity(integrationQueryIdentity("integration-query-1")), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("integration-query-1")},
				{Path: tfjsonpath.New("organization_id"), KnownValue: knownvalue.StringExact("organization-1")},
				{Path: tfjsonpath.New("enabled"), KnownValue: knownvalue.Bool(true)},
				{Path: tfjsonpath.New("credentials"), KnownValue: knownvalue.Null()},
				{Path: tfjsonpath.New("auth").AtMapKey("oauth").AtMapKey("client_secret_version"), KnownValue: knownvalue.Null()},
			}),
			querycheck.ExpectResourceKnownValues("ona_integration.all", queryfilter.ByResourceIdentity(integrationQueryIdentity("integration-query-2")), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("integration_definition_id"), KnownValue: knownvalue.StringExact("definition-1")},
				{Path: tfjsonpath.New("runner_id"), KnownValue: knownvalue.StringExact("runner-1")},
				{Path: tfjsonpath.New("external_installation").AtMapKey("id"), KnownValue: knownvalue.StringExact("installation-1")},
			}),
			integrationQueryOmitsSecrets{},
		},
	}))

	calls, pageSizes := server.service.integrationListStats()
	got := integrationListExpectation{Calls: calls, PageSizes: pageSizes}
	expected := integrationListExpectation{Calls: 2, PageSizes: []int32{100, 99}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ListIntegrations requests mismatch (-want +got):\n%s", diff)
	}
}

func TestAccIntegrationQueryLimit(t *testing.T) {
	t.Parallel()

	server := newIntegrationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedIntegration(&v1.Integration{Id: "integration-query-1", OrganizationId: "organization-1", Name: "First"})
	server.service.seedIntegration(&v1.Integration{Id: "integration-query-2", OrganizationId: "organization-1", Name: "Second"})

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: integrationQueryConfig(false, 1),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_integration.all", 1),
			querycheck.ExpectIdentity("ona_integration.all", integrationQueryIdentity("integration-query-1")),
		},
	}))

	calls, pageSizes := server.service.integrationListStats()
	got := integrationListExpectation{Calls: calls, PageSizes: pageSizes}
	expected := integrationListExpectation{Calls: 1, PageSizes: []int32{1}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ListIntegrations requests mismatch (-want +got):\n%s", diff)
	}
}

type integrationListExpectation struct {
	Calls     int
	PageSizes []int32
}

func TestAccIntegrationQueryReportsListError(t *testing.T) {
	t.Parallel()

	server := newIntegrationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.listErr = connect.NewError(connect.CodePermissionDenied, errors.New("integration read denied"))

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      integrationQueryConfig(false, 0),
		ExpectError: regexp.MustCompile("Unable to List Ona Integrations"),
	}))
}

func TestAccIntegrationQueryRejectsMissingID(t *testing.T) {
	t.Parallel()

	server := newIntegrationAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seedIntegration(&v1.Integration{Name: "Missing ID"})

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:       true,
		Config:      integrationQueryConfig(false, 0),
		ExpectError: regexp.MustCompile("integration without an ID"),
	}))
}

func integrationQueryIdentity(id string) map[string]knownvalue.Check {
	return map[string]knownvalue.Check{"integration_id": knownvalue.StringExact(id)}
}

func integrationQueryConfig(includeResource bool, limit int) string {
	includeResourceLine := ""
	if includeResource {
		includeResourceLine = "  include_resource = true\n"
	}
	limitLine := ""
	if limit > 0 {
		limitLine = fmt.Sprintf("  limit            = %d\n", limit)
	}
	return `
list "ona_integration" "all" {
  provider = ona
` + includeResourceLine + limitLine + `}
`
}

type integrationQueryOmitsSecrets struct{}

func (integrationQueryOmitsSecrets) CheckQuery(_ context.Context, req querycheck.CheckQueryRequest, resp *querycheck.CheckQueryResponse) {
	for _, result := range req.Query {
		if result.ResourceObject["credentials"] != nil {
			resp.Error = fmt.Errorf("integration %q returned credentials in the resource object", result.DisplayName)
			return
		}
		for _, field := range []string{"credentials", "client_secret_version", "webhook_secret_version", "private_key_version", "api_key_version"} {
			if strings.Contains(result.Config, field) {
				resp.Error = fmt.Errorf("integration %q generated configuration containing %q", result.DisplayName, field)
				return
			}
		}
	}
}
