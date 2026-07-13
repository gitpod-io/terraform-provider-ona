// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAccSecurityPolicyQuery(t *testing.T) {
	server := newPolicyAPIServer(t)
	t.Cleanup(server.Close)
	now := timestamppb.New(time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC))
	server.security.policies["policy-1"] = &v1.SecurityPolicy{
		Id:             "policy-1",
		OrganizationId: "org-1",
		Metadata:       &v1.SecurityPolicy_Metadata{Name: "port-controls"},
		Spec:           &v1.SecurityPolicy_Spec{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	server.security.policies["policy-2"] = &v1.SecurityPolicy{
		Id:             "policy-2",
		OrganizationId: "org-1",
		Metadata:       &v1.SecurityPolicy_Metadata{Name: "file-controls"},
		Spec:           &v1.SecurityPolicy_Spec{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query:  true,
		Config: securityPolicyQueryConfig(),
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_security_policy.all", 1),
			querycheck.ExpectIdentity("ona_security_policy.all", map[string]knownvalue.Check{
				"id": knownvalue.StringExact("policy-1"),
			}),
			querycheck.ExpectResourceKnownValues(
				"ona_security_policy.all",
				queryfilter.ByDisplayName(knownvalue.StringExact("port-controls")),
				[]querycheck.KnownValueCheck{
					{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("policy-1")},
					{Path: tfjsonpath.New("organization_id"), KnownValue: knownvalue.StringExact("org-1")},
					{Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact("port-controls")},
				},
			),
		},
	}))
}

func securityPolicyQueryConfig() string {
	return `
list "ona_security_policy" "all" {
  provider         = ona
  include_resource = true

  config {
    organization_id    = "org-1"
    search             = "port"
    security_policy_ids = ["policy-1"]
  }
}
`
}
