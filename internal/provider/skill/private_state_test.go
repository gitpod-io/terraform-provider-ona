// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

type fakePrivateState struct {
	data map[string][]byte
}

func (s *fakePrivateState) GetKey(_ context.Context, key string) ([]byte, diag.Diagnostics) {
	return s.data[key], nil
}

func (s *fakePrivateState) SetKey(_ context.Context, key string, value []byte) diag.Diagnostics {
	if s.data == nil {
		s.data = map[string][]byte{}
	}
	s.data[key] = value
	return nil
}

func TestPrivateOrganizationID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Errors []string
	}
	tests := []struct {
		Name          string
		Stored        []byte
		Authenticated string
		Expected      Expectation
	}{
		{Name: "absent_import_state_is_allowed", Authenticated: testOrganizationID},
		{Name: "matching_organization", Stored: []byte(`"` + testOrganizationID + `"`), Authenticated: testOrganizationID},
		{Name: "organization_change_is_rejected", Stored: []byte(`"` + testOrganizationID + `"`), Authenticated: testOtherOrgID, Expected: Expectation{Errors: []string{"Ona Skill Organization Changed"}}},
		{Name: "malformed_private_state_is_rejected", Stored: []byte(`{`), Authenticated: testOrganizationID, Expected: Expectation{Errors: []string{"Unable to Read Ona Skill Private State"}}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			state := &fakePrivateState{data: map[string][]byte{privateOrganizationIDKey: tc.Stored}}
			diags := verifyPrivateOrganizationID(t.Context(), state, tc.Authenticated)
			got := Expectation{Errors: diagnosticSummaries(diags)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("verifyPrivateOrganizationID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSetPrivateOrganizationID(t *testing.T) {
	t.Parallel()

	state := &fakePrivateState{}
	diags := setPrivateOrganizationID(t.Context(), state, testOrganizationID)
	if diff := cmp.Diff([]string(nil), diagnosticSummaries(diags)); diff != "" {
		t.Fatalf("setPrivateOrganizationID() diagnostics mismatch (-want +got):\n%s", diff)
	}
	verification := verifyPrivateOrganizationID(t.Context(), state, testOrganizationID)
	if diff := cmp.Diff([]string(nil), diagnosticSummaries(verification)); diff != "" {
		t.Errorf("stored private organization mismatch (-want +got):\n%s", diff)
	}
}
