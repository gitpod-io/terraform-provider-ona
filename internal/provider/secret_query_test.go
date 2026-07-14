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

func TestAccSecretQuery(t *testing.T) {
	server := newSecretAPIServer(t)
	t.Cleanup(server.Close)
	id := secretTestSecretID(1)
	server.service.secrets[id] = &v1.Secret{Id: id, Name: "THIRD_PARTY_API_KEY", Scope: &v1.SecretScope{Scope: &v1.SecretScope_OrganizationId{OrganizationId: secretTestOrgID}}, Mount: &v1.Secret_EnvironmentVariable{EnvironmentVariable: true}}
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: secretQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_secret.all", 1),
		querycheck.ExpectIdentity("ona_secret.all", map[string]knownvalue.Check{"id": knownvalue.StringExact(id), "scope": knownvalue.StringExact("organization"), "organization_id": knownvalue.StringExact(secretTestOrgID), "project_id": knownvalue.Null(), "user_id": knownvalue.Null(), "service_account_id": knownvalue.Null()}),
		querycheck.ExpectResourceKnownValues("ona_secret.all", queryfilter.ByDisplayName(knownvalue.StringExact("THIRD_PARTY_API_KEY")), []querycheck.KnownValueCheck{
			{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(id)}, {Path: tfjsonpath.New("scope"), KnownValue: knownvalue.StringExact("organization")}, {Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact("THIRD_PARTY_API_KEY")}, {Path: tfjsonpath.New("value"), KnownValue: knownvalue.Null()},
		}),
	}}))
	if server.service.getSecretValueCalls() != 0 {
		t.Fatal("Query called GetSecretValue")
	}
}

func secretQueryConfig() string {
	return `
list "ona_secret" "all" {
  provider = ona
  include_resource = true
  config {
    scope = "organization"
    organization_id = "01980ed3-a090-7b5b-a74c-9bf5d8cfe500"
  }
}
`
}
