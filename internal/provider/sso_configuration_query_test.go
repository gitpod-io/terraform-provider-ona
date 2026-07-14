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
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
			{Path: tfjsonpath.New("client_secret"), KnownValue: knownvalue.Null()},
			{Path: tfjsonpath.New("issuer_url"), KnownValue: knownvalue.StringExact("https://idp.example.com")},
		}),
	}}))
}

func TestSSOConfigurationGeneratedConfigOmitsClientSecret(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	providerServer, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatalf("creating provider server: %v", err)
	}

	schemaResp, err := providerServer.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema() error: %v", err)
	}
	resourceSchema := schemaResp.ResourceSchemas["ona_sso_configuration"]
	if resourceSchema == nil {
		t.Fatal("ona_sso_configuration schema not found")
	}

	stateType := resourceSchema.ValueType()
	stateObject, ok := stateType.(tftypes.Object)
	if !ok {
		t.Fatalf("ona_sso_configuration state type = %T, want tftypes.Object", stateType)
	}
	values := ssoConfigurationGeneratedConfigStateValues(t, stateObject.AttributeTypes)
	state, err := tfprotov6.NewDynamicValue(stateType, tftypes.NewValue(stateType, values))
	if err != nil {
		t.Fatalf("encoding state dynamic value: %v", err)
	}

	resp, err := providerServer.GenerateResourceConfig(ctx, &tfprotov6.GenerateResourceConfigRequest{
		TypeName: "ona_sso_configuration",
		State:    &state,
	})
	if err != nil {
		t.Fatalf("GenerateResourceConfig() error: %v", err)
	}
	if len(resp.Diagnostics) > 0 {
		t.Fatalf("GenerateResourceConfig() diagnostics: %v", resp.Diagnostics)
	}
	if resp.Config == nil {
		t.Fatal("GenerateResourceConfig() returned nil config")
	}

	config, err := resp.Config.Unmarshal(stateType)
	if err != nil {
		t.Fatalf("decoding generated config: %v", err)
	}
	var configValues map[string]tftypes.Value
	if err := config.As(&configValues); err != nil {
		t.Fatalf("generated config value.As(): %v", err)
	}
	if got := configValues["client_secret"]; !got.IsNull() {
		t.Fatalf("generated config client_secret = %s, want null so generated HCL omits it", got)
	}
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

func ssoConfigurationGeneratedConfigStateValues(t *testing.T, attrTypes map[string]tftypes.Type) map[string]tftypes.Value {
	t.Helper()

	values := make(map[string]tftypes.Value, len(attrTypes))
	for name, typ := range attrTypes {
		values[name] = tftypes.NewValue(typ, nil)
	}
	values["id"] = tftypes.NewValue(attrTypes["id"], "sso-custom")
	values["client_id"] = tftypes.NewValue(attrTypes["client_id"], "client-id")
	values["issuer_url"] = tftypes.NewValue(attrTypes["issuer_url"], "https://idp.example.com")
	values["display_name"] = tftypes.NewValue(attrTypes["display_name"], "Example IdP")
	values["email_domains"] = tftypes.NewValue(attrTypes["email_domains"], []tftypes.Value{
		tftypes.NewValue(tftypes.String, "example.com"),
	})
	values["state"] = tftypes.NewValue(attrTypes["state"], "active")
	values["provider_type"] = tftypes.NewValue(attrTypes["provider_type"], "custom")
	return values
}
