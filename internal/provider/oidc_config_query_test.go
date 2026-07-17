// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"google.golang.org/protobuf/proto"
)

type fakeOIDCConfigService struct {
	v1connect.UnimplementedOrganizationServiceHandler
	v1connect.UnimplementedIdentityServiceHandler

	mu             sync.Mutex
	organizationID string
	oidcConfig     *v1.OIDCConfig
}

func newFakeOIDCConfigService(organizationID string) *fakeOIDCConfigService {
	return &fakeOIDCConfigService{organizationID: organizationID}
}

func newOIDCConfigAPIServer(t *testing.T, service *fakeOIDCConfigService) *httptest.Server {
	t.Helper()

	organizationPath, organizationHandler := v1connect.NewOrganizationServiceHandler(service)
	identityPath, identityHandler := v1connect.NewIdentityServiceHandler(service)

	mux := http.NewServeMux()
	mux.Handle(organizationPath, organizationHandler)
	mux.Handle(identityPath, identityHandler)

	return httptest.NewServer(http.StripPrefix("/api", mux))
}

func (s *fakeOIDCConfigService) GetAuthenticatedIdentity(ctx context.Context, req *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{
		Subject: &v1.Subject{
			Id:        "service-account-1",
			Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT,
		},
		OrganizationId: s.organizationID,
	}), nil
}

func (s *fakeOIDCConfigService) GetOIDCConfig(ctx context.Context, req *connect.Request[v1.GetOIDCConfigRequest]) (*connect.Response[v1.GetOIDCConfigResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.organizationID || s.oidcConfig == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("OIDC config not found"))
	}
	return connect.NewResponse(&v1.GetOIDCConfigResponse{OidcConfig: proto.CloneOf(s.oidcConfig)}), nil
}

func (s *fakeOIDCConfigService) UpdateOIDCConfig(ctx context.Context, req *connect.Request[v1.UpdateOIDCConfigRequest]) (*connect.Response[v1.UpdateOIDCConfigResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Msg.GetOrganizationId() != s.organizationID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("organization not found"))
	}
	s.oidcConfig = proto.CloneOf(req.Msg.GetOidcConfig())
	return connect.NewResponse(&v1.UpdateOIDCConfigResponse{OidcConfig: proto.CloneOf(s.oidcConfig)}), nil
}

func (s *fakeOIDCConfigService) seedOIDCConfig(config *v1.OIDCConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.oidcConfig = proto.CloneOf(config)
}

func TestAccOIDCConfigQuery(t *testing.T) {
	service := newFakeOIDCConfigService("org-1")
	service.seedOIDCConfig(oidcConfigV3("project_id"))
	server := newOIDCConfigAPIServer(t, service)
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

func TestAccOIDCConfigQueryReturnsNoResultsWhenNotFound(t *testing.T) {
	service := newFakeOIDCConfigService("org-1")
	server := newOIDCConfigAPIServer(t, service)
	t.Cleanup(server.Close)

	testresource.UnitTest(t, QueryTestCase(server.URL, testresource.TestStep{
		Query: true, Config: oidcConfigQueryConfig(), QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("ona_oidc_config.all", 0),
		},
	}))
}

func TestAccOIDCConfigImportStateSupportsLegacyIDAndResourceIdentity(t *testing.T) {
	service := newFakeOIDCConfigService("org-1")
	server := newOIDCConfigAPIServer(t, service)
	t.Cleanup(server.Close)

	testresource.UnitTest(t, testresource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0),
		},
		PreCheck:                 func() {},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []testresource.TestStep{
			{
				Config: oidcConfigResourceConfig(server.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentityValue("ona_oidc_config.org", tfjsonpath.New("organization_id"), knownvalue.StringExact("org-1")),
					statecheck.ExpectIdentityValueMatchesStateAtPath("ona_oidc_config.org", tfjsonpath.New("organization_id"), tfjsonpath.New("id")),
				},
			},
			{
				ResourceName:      "ona_oidc_config.org",
				ImportState:       true,
				ImportStateId:     "current",
				ImportStateCheck:  checkOIDCConfigImportState("org-1"),
				ImportStateVerify: true,
			},
			{
				ResourceName:    "ona_oidc_config.org",
				ImportState:     true,
				ImportStateKind: testresource.ImportBlockWithResourceIdentity,
			},
		},
	})
}

func checkOIDCConfigImportState(organizationID string) testresource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("expected 1 imported OIDC config state, got %d", len(states))
		}
		if got := states[0].Attributes["id"]; got != organizationID {
			return fmt.Errorf("expected imported OIDC config id %q, got %q", organizationID, got)
		}
		if got := states[0].Attributes["version"]; got != "v3" {
			return fmt.Errorf("expected imported OIDC config version %q, got %q", "v3", got)
		}
		return nil
	}
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

func oidcConfigResourceConfig(host string) string {
	return QueryProviderConfig(host) + `
resource "ona_oidc_config" "org" {
  version          = "v3"
  extra_sub_fields = ["project_id"]
}
`
}

func oidcConfigV3(extraSubFields ...string) *v1.OIDCConfig {
	return &v1.OIDCConfig{
		Version: &v1.OIDCConfig_V3{
			V3: &v1.OIDCConfigV3{
				ExtraSubFields: extraSubFields,
			},
		},
	}
}
