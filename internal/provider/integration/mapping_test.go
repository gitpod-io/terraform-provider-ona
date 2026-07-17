// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestCategoryMappings(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		API   v1.IntegrationCategory
		Name  string
		Valid bool
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{Name: "source_control", Input: "source_control", Expected: Expectation{API: v1.IntegrationCategory_INTEGRATION_CATEGORY_SOURCE_CONTROL, Name: "source_control", Valid: true}},
		{Name: "communication", Input: "communication", Expected: Expectation{API: v1.IntegrationCategory_INTEGRATION_CATEGORY_COMMUNICATION, Name: "communication", Valid: true}},
		{Name: "project_management", Input: "project_management", Expected: Expectation{API: v1.IntegrationCategory_INTEGRATION_CATEGORY_PROJECT_MANAGEMENT, Name: "project_management", Valid: true}},
		{Name: "observability", Input: "observability", Expected: Expectation{API: v1.IntegrationCategory_INTEGRATION_CATEGORY_OBSERVABILITY, Name: "observability", Valid: true}},
		{Name: "data_analytics", Input: "data_analytics", Expected: Expectation{API: v1.IntegrationCategory_INTEGRATION_CATEGORY_DATA_ANALYTICS, Name: "data_analytics", Valid: true}},
		{Name: "knowledge", Input: "knowledge", Expected: Expectation{API: v1.IntegrationCategory_INTEGRATION_CATEGORY_KNOWLEDGE, Name: "knowledge", Valid: true}},
		{Name: "mcp", Input: "mcp", Expected: Expectation{API: v1.IntegrationCategory_INTEGRATION_CATEGORY_MCP, Name: "mcp", Valid: true}},
		{Name: "automation_triggers", Input: "automation_triggers", Expected: Expectation{API: v1.IntegrationCategory_INTEGRATION_CATEGORY_AUTOMATION_TRIGGERS, Name: "automation_triggers", Valid: true}},
		{Name: "ai", Input: "ai", Expected: Expectation{API: v1.IntegrationCategory_INTEGRATION_CATEGORY_AI, Name: "ai", Valid: true}},
		{Name: "unknown", Input: "unknown", Expected: Expectation{API: v1.IntegrationCategory_INTEGRATION_CATEGORY_UNSPECIFIED}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			apiCategory, valid := categoryFromString(tc.Input)
			got := Expectation{API: apiCategory, Valid: valid}
			if valid {
				got.Name = categoryToString(apiCategory)
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("category mapping mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateIntegrationRequest(t *testing.T) {
	t.Parallel()

	capabilities := testCapabilitiesObject(t, "https://mcp.example.com/mcp")
	auth := testAuthObject(t, false, "client-id", "v1")
	credentials := testCredentialsObject(t, "client-secret")
	categories := testStringSet(t, "mcp", "ai")
	plan := Model{
		IntegrationDefinitionID: types.StringValue("definition-1"),
		RunnerID:                types.StringValue("runner-1"),
		Enabled:                 types.BoolValue(true),
	}
	config := Model{
		Capabilities: capabilities,
		Auth:         auth,
		Credentials:  credentials,
		Host:         types.StringValue("example.com"),
		Categories:   categories,
	}

	request, diags := createIntegrationRequest(t.Context(), plan, config)
	if diags.HasError() {
		t.Fatalf("createIntegrationRequest() diagnostics: %v", diags)
	}
	expected := &v1.CreateIntegrationRequest{
		IntegrationDefinitionId: "definition-1",
		RunnerId:                "runner-1",
		Enabled:                 true,
		Capabilities: &v1.IntegrationCapabilities{
			Mcp: &v1.IntegrationMCPCapability{Url: "https://mcp.example.com/mcp"},
		},
		Auth: &v1.IntegrationAuthentication{
			Oauth: &v1.IntegrationOAuthConfig{ClientId: "client-id", ClientSecret: "client-secret"},
		},
		Host:       "example.com",
		Categories: []v1.IntegrationCategory{v1.IntegrationCategory_INTEGRATION_CATEGORY_AI, v1.IntegrationCategory_INTEGRATION_CATEGORY_MCP},
	}
	if diff := cmp.Diff(expected, request, protocmp.Transform()); diff != "" {
		t.Errorf("createIntegrationRequest() mismatch (-want +got):\n%s", diff)
	}
}

func TestAuthResourceObjectOmitsSecretsAndPreservesVersions(t *testing.T) {
	t.Parallel()

	prior := testAuthObject(t, false, "client-id", "v7")
	value, diags := authResourceObjectFromAPI(t.Context(), &v1.IntegrationAuthentication{
		Oauth: &v1.IntegrationOAuthConfig{ClientId: "client-id", ClientSecret: "server-secret"},
	}, prior)
	if diags.HasError() {
		t.Fatalf("authResourceObjectFromAPI() diagnostics: %v", diags)
	}
	var auth authModel
	diags.Append(value.As(t.Context(), &auth, basetypesObjectAsOptions())...)
	var oauth oauthModel
	diags.Append(auth.OAuth.As(t.Context(), &oauth, basetypesObjectAsOptions())...)
	if diags.HasError() {
		t.Fatalf("decode auth object: %v", diags)
	}

	type Expectation struct {
		SecretAttributeDefined bool
		Version                string
	}
	_, secretAttributeDefined := oauthResourceAttributeTypes["client_secret"]
	got := Expectation{SecretAttributeDefined: secretAttributeDefined, Version: oauth.ClientSecretVersion.ValueString()}
	expected := Expectation{Version: "v7"}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("auth state mismatch (-want +got):\n%s", diff)
	}
}

func TestUpdateIntegrationRequestRequiresOAuthSecret(t *testing.T) {
	t.Parallel()

	emptyCategories := testStringSet(t)
	stateAuth := testAuthObject(t, false, "client-id", "v1")
	planAuth := testAuthObject(t, false, "client-id-updated", "v2")
	configAuth := testAuthObject(t, false, "client-id-updated", "v2")
	configWithoutSecret := types.ObjectNull(credentialsAttributeTypes)
	configWithSecret := testCredentialsObject(t, "new-secret")

	type Expectation struct {
		Secret string
		Error  string
	}
	tests := []struct {
		Name     string
		Config   types.Object
		Expected Expectation
	}{
		{Name: "missing_secret", Config: configWithoutSecret, Expected: Expectation{Error: "Missing OAuth Client Secret"}},
		{Name: "configured_secret", Config: configWithSecret, Expected: Expectation{Secret: "new-secret"}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			plan := Model{ID: types.StringValue("integration-1"), Enabled: types.BoolValue(false), Auth: planAuth, Capabilities: types.ObjectNull(capabilitiesAttributeTypes), Categories: emptyCategories}
			state := Model{ID: types.StringValue("integration-1"), Enabled: types.BoolValue(false), Auth: stateAuth, Capabilities: types.ObjectNull(capabilitiesAttributeTypes), Categories: emptyCategories}
			config := Model{Auth: configAuth, Credentials: tc.Config, Categories: emptyCategories}
			request, diags := updateIntegrationRequest(t.Context(), plan, state, config)
			var got Expectation
			if diags.HasError() {
				got.Error = diags[0].Summary()
			} else {
				got.Secret = request.GetAuth().GetOauth().GetClientSecret()
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("updateIntegrationRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func testCapabilitiesObject(t *testing.T, mcpURL string) types.Object {
	t.Helper()
	var diags diag.Diagnostics
	mcp := objectValue(mcpAttributeTypes, map[string]attr.Value{"url": stringValue(mcpURL)}, &diags)
	value := objectValue(capabilitiesAttributeTypes, map[string]attr.Value{
		"mcp":                mcp,
		"context_parsing":    types.ObjectNull(emptyObjectAttributeTypes),
		"source_code_access": types.ObjectNull(emptyObjectAttributeTypes),
		"login":              types.ObjectNull(emptyObjectAttributeTypes),
		"agent_client":       types.ObjectNull(agentClientAttributeTypes),
		"scm_pr_events":      types.ObjectNull(emptyObjectAttributeTypes),
	}, &diags)
	if diags.HasError() {
		t.Fatalf("create capabilities object: %v", diags)
	}
	return value
}

func testAuthObject(t *testing.T, dynamic bool, clientID, version string) types.Object {
	t.Helper()
	var diags diag.Diagnostics
	oauth := objectValue(oauthResourceAttributeTypes, map[string]attr.Value{
		"auth_url":              types.StringNull(),
		"token_url":             types.StringNull(),
		"scopes":                types.SetNull(types.StringType),
		"client_id":             optionalStringValue(clientID),
		"client_secret_version": optionalStringValue(version),
		"redirect_url":          types.StringNull(),
		"dynamic_registration":  types.BoolValue(dynamic),
		"auth_params":           types.MapNull(types.StringType),
	}, &diags)
	value := objectValue(authResourceAttributeTypes, map[string]attr.Value{
		"requires_auth":   types.BoolNull(),
		"api_key":         types.ObjectNull(emptyObjectAttributeTypes),
		"oauth":           oauth,
		"proprietary_app": types.ObjectNull(proprietaryAppResourceAttributeTypes),
	}, &diags)
	if diags.HasError() {
		t.Fatalf("create auth object: %v", diags)
	}
	return value
}

func testCredentialsObject(t *testing.T, oauthClientSecret string) types.Object {
	t.Helper()
	var diags diag.Diagnostics
	value := objectValue(credentialsAttributeTypes, map[string]attr.Value{
		"oauth_client_secret":        optionalStringValue(oauthClientSecret),
		"proprietary_client_secret":  types.StringNull(),
		"proprietary_webhook_secret": types.StringNull(),
		"proprietary_private_key":    types.StringNull(),
		"proprietary_api_key":        types.StringNull(),
	}, &diags)
	if diags.HasError() {
		t.Fatalf("create credentials object: %v", diags)
	}
	return value
}

func testStringSet(t *testing.T, values ...string) types.Set {
	t.Helper()
	result, diags := types.SetValueFrom(t.Context(), types.StringType, values)
	if diags.HasError() {
		t.Fatalf("create string set: %v", diags)
	}
	return result
}

func basetypesObjectAsOptions() basetypes.ObjectAsOptions {
	return basetypes.ObjectAsOptions{}
}
