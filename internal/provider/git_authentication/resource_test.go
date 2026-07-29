// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package gitauthentication

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSchemaRestrictsSubjectAndProtectsPAT(t *testing.T) {
	t.Parallel()

	var resp resource.SchemaResponse
	(&Resource{}).Schema(t.Context(), resource.SchemaRequest{}, &resp)
	pat, ok := resp.Schema.Attributes["personal_access_token"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("personal_access_token has type %T, want schema.StringAttribute", resp.Schema.Attributes["personal_access_token"])
	}
	serviceAccountID, ok := resp.Schema.Attributes["service_account_id"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("service_account_id has type %T, want schema.StringAttribute", resp.Schema.Attributes["service_account_id"])
	}
	_, hasUserID := resp.Schema.Attributes["user_id"]
	_, hasPrincipal := resp.Schema.Attributes["principal"]

	type Expectation struct {
		PATOptional              bool
		PATSensitive             bool
		PATWriteOnly             bool
		ServiceAccountIDRequired bool
		HasUserID                bool
		HasPrincipal             bool
	}
	want := Expectation{PATOptional: true, PATSensitive: true, PATWriteOnly: true, ServiceAccountIDRequired: true}
	got := Expectation{
		PATOptional:              pat.Optional,
		PATSensitive:             pat.Sensitive,
		PATWriteOnly:             pat.WriteOnly,
		ServiceAccountIDRequired: serviceAccountID.Required,
		HasUserID:                hasUserID,
		HasPrincipal:             hasPrincipal,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Schema() mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateHostAuthenticationTokenRequest(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Request *v1.CreateHostAuthenticationTokenRequest
		Err     string
	}
	tests := []struct {
		Name        string
		Model       Model
		Integration *v1.SCMIntegration
		PAT         types.String
		Expected    Expectation
	}{
		{
			Name: "maps_service_account_pat",
			Model: Model{
				ServiceAccountID: types.StringValue("service-account-1"),
				SCMIntegrationID: types.StringValue("scm-1"),
			},
			Integration: &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", Host: "github.com", Pat: true},
			PAT:         types.StringValue("secret-pat"),
			Expected: Expectation{Request: &v1.CreateHostAuthenticationTokenRequest{
				RunnerId:      "runner-1",
				Host:          "github.com",
				Token:         "secret-pat",
				Source:        v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT,
				IntegrationId: "scm-1",
				Subject:       &v1.Subject{Id: "service-account-1", Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT},
			}},
		},
		{
			Name:        "rejects_missing_service_account",
			Model:       Model{ServiceAccountID: types.StringNull(), SCMIntegrationID: types.StringValue("scm-1")},
			Integration: &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", Host: "github.com", Pat: true},
			PAT:         types.StringValue("secret-pat"),
			Expected:    Expectation{Err: "Missing Service Account ID"},
		},
		{
			Name:        "rejects_non_pat_integration",
			Model:       Model{ServiceAccountID: types.StringValue("service-account-1"), SCMIntegrationID: types.StringValue("scm-1")},
			Integration: &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", Host: "github.com"},
			PAT:         types.StringValue("secret-pat"),
			Expected:    Expectation{Err: "SCM Integration Does Not Support PAT Authentication"},
		},
		{
			Name:        "rejects_unknown_pat",
			Model:       Model{ServiceAccountID: types.StringValue("service-account-1"), SCMIntegrationID: types.StringValue("scm-1")},
			Integration: &v1.SCMIntegration{Id: "scm-1", RunnerId: "runner-1", Host: "github.com", Pat: true},
			PAT:         types.StringUnknown(),
			Expected:    Expectation{Err: "Missing Personal Access Token"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var got Expectation
			request, diags := createHostAuthenticationTokenRequest(tc.Model, tc.Integration, tc.PAT)
			if diags.HasError() {
				got.Err = diags[0].Summary()
			} else {
				got.Request = request
			}
			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("createHostAuthenticationTokenRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSecretVersionChanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Current  types.String
		Prior    types.String
		Expected bool
	}{
		{Name: "unknown_current", Current: types.StringUnknown(), Prior: types.StringValue("v1")},
		{Name: "unknown_prior", Current: types.StringValue("v1"), Prior: types.StringUnknown()},
		{Name: "both_null", Current: types.StringNull(), Prior: types.StringNull()},
		{Name: "null_to_known", Current: types.StringValue("v1"), Prior: types.StringNull(), Expected: true},
		{Name: "known_to_null", Current: types.StringNull(), Prior: types.StringValue("v1"), Expected: true},
		{Name: "equal_known", Current: types.StringValue("v1"), Prior: types.StringValue("v1")},
		{Name: "different_known", Current: types.StringValue("v2"), Prior: types.StringValue("v1"), Expected: true},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tc.Expected, secretVersionChanged(tc.Current, tc.Prior)); diff != "" {
				t.Errorf("secretVersionChanged() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetHostAuthenticationTokenAfterWrite(t *testing.T) {
	t.Parallel()

	type Input struct {
		Results   []*v1.HostAuthenticationToken
		GetErrAt  int
		WaitErrAt int
	}
	type Expectation struct {
		TokenID string
		Err     string
		Calls   int
		Waits   []time.Duration
	}
	tests := []struct {
		Name     string
		Input    Input
		Expected Expectation
	}{
		{
			Name:     "returns_immediate_read",
			Input:    Input{Results: []*v1.HostAuthenticationToken{{Id: "git-auth-1"}}},
			Expected: Expectation{TokenID: "git-auth-1", Calls: 1},
		},
		{
			Name:     "retries_not_found_until_visible",
			Input:    Input{Results: []*v1.HostAuthenticationToken{nil, nil, {Id: "git-auth-1"}}},
			Expected: Expectation{TokenID: "git-auth-1", Calls: 3, Waits: []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}},
		},
		{
			Name:     "returns_nil_after_bounded_attempts",
			Input:    Input{},
			Expected: Expectation{Calls: 4, Waits: []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}},
		},
		{
			Name:     "returns_get_error_without_retry",
			Input:    Input{GetErrAt: 1},
			Expected: Expectation{Err: "get failed", Calls: 1},
		},
		{
			Name:     "returns_wait_error",
			Input:    Input{WaitErrAt: 1},
			Expected: Expectation{Err: "wait before retrying host authentication token read: wait failed", Calls: 1, Waits: []time.Duration{100 * time.Millisecond}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			get := func(_ context.Context, _ string) (*v1.HostAuthenticationToken, error) {
				got.Calls++
				if tc.Input.GetErrAt == got.Calls {
					return nil, errors.New("get failed")
				}
				if got.Calls <= len(tc.Input.Results) {
					return tc.Input.Results[got.Calls-1], nil
				}
				return nil, nil
			}
			wait := func(_ context.Context, delay time.Duration) error {
				got.Waits = append(got.Waits, delay)
				if tc.Input.WaitErrAt == len(got.Waits) {
					return errors.New("wait failed")
				}
				return nil
			}

			token, err := getHostAuthenticationTokenAfterWrite(t.Context(), "git-auth-1", get, wait)
			if err != nil {
				got.Err = err.Error()
			} else if token != nil {
				got.TokenID = token.GetId()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("getHostAuthenticationTokenAfterWrite() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
