// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccGitHubAppQuery(t *testing.T) {
	t.Parallel()
	server, service := newGitHubAppAPIServer(t)
	sharedID := "01980ed3-a090-7b5b-a74c-9bf5d8cfe500"
	service.integrations[sharedID] = githubAppRemote(sharedID, false)
	service.integrations[githubAppIntegrationID] = githubAppRemote(githubAppIntegrationID, true)
	service.integrations[githubAppIntegrationID].Auth.ProprietaryApp.PrivateKey = "private-must-not-appear"
	resource.UnitTest(t, QueryTestCase(server.URL, resource.TestStep{Query: true, Config: `
list "ona_github_app_integration" "all" {
  provider = ona
  include_resource = true
  limit = 1
  config {}
}
`, QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_github_app_integration.all", 1),
		querycheck.ExpectIdentity("ona_github_app_integration.all", map[string]knownvalue.Check{"integration_id": knownvalue.StringExact(githubAppIntegrationID)}),
		querycheck.ExpectResourceKnownValues("ona_github_app_integration.all", queryfilter.ByDisplayName(knownvalue.StringExact("owned_app")), []querycheck.KnownValueCheck{
			{Path: tfjsonpath.New("app_id"), KnownValue: knownvalue.StringExact("123")}, {Path: tfjsonpath.New("credentials"), KnownValue: knownvalue.Null()}, {Path: tfjsonpath.New("credentials_version"), KnownValue: knownvalue.Null()}, {Path: tfjsonpath.New("installation"), KnownValue: knownvalue.Null()},
		}), githubAppQuerySecretsCheck{},
	}}))
	type Expectation struct{ Creates, Rotations, Validation int }
	service.mu.Lock()
	got := Expectation{service.creates, service.rotations, service.tokenCalls}
	service.mu.Unlock()
	if diff := cmp.Diff(Expectation{}, got); diff != "" {
		t.Errorf("query mutation mismatch (-want +got):\n%s", diff)
	}
}

