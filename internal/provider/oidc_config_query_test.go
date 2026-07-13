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

type fakeOIDCConfigService struct {
	v1connect.UnimplementedOrganizationServiceHandler
}

func (s *fakeOIDCConfigService) GetOIDCConfig(ctx context.Context, req *connect.Request[v1.GetOIDCConfigRequest]) (*connect.Response[v1.GetOIDCConfigResponse], error) {
	return connect.NewResponse(&v1.GetOIDCConfigResponse{OidcConfig: &v1.OIDCConfig{Version: &v1.OIDCConfig_V3{V3: &v1.OIDCConfigV3{ExtraSubFields: []string{"project_id"}}}}}), nil
}

func TestAccOIDCConfigQuery(t *testing.T) {
	service := &fakeOIDCConfigService{}
	path, handler := v1connect.NewOrganizationServiceHandler(service)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(http.StripPrefix("/api", mux))
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query: true, Config: oidcConfigQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_oidc_config.all", 1),
			querycheck.ExpectIdentity("ona_oidc_config.all", map[string]knownvalue.Check{"organization_id": knownvalue.StringExact("org-1")}),
			querycheck.ExpectResourceKnownValues("ona_oidc_config.all", queryfilter.ByDisplayName(knownvalue.StringExact("org-1")), []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact("org-1")},
				{Path: tfjsonpath.New("version"), KnownValue: knownvalue.StringExact("v3")},
			}),
		},
	}))
}

func oidcConfigQueryConfig() string {
	return `
list "ona_oidc_config" "all" {
  provider         = ona
  include_resource = true
  config { organization_id = "org-1" }
}
`
}
