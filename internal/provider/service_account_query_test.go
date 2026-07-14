// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func (s *fakeServiceAccountService) ListServiceAccounts(ctx context.Context, req *connect.Request[v1.ListServiceAccountsRequest]) (*connect.Response[v1.ListServiceAccountsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var accounts []*v1.ServiceAccount
	for _, account := range s.accounts {
		accounts = append(accounts, cloneServiceAccount(account))
	}
	return connect.NewResponse(&v1.ListServiceAccountsResponse{ServiceAccounts: accounts}), nil
}

func TestAccServiceAccountQuery(t *testing.T) {
	server := newServiceAccountAPIServer(t)
	t.Cleanup(server.Close)
	server.service.seed(newTestServiceAccount(serviceAccountID1, "Terraform Automation", "Managed by Terraform"))
	suspendedAccount := newTestServiceAccount(serviceAccountID2, "Suspended Automation", "")
	suspendedAccount.Suspended = true
	server.service.seed(suspendedAccount)
	systemManagedAccount := newTestServiceAccount(serviceAccountID3, "System Automation", "")
	systemManagedAccount.SystemManaged = true
	server.service.seed(systemManagedAccount)
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: serviceAccountQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_service_account.all", 1),
		querycheck.ExpectIdentity("ona_service_account.all", map[string]knownvalue.Check{"service_account_id": knownvalue.StringExact(serviceAccountID1)}),
		querycheck.ExpectNoIdentity("ona_service_account.all", map[string]knownvalue.Check{"service_account_id": knownvalue.StringExact(serviceAccountID2)}),
		querycheck.ExpectNoIdentity("ona_service_account.all", map[string]knownvalue.Check{"service_account_id": knownvalue.StringExact(serviceAccountID3)}),
		querycheck.ExpectResourceKnownValues("ona_service_account.all", queryfilter.ByDisplayName(knownvalue.StringExact("Terraform Automation")), []querycheck.KnownValueCheck{
			{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(serviceAccountID1)},
			{Path: tfjsonpath.New("service_account_id"), KnownValue: knownvalue.StringExact(serviceAccountID1)},
			{Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact("Terraform Automation")},
		}),
	}}))
	server.service.mu.Lock()
	defer server.service.mu.Unlock()
	if len(server.service.accessTokenCalls) != 0 || len(server.service.serviceTokenCalls) != 0 {
		t.Fatal("Query called a token-issuing endpoint")
	}
}

func serviceAccountQueryConfig() string {
	return `
list "ona_service_account" "all" {
  provider = ona
  include_resource = true
}
`
}