func TestAccGitHubAppMetadataClassification(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		QueryCount  int
		ImportError string
	}
	custom := Expectation{QueryCount: 1}
	shared := Expectation{ImportError: "Not an Organization-Owned GitHub App"}
	tests := []struct {
		Name     string
		Mutate   func(*v1.Integration)
		Expected Expectation
	}{
		{Name: "all_equal_with_local_credentials", Mutate: func(i *v1.Integration) { i.Auth.ProprietaryApp.PrivateKey = "private-must-not-appear" }, Expected: shared},
		{Name: "app_id_only", Mutate: func(i *v1.Integration) { i.Auth.ProprietaryApp.AppId = "123" }, Expected: custom},
		{Name: "app_slug_only", Mutate: func(i *v1.Integration) { i.Auth.ProprietaryApp.AppSlug = "owned-app" }, Expected: custom},
		{Name: "client_id_only", Mutate: func(i *v1.Integration) { i.Auth.ProprietaryApp.ClientId = "client-id" }, Expected: custom},
		{Name: "all_differ_pending", Mutate: func(i *v1.Integration) { i.Auth = githubAppRemote(i.Id, true).Auth }, Expected: custom},
		{Name: "disabled", Mutate: func(i *v1.Integration) { i.Auth = githubAppRemote(i.Id, true).Auth; i.Enabled = false }, Expected: custom},
		{Name: "different_definition", Mutate: func(i *v1.Integration) {
			i.Auth = githubAppRemote(i.Id, true).Auth
			i.IntegrationDefinitionId = "other-definition"
		}, Expected: shared},
		{Name: "host_without_definition", Mutate: func(i *v1.Integration) {
			i.Auth = githubAppRemote(i.Id, true).Auth
			i.IntegrationDefinitionId = ""
			i.Host = "github.com"
		}, Expected: shared},
		{Name: "missing_app", Mutate: func(i *v1.Integration) { i.Auth.ProprietaryApp = nil }, Expected: shared},
		{Name: "missing_auth", Mutate: func(i *v1.Integration) { i.Auth = nil }, Expected: shared},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			server, service := newGitHubAppAPIServer(t)
			sharedID := "01980ed3-a090-7b5b-a74c-9bf5d8cfe500"
			service.integrations[sharedID] = githubAppRemote(sharedID, false)
			remote := githubAppRemote(githubAppIntegrationID, false)
			tc.Mutate(remote)
			service.integrations[githubAppIntegrationID] = remote
			checks := []querycheck.QueryResultCheck{querycheck.ExpectLength("ona_github_app_integration.all", tc.Expected.QueryCount), githubAppQuerySecretsCheck{}}
			appID := remote.GetAuth().GetProprietaryApp().GetAppId()
			if appID == "" {
				appID = "456"
			}
			if tc.Expected.QueryCount == 1 {
				checks = append(checks,
					querycheck.ExpectIdentity("ona_github_app_integration.all", map[string]knownvalue.Check{"integration_id": knownvalue.StringExact(githubAppIntegrationID)}),
					querycheck.ExpectResourceKnownValues("ona_github_app_integration.all", queryfilter.ByResourceIdentity(map[string]knownvalue.Check{"integration_id": knownvalue.StringExact(githubAppIntegrationID)}), []querycheck.KnownValueCheck{
						{Path: tfjsonpath.New("app_id"), KnownValue: knownvalue.StringExact(appID)},
						{Path: tfjsonpath.New("app_slug"), KnownValue: knownvalue.StringExact(remote.Auth.ProprietaryApp.AppSlug)},
						{Path: tfjsonpath.New("client_id"), KnownValue: knownvalue.StringExact(remote.Auth.ProprietaryApp.ClientId)},
						{Path: tfjsonpath.New("enabled"), KnownValue: knownvalue.Bool(remote.Enabled)},
						{Path: tfjsonpath.New("installation"), KnownValue: knownvalue.Null()},
						{Path: tfjsonpath.New("credentials"), KnownValue: knownvalue.Null()},
						{Path: tfjsonpath.New("credentials_version"), KnownValue: knownvalue.Null()},
					}),
				)
			}
			testCase := QueryTestCase(server.URL, resource.TestStep{Query: true, Config: `
list "ona_github_app_integration" "all" {
  provider = ona
  include_resource = true
  config {}
}
`, QueryResultChecks: checks})
			cfg := QueryProviderConfig(server.URL) + fmt.Sprintf(`
import {
  to = ona_github_app_integration.test
  identity = { integration_id = %q }
}
resource "ona_github_app_integration" "test" {
  app_id = %q
  enabled = %t
  credentials = {}
}
`, githubAppIntegrationID, appID, remote.Enabled)
			adopt := resource.TestStep{Config: cfg}
			if tc.Expected.ImportError != "" {
				adopt.ExpectError = regexp.MustCompile(tc.Expected.ImportError)
			} else {
				adopt.Check = resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(githubAppAddress, "id", githubAppIntegrationID),
					resource.TestCheckResourceAttr(githubAppAddress, "app_slug", remote.Auth.ProprietaryApp.AppSlug),
					resource.TestCheckNoResourceAttr(githubAppAddress, "credentials"),
					resource.TestCheckNoResourceAttr(githubAppAddress, "credentials_version"),
				)
			}
			testCase.Steps = append(testCase.Steps, adopt)
			if tc.Expected.ImportError == "" {
				testCase.Steps = append(testCase.Steps, resource.TestStep{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}})
			}
			resource.UnitTest(t, testCase)
			type Mutations struct{ Creates, Updates, Validation int }
			service.mu.Lock()
			got := Mutations{service.creates, service.updates, service.tokenCalls}
			service.mu.Unlock()
			if diff := cmp.Diff(Mutations{}, got); diff != "" {
				t.Errorf("discovery and adoption mutated the integration (-want +got):\n%s", diff)
			}
		})
	}
}

type githubAppQuerySecretsCheck struct{}

func (githubAppQuerySecretsCheck) CheckQuery(_ context.Context, req querycheck.CheckQueryRequest, resp *querycheck.CheckQueryResponse) {
	resp.Error = githubAppCheckSecrets(req.Query)
}
