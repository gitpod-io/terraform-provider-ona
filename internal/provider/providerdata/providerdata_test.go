// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package providerdata

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/gitpod-sdk-go/v1/v1connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func TestConfigureClient(t *testing.T) {
	t.Parallel()

	client := managementclient.NewWithServices(managementclient.Services{})
	type configureFunc func(any, *managementclient.ManagementPlane, *diag.Diagnostics) *managementclient.ManagementPlane
	type Expectation struct {
		ConfiguredClient  bool
		DiagnosticSummary string
		DiagnosticDetail  string
	}
	tests := []struct {
		Name      string
		Configure configureFunc
		Input     any
		Current   *managementclient.ManagementPlane
		Expected  Expectation
	}{
		{Name: "resource_nil_data", Configure: ResourceClient},
		{Name: "resource_nil_data_preserves_current", Configure: ResourceClient, Current: client, Expected: Expectation{ConfiguredClient: true}},
		{Name: "resource_valid_data", Configure: ResourceClient, Input: &Data{Client: client}, Expected: Expectation{ConfiguredClient: true}},
		{Name: "resource_wrong_type", Configure: ResourceClient, Input: "invalid", Expected: Expectation{DiagnosticSummary: "Unexpected Resource Configure Type", DiagnosticDetail: "Expected *providerdata.Data, got: string. Please report this issue to the provider developers."}},
		{Name: "resource_wrong_type_preserves_current", Configure: ResourceClient, Input: "invalid", Current: client, Expected: Expectation{ConfiguredClient: true, DiagnosticSummary: "Unexpected Resource Configure Type", DiagnosticDetail: "Expected *providerdata.Data, got: string. Please report this issue to the provider developers."}},
		{Name: "data_source_nil_data", Configure: DataSourceClient},
		{Name: "data_source_nil_data_preserves_current", Configure: DataSourceClient, Current: client, Expected: Expectation{ConfiguredClient: true}},
		{Name: "data_source_valid_data", Configure: DataSourceClient, Input: &Data{Client: client}, Expected: Expectation{ConfiguredClient: true}},
		{Name: "data_source_wrong_type", Configure: DataSourceClient, Input: "invalid", Expected: Expectation{DiagnosticSummary: "Unexpected Data Source Configure Type", DiagnosticDetail: "Expected *providerdata.Data, got: string. Please report this issue to the provider developers."}},
		{Name: "data_source_wrong_type_preserves_current", Configure: DataSourceClient, Input: "invalid", Current: client, Expected: Expectation{ConfiguredClient: true, DiagnosticSummary: "Unexpected Data Source Configure Type", DiagnosticDetail: "Expected *providerdata.Data, got: string. Please report this issue to the provider developers."}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			configured := tc.Configure(tc.Input, tc.Current, &diags)
			got := Expectation{ConfiguredClient: configured == client}
			if len(diags) > 0 {
				got.DiagnosticSummary = diags[0].Summary()
				got.DiagnosticDetail = diags[0].Detail()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("configure client mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEphemeralResourceData(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Configured        bool
		APIBaseURL        string
		UserAgent         string
		DiagnosticSummary string
		DiagnosticDetail  string
	}
	tests := []struct {
		Name     string
		Input    any
		Current  *Data
		Expected Expectation
	}{
		{Name: "nil_data"},
		{Name: "nil_data_preserves_current", Current: &Data{APIBaseURL: "https://current.example.com", UserAgent: "current"}, Expected: Expectation{Configured: true, APIBaseURL: "https://current.example.com", UserAgent: "current"}},
		{Name: "valid_data", Input: &Data{APIBaseURL: "https://example.com", UserAgent: "test"}, Expected: Expectation{Configured: true, APIBaseURL: "https://example.com", UserAgent: "test"}},
		{Name: "wrong_type", Input: "invalid", Expected: Expectation{DiagnosticSummary: "Unexpected Ephemeral Resource Configure Type", DiagnosticDetail: "Expected *providerdata.Data, got: string. Please report this issue to the provider developers."}},
		{Name: "wrong_type_preserves_current", Input: "invalid", Current: &Data{APIBaseURL: "https://current.example.com", UserAgent: "current"}, Expected: Expectation{Configured: true, APIBaseURL: "https://current.example.com", UserAgent: "current", DiagnosticSummary: "Unexpected Ephemeral Resource Configure Type", DiagnosticDetail: "Expected *providerdata.Data, got: string. Please report this issue to the provider developers."}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			data := EphemeralResourceData(tc.Input, tc.Current, &diags)
			got := Expectation{Configured: data != nil}
			if data != nil {
				got.APIBaseURL = data.APIBaseURL
				got.UserAgent = data.UserAgent
			}
			if len(diags) > 0 {
				got.DiagnosticSummary = diags[0].Summary()
				got.DiagnosticDetail = diags[0].Detail()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("EphemeralResourceData() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRequireClient(t *testing.T) {
	t.Parallel()

	client := managementclient.NewWithServices(managementclient.Services{})
	type requireFunc func(*managementclient.ManagementPlane, *diag.Diagnostics) bool
	type Expectation struct {
		Available         bool
		DiagnosticSummary string
		DiagnosticDetail  string
	}
	tests := []struct {
		Name     string
		Client   *managementclient.ManagementPlane
		Require  requireFunc
		Expected Expectation
	}{
		{Name: "resource_available", Client: client, Require: func(client *managementclient.ManagementPlane, diags *diag.Diagnostics) bool {
			return RequireResourceClient(client, diags, "creating", "ona_widget")
		}, Expected: Expectation{Available: true}},
		{Name: "resource_missing", Require: func(client *managementclient.ManagementPlane, diags *diag.Diagnostics) bool {
			return RequireResourceClient(client, diags, "creating", "ona_widget")
		}, Expected: Expectation{DiagnosticSummary: "Ona API Client Is Not Configured", DiagnosticDetail: "Set the provider token argument or ONA_TOKEN before creating ona_widget resources."}},
		{Name: "data_source_available", Client: client, Require: func(client *managementclient.ManagementPlane, diags *diag.Diagnostics) bool {
			return RequireDataSourceClient(client, diags, "ona_widgets")
		}, Expected: Expectation{Available: true}},
		{Name: "data_source_missing", Require: func(client *managementclient.ManagementPlane, diags *diag.Diagnostics) bool {
			return RequireDataSourceClient(client, diags, "ona_widgets")
		}, Expected: Expectation{DiagnosticSummary: "Ona API Client Is Not Configured", DiagnosticDetail: "Set the provider token argument or ONA_TOKEN before reading ona_widgets data sources."}},
		{Name: "ephemeral_resource_available", Client: client, Require: func(client *managementclient.ManagementPlane, diags *diag.Diagnostics) bool {
			return RequireEphemeralResourceClient(client, diags, "ona_widget_token")
		}, Expected: Expectation{Available: true}},
		{Name: "ephemeral_resource_missing", Require: func(client *managementclient.ManagementPlane, diags *diag.Diagnostics) bool {
			return RequireEphemeralResourceClient(client, diags, "ona_widget_token")
		}, Expected: Expectation{DiagnosticSummary: "Ona API Client Is Not Configured", DiagnosticDetail: "Set the provider token argument or ONA_TOKEN before opening ona_widget_token ephemeral resources."}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			got := Expectation{Available: tc.Require(tc.Client, &diags)}
			if len(diags) > 0 {
				got.DiagnosticSummary = diags[0].Summary()
				got.DiagnosticDetail = diags[0].Detail()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("require client mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAuthenticatedOrganizationID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		OrganizationID string
		Err            string
		Calls          int
	}
	tests := []struct {
		Name     string
		Response *connect.Response[v1.GetAuthenticatedIdentityResponse]
		Err      error
		Expected Expectation
	}{
		{Name: "returns_organization_id", Response: connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{OrganizationId: "organization-id"}), Expected: Expectation{OrganizationID: "organization-id", Calls: 1}},
		{Name: "propagates_api_error", Err: connect.NewError(connect.CodeUnauthenticated, errors.New("bad token")), Expected: Expectation{Err: "get authenticated identity: unauthenticated: bad token", Calls: 1}},
		{Name: "rejects_nil_response", Expected: Expectation{Err: "get authenticated identity: API returned an empty response", Calls: 1}},
		{Name: "rejects_nil_message", Response: &connect.Response[v1.GetAuthenticatedIdentityResponse]{}, Expected: Expectation{Err: "get authenticated identity: API returned an empty response", Calls: 1}},
		{Name: "rejects_missing_organization_id", Response: connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{}), Expected: Expectation{Err: "authenticated identity did not include an organization ID", Calls: 1}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			service := &fakeIdentityService{response: tc.Response, err: tc.Err}
			client := managementclient.NewWithServices(managementclient.Services{IdentityService: service})
			var got Expectation
			organizationID, err := AuthenticatedOrganizationID(t.Context(), client)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.OrganizationID = organizationID
			}
			got.Calls = service.calls
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("AuthenticatedOrganizationID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type fakeIdentityService struct {
	v1connect.IdentityServiceClient
	response *connect.Response[v1.GetAuthenticatedIdentityResponse]
	err      error
	calls    int
}

func (f *fakeIdentityService) GetAuthenticatedIdentity(context.Context, *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	f.calls++
	return f.response, f.err
}
