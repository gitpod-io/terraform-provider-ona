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
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAccSecurityPolicyQuery(t *testing.T) {
	server := newSecurityPolicyQueryAPIServer(t)
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: securityPolicyQueryConfig(""),
		QueryResultChecks: []querycheck.QueryResultCheck{
			expectSecurityPolicyQueryResults{
				Expected: []securityPolicyQueryResult{
					expectedSecurityPolicyQueryResult("policy-1", "org-1", "port-controls"),
					expectedSecurityPolicyQueryResult("policy-2", "org-1", "file-controls"),
				},
			},
		},
	}))
}

func TestAccSecurityPolicyQueryFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Config   string
		Expected []securityPolicyQueryResult
	}{
		{
			Name: "organization_id",
			Config: securityPolicyQueryConfig(`
organization_id = "org-2"
`),
			Expected: []securityPolicyQueryResult{
				expectedSecurityPolicyQueryResult("policy-3", "org-2", "network-controls"),
			},
		},
		{
			Name: "search",
			Config: securityPolicyQueryConfig(`
search = "port"
`),
			Expected: []securityPolicyQueryResult{
				expectedSecurityPolicyQueryResult("policy-1", "org-1", "port-controls"),
			},
		},
		{
			Name: "security_policy_ids",
			Config: securityPolicyQueryConfig(`
security_policy_ids = ["policy-2"]
`),
			Expected: []securityPolicyQueryResult{
				expectedSecurityPolicyQueryResult("policy-2", "org-1", "file-controls"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			server := newSecurityPolicyQueryAPIServer(t)
			t.Cleanup(server.Close)

			testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
				Query:  true,
				Config: tc.Config,
				QueryResultChecks: []querycheck.QueryResultCheck{
					expectSecurityPolicyQueryResults{
						Expected: tc.Expected,
					},
				},
			}))
		})
	}
}

func TestAccSecurityPolicyQueryRejectsTooManyIDs(t *testing.T) {
	server := newSecurityPolicyQueryAPIServer(t)
	t.Cleanup(server.Close)

	ids := make([]string, 26)
	for i := range ids {
		ids[i] = fmt.Sprintf("%q", fmt.Sprintf("policy-%d", i+1))
	}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query: true,
		Config: securityPolicyQueryConfig(fmt.Sprintf(`
security_policy_ids = [%s]
`, strings.Join(ids, ", "))),
		ExpectError: regexp.MustCompile("Too Many Security Policy IDs"),
	}))
}

func newSecurityPolicyQueryAPIServer(t *testing.T) *policyAPIServer {
	t.Helper()

	server := newPolicyAPIServer(t)
	now := timestamppb.New(time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC))
	server.security.policies["policy-1"] = newTestSecurityPolicy("policy-1", "org-1", "port-controls", now)
	server.security.policies["policy-2"] = newTestSecurityPolicy("policy-2", "org-1", "file-controls", now)
	server.security.policies["policy-3"] = newTestSecurityPolicy("policy-3", "org-2", "network-controls", now)
	return server
}

func newTestSecurityPolicy(id, organizationID, name string, timestamp *timestamppb.Timestamp) *v1.SecurityPolicy {
	return &v1.SecurityPolicy{
		Id:             id,
		OrganizationId: organizationID,
		Metadata:       &v1.SecurityPolicy_Metadata{Name: name},
		Spec:           &v1.SecurityPolicy_Spec{},
		CreatedAt:      timestamp,
		UpdatedAt:      timestamp,
	}
}

func securityPolicyQueryConfig(config string) string {
	config = strings.TrimSpace(config)
	if config == "" {
		return `
list "ona_security_policy" "all" {
  provider         = ona
  include_resource = true
}
`
	}

	return fmt.Sprintf(`
list "ona_security_policy" "all" {
  provider         = ona
  include_resource = true

  config {
%s
  }
}
`, indentSecurityPolicyQueryConfig(config))
}

func indentSecurityPolicyQueryConfig(config string) string {
	lines := strings.Split(config, "\n")
	for i := range lines {
		lines[i] = "    " + strings.TrimSpace(lines[i])
	}
	return strings.Join(lines, "\n")
}

func expectedSecurityPolicyQueryResult(id, organizationID, name string) securityPolicyQueryResult {
	return securityPolicyQueryResult{
		Address:        "list.ona_security_policy.all",
		DisplayName:    name,
		ID:             id,
		OrganizationID: organizationID,
		Name:           name,
	}
}

type expectSecurityPolicyQueryResults struct {
	Expected []securityPolicyQueryResult
}

type securityPolicyQueryResult struct {
	Address        string
	DisplayName    string
	ID             string
	OrganizationID string
	Name           string
}

func (e expectSecurityPolicyQueryResults) CheckQuery(_ context.Context, req querycheck.CheckQueryRequest, resp *querycheck.CheckQueryResponse) {
	got := make([]securityPolicyQueryResult, 0, len(req.Query))
	for _, result := range req.Query {
		got = append(got, securityPolicyQueryResult{
			Address:        result.Address,
			DisplayName:    result.DisplayName,
			ID:             stringMapValue(result.Identity, "id"),
			OrganizationID: stringMapValue(result.ResourceObject, "organization_id"),
			Name:           stringMapValue(result.ResourceObject, "name"),
		})
	}
	sort.Slice(got, func(i, j int) bool {
		return got[i].ID < got[j].ID
	})

	if diff := cmp.Diff(e.Expected, got); diff != "" {
		resp.Error = fmt.Errorf("security policy query results mismatch (-want +got):\n%s", diff)
	}
}
