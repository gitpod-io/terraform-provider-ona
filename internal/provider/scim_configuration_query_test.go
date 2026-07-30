// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeSCIMQueryService struct {
	v1connect.UnimplementedOrganizationServiceHandler
	v1connect.UnimplementedIdentityServiceHandler
}

func (s *fakeSCIMQueryService) GetAuthenticatedIdentity(ctx context.Context, req *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{OrganizationId: "org-1"}), nil
}

func (s *fakeSCIMQueryService) ListSCIMConfigurations(ctx context.Context, req *connect.Request[v1.ListSCIMConfigurationsRequest]) (*connect.Response[v1.ListSCIMConfigurationsResponse], error) {
	expires := timestamppb.New(time.Date(2027, 7, 14, 0, 0, 0, 0, time.UTC))
	return connect.NewResponse(&v1.ListSCIMConfigurationsResponse{ScimConfigurations: []*v1.SCIMConfiguration{{
		Id: "scim-1", OrganizationId: "org-1", SsoConfigurationId: "sso-1", Name: "Okta", Enabled: true, TokenExpiresAt: expires,
	}}}), nil
}

func TestAccSCIMConfigurationQuery(t *testing.T) {
	service := &fakeSCIMQueryService{}
	mux := http.NewServeMux()
	organizationPath, organizationHandler := v1connect.NewOrganizationServiceHandler(service)
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(service)
	mux.Handle(organizationPath, organizationHandler)
	mux.Handle(identityPath, identityHandler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	t.Cleanup(server.Close)
	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{Query: true, Config: scimConfigurationQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
		querycheck.ExpectLength("ona_scim_configuration.all", 1),
		querycheck.ExpectIdentity("ona_scim_configuration.all", map[string]knownvalue.Check{"id": knownvalue.StringExact("scim-1")}),
		querycheck.ExpectResourceKnownValues("ona_scim_configuration.all", queryfilter.ByDisplayName(knownvalue.StringExact("Okta")), []querycheck.KnownValueCheck{
			{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("scim-1")},
			{Path: tfjsonpath.New("sso_configuration_id"), KnownValue: knownvalue.StringExact("sso-1")},
			{Path: tfjsonpath.New("token_expires_at"), KnownValue: knownvalue.StringExact("2027-07-14T00:00:00Z")},
		}),
	}}))
}

func scimConfigurationQueryConfig() string {
	return `
list "ona_scim_configuration" "all" {
  provider         = ona
  include_resource = true
}
`
}
