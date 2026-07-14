// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1/v1connect"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

type fakeSSOQueryService struct {
	v1connect.UnimplementedOrganizationServiceHandler
}

func (s *fakeSSOQueryService) ListSSOConfigurations(ctx context.Context, req *connect.Request[v1.ListSSOConfigurationsRequest]) (*connect.Response[v1.ListSSOConfigurationsResponse], error) {
	return connect.NewResponse(&v1.ListSSOConfigurationsResponse{SsoConfigurations: []*v1.SSOConfiguration{
		{Id: "sso-custom", OrganizationId: req.Msg.GetOrganizationId(), ClientId: "client-id", IssuerUrl: "https://idp.example.com", DisplayName: "Example IdP", ProviderType: v1.SSOConfiguration_PROVIDER_TYPE_CUSTOM, State: v1.SSOConfigurationState_SSO_CONFIGURATION_STATE_ACTIVE},
		{Id: "sso-builtin", OrganizationId: req.Msg.GetOrganizationId(), ProviderType: v1.SSOConfiguration_PROVIDER_TYPE_BUILTIN},
	}}), nil
}

func TestAccSSOConfigurationQuery(t *testing.T) {
	path, handler := v1connect.NewOrganizationServiceHandler(&fakeSSOQueryService{})
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	t.Cleanup(server.Close)
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: ssoConfigurationQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_sso_configuration.all", 1),
		querycheck.ExpectIdentity("ona_sso_configuration.all", map[string]knownvalue.Check{"id": knownvalue.StringExact("sso-custom")}),
		querycheck.ExpectResourceKnownValues("ona_sso_configuration.all", queryfilter.ByDisplayName(knownvalue.StringExact("Example IdP")), []querycheck.KnownValueCheck{
			{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("sso-custom")},
			{Path: tfjsonpath.New("client_id"), KnownValue: knownvalue.StringExact("client-id")},
			{Path: tfjsonpath.New("issuer_url"), KnownValue: knownvalue.StringExact("https://idp.example.com")},
		}),
	}}))
}

func ssoConfigurationQueryConfig() string {
	return `
list "ona_sso_configuration" "all" {
  provider         = ona
  include_resource = true
  config { organization_id = "org-1" }
}
`
}
