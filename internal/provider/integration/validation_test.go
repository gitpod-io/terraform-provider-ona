// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	manualAuth := testAuthObject(t, false, "client-id", "v1")
	dynamicAuth := testAuthObject(t, true, "", "")
	dynamicAuthWithClient := testAuthObject(t, true, "client-id", "")
	manualAuthWithoutSecret := testAuthObject(t, false, "client-id", "v1")
	manualCredentials := testCredentialsObject(t, "client-secret")
	emptyCredentials := types.ObjectNull(credentialsAttributeTypes)
	validCapabilities := testCapabilitiesObject(t, "https://mcp.example.com/mcp")
	httpCapabilities := testCapabilitiesObject(t, "http://mcp.example.com/mcp")

	type Expectation struct {
		Errors []string
	}
	tests := []struct {
		Name     string
		Input    Model
		Expected Expectation
	}{
		{
			Name: "definition_backed_rejects_name",
			Input: Model{
				IntegrationDefinitionID: types.StringValue("definition-1"),
				Name:                    types.StringValue("override"),
				Categories:              types.SetNull(types.StringType),
				Capabilities:            types.ObjectNull(capabilitiesAttributeTypes),
				Auth:                    types.ObjectNull(authResourceAttributeTypes),
			},
			Expected: Expectation{Errors: []string{"Invalid Definition-Backed Integration Name"}},
		},
		{
			Name: "custom_manual_oauth",
			Input: Model{
				IntegrationDefinitionID: types.StringNull(),
				Name:                    types.StringValue("Custom MCP"),
				Categories:              types.SetNull(types.StringType),
				Capabilities:            validCapabilities,
				Auth:                    manualAuth,
				Credentials:             manualCredentials,
			},
			Expected: Expectation{},
		},
		{
			Name: "custom_dynamic_registration",
			Input: Model{
				IntegrationDefinitionID: types.StringNull(),
				Name:                    types.StringValue("Custom MCP"),
				Categories:              types.SetNull(types.StringType),
				Capabilities:            validCapabilities,
				Auth:                    dynamicAuth,
				Credentials:             emptyCredentials,
			},
			Expected: Expectation{},
		},
		{
			Name: "dynamic_registration_rejects_client_id",
			Input: Model{
				IntegrationDefinitionID: types.StringNull(),
				Name:                    types.StringValue("Custom MCP"),
				Categories:              types.SetNull(types.StringType),
				Capabilities:            validCapabilities,
				Auth:                    dynamicAuthWithClient,
				Credentials:             emptyCredentials,
			},
			Expected: Expectation{Errors: []string{"Invalid Dynamic Registration Configuration"}},
		},
		{
			Name: "manual_oauth_requires_secret",
			Input: Model{
				IntegrationDefinitionID: types.StringNull(),
				Name:                    types.StringValue("Custom MCP"),
				Categories:              types.SetNull(types.StringType),
				Capabilities:            validCapabilities,
				Auth:                    manualAuthWithoutSecret,
				Credentials:             emptyCredentials,
			},
			Expected: Expectation{Errors: []string{"Missing OAuth Client Secret"}},
		},
		{
			Name: "custom_mcp_requires_https",
			Input: Model{
				IntegrationDefinitionID: types.StringNull(),
				Name:                    types.StringValue("Custom MCP"),
				Categories:              types.SetNull(types.StringType),
				Capabilities:            httpCapabilities,
				Auth:                    manualAuth,
				Credentials:             manualCredentials,
			},
			Expected: Expectation{Errors: []string{"Invalid HTTPS URL"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var diags diag.Diagnostics
			validateConfig(t.Context(), tc.Input, &diags)
			got := Expectation{}
			for _, diagnostic := range diags {
				if diagnostic.Severity() == diag.SeverityError {
					got.Errors = append(got.Errors, diagnostic.Summary())
				}
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
