// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func validateConfig(ctx context.Context, data Model, diags *diag.Diagnostics) {
	_, categoryDiags := categoriesFromSet(ctx, data.Categories, path.Root("categories"))
	diags.Append(categoryDiags...)
	validateAgentClient(ctx, data.Capabilities, diags)
	if diags.HasError() {
		return
	}

	if data.IntegrationDefinitionID.IsUnknown() {
		return
	}
	if isKnownString(data.IntegrationDefinitionID) {
		validateDefinitionBackedConfig(ctx, data, diags)
		return
	}
	validateCustomConfig(ctx, data, diags)
}

func validateDefinitionBackedConfig(ctx context.Context, data Model, diags *diag.Diagnostics) {
	if isKnownString(data.Name) {
		diags.AddAttributeError(path.Root("name"), "Invalid Definition-Backed Integration Name", "Do not configure name when integration_definition_id is set. Ona resolves the name from the selected definition.")
	}
	if isKnownString(data.Description) {
		diags.AddAttributeError(path.Root("description"), "Invalid Definition-Backed Integration Description", "Do not configure description when integration_definition_id is set. Ona resolves the description from the selected definition.")
	}
	if data.Auth.IsNull() || data.Auth.IsUnknown() {
		return
	}
	var auth authModel
	diags.Append(data.Auth.As(ctx, &auth, basetypes.ObjectAsOptions{})...)
	if !auth.RequiresAuth.IsNull() && !auth.RequiresAuth.IsUnknown() {
		diags.AddAttributeError(path.Root("auth").AtName("requires_auth"), "Invalid Authentication Override", "Do not configure requires_auth for a definition-backed integration. Ona resolves it from the selected definition.")
	}
}

func validateCustomConfig(ctx context.Context, data Model, diags *diag.Diagnostics) {
	if !data.Name.IsUnknown() && !isKnownString(data.Name) {
		diags.AddAttributeError(path.Root("name"), "Missing Custom Integration Name", "Set name when integration_definition_id is omitted.")
	}

	var capabilities capabilitiesModel
	if data.Capabilities.IsNull() {
		diags.AddAttributeError(path.Root("capabilities").AtName("mcp"), "Missing Custom Integration MCP Capability", "Custom integrations require capabilities.mcp.url.")
		return
	}
	if data.Capabilities.IsUnknown() {
		return
	}
	diags.Append(data.Capabilities.As(ctx, &capabilities, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}
	if capabilities.MCP.IsNull() {
		diags.AddAttributeError(path.Root("capabilities").AtName("mcp"), "Missing Custom Integration MCP Capability", "Custom integrations require capabilities.mcp.url.")
		return
	}
	if capabilities.MCP.IsUnknown() {
		return
	}
	var mcp mcpModel
	diags.Append(capabilities.MCP.As(ctx, &mcp, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}
	if !mcp.URL.IsUnknown() {
		if !isKnownString(mcp.URL) {
			diags.AddAttributeError(path.Root("capabilities").AtName("mcp").AtName("url"), "Missing Custom Integration MCP URL", "Custom integrations require capabilities.mcp.url.")
		} else {
			validateHTTPSURL(mcp.URL.ValueString(), path.Root("capabilities").AtName("mcp").AtName("url"), diags)
		}
	}

	if data.Auth.IsNull() {
		diags.AddAttributeError(path.Root("auth").AtName("oauth"), "Missing Custom Integration OAuth Configuration", "Custom integrations require an auth.oauth block.")
		return
	}
	if data.Auth.IsUnknown() {
		return
	}
	var auth authModel
	diags.Append(data.Auth.As(ctx, &auth, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}
	if auth.OAuth.IsNull() {
		diags.AddAttributeError(path.Root("auth").AtName("oauth"), "Missing Custom Integration OAuth Configuration", "Custom integrations require an auth.oauth block.")
		return
	}
	if auth.OAuth.IsUnknown() {
		return
	}
	var oauth oauthModel
	diags.Append(auth.OAuth.As(ctx, &oauth, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}
	credentials := credentialsFromObject(ctx, data.Credentials, diags)

	if isKnownBool(oauth.DynamicRegistration) && oauth.DynamicRegistration.ValueBool() {
		for _, field := range []struct {
			name  string
			value types.String
		}{
			{"client_id", oauth.ClientID},
			{"auth_url", oauth.AuthURL},
			{"token_url", oauth.TokenURL},
		} {
			if isKnownString(field.value) {
				diags.AddAttributeError(path.Root("auth").AtName("oauth").AtName(field.name), "Invalid Dynamic Registration Configuration", fmt.Sprintf("Do not configure %s when dynamic_registration is true.", field.name))
			}
		}
		if isKnownString(credentials.OAuthClientSecret) {
			diags.AddAttributeError(path.Root("credentials").AtName("oauth_client_secret"), "Invalid Dynamic Registration Configuration", "Do not configure credentials.oauth_client_secret when dynamic_registration is true.")
		}
	} else if !oauth.DynamicRegistration.IsUnknown() {
		if !isKnownString(oauth.ClientID) {
			diags.AddAttributeError(path.Root("auth").AtName("oauth").AtName("client_id"), "Missing OAuth Client ID", "Set client_id for a custom integration using manual OAuth, or set dynamic_registration to true.")
		}
		if !isKnownString(credentials.OAuthClientSecret) {
			diags.AddAttributeError(path.Root("credentials").AtName("oauth_client_secret"), "Missing OAuth Client Secret", "Set credentials.oauth_client_secret for a custom integration using manual OAuth.")
		}
	}

	if isKnownString(oauth.AuthURL) {
		validateHTTPSURL(oauth.AuthURL.ValueString(), path.Root("auth").AtName("oauth").AtName("auth_url"), diags)
	}
	if isKnownString(oauth.TokenURL) {
		validateHTTPSURL(oauth.TokenURL.ValueString(), path.Root("auth").AtName("oauth").AtName("token_url"), diags)
	}
}

func validateAgentClient(ctx context.Context, capabilities types.Object, diags *diag.Diagnostics) {
	if capabilities.IsNull() || capabilities.IsUnknown() {
		return
	}
	var model capabilitiesModel
	diags.Append(capabilities.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() || model.AgentClient.IsNull() || model.AgentClient.IsUnknown() {
		return
	}
	var agent agentClientModel
	diags.Append(model.AgentClient.As(ctx, &agent, basetypes.ObjectAsOptions{})...)
	if diags.HasError() || !isKnownString(agent.SeverityThreshold) {
		return
	}
	switch agent.SeverityThreshold.ValueString() {
	case "SEV1", "SEV2", "SEV3":
	default:
		diags.AddAttributeError(path.Root("capabilities").AtName("agent_client").AtName("severity_threshold"), "Invalid Agent Client Severity", "Supported values are SEV1, SEV2, and SEV3.")
	}
}

func validateHTTPSURL(value string, attributePath path.Path, diags *diag.Diagnostics) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		diags.AddAttributeError(attributePath, "Invalid HTTPS URL", fmt.Sprintf("%q is not a valid absolute URL.", value))
		return
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		diags.AddAttributeError(attributePath, "Invalid HTTPS URL", "The URL must use the https scheme.")
	}
}
