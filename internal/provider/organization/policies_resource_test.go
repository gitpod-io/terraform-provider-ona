// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestPoliciesBaseline(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Policies *v1.OrganizationPolicies
		Errors   []string
	}

	tests := []struct {
		Name     string
		HasState bool
		Stored   []byte
		Policies *v1.OrganizationPolicies
		Store    bool
		Read     bool
		Expected Expectation
	}{
		{
			Name:     "round_trip",
			HasState: true,
			Policies: &v1.OrganizationPolicies{
				OrganizationId:         "org-1",
				MembersRequireProjects: true,
				AgentPolicy: &v1.AgentPolicy{
					CommandDenyList:  []string{"rm -rf /"},
					GoalModeDisabled: true,
				},
				SecurityAgentPolicy: &v1.SecurityAgentPolicy{},
			},
			Store: true,
			Read:  true,
			Expected: Expectation{
				Policies: &v1.OrganizationPolicies{
					OrganizationId:         "org-1",
					MembersRequireProjects: true,
					AgentPolicy: &v1.AgentPolicy{
						CommandDenyList: []string{"rm -rf /"},
					},
				},
			},
		},
		{
			Name:     "rejects_missing_private_state",
			Read:     true,
			Expected: Expectation{Errors: []string{"Unable to Restore Organization Policy Baseline"}},
		},
		{
			Name:     "rejects_missing_private_state_on_store",
			Policies: &v1.OrganizationPolicies{OrganizationId: "org-1"},
			Store:    true,
			Expected: Expectation{Errors: []string{"Unable to Store Organization Policy Baseline"}},
		},
		{
			Name:     "rejects_missing_baseline",
			HasState: true,
			Read:     true,
			Expected: Expectation{Errors: []string{"Unable to Restore Organization Policy Baseline"}},
		},
		{
			Name:     "rejects_invalid_baseline",
			HasState: true,
			Stored:   []byte("{"),
			Read:     true,
			Expected: Expectation{Errors: []string{"Unable to Restore Organization Policy Baseline"}},
		},
		{
			Name:     "rejects_empty_policies",
			HasState: true,
			Store:    true,
			Expected: Expectation{Errors: []string{"Unable to Store Organization Policy Baseline"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var state privateState
			if tc.HasState {
				state = &fakePrivateState{data: map[string][]byte{}}
				if len(tc.Stored) > 0 {
					state.SetKey(t.Context(), policiesBaselinePrivateKey, tc.Stored)
				}
			}
			var diags diag.Diagnostics
			if tc.Store {
				diags.Append(setPoliciesBaseline(t.Context(), state, tc.Policies)...)
			}
			var policies *v1.OrganizationPolicies
			if tc.Read {
				var readDiags diag.Diagnostics
				policies, readDiags = policiesBaseline(t.Context(), state)
				diags.Append(readDiags...)
			}
			got := Expectation{Policies: policies, Errors: diagSummaries(diags)}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform(), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("organization policy baseline mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
