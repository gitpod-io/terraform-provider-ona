// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"sort"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	ssoStateActive   = "active"
	ssoStateInactive = "inactive"

	ssoProviderTypeBuiltin = "builtin"
	ssoProviderTypeCustom  = "custom"
)

func createSSOConfigurationRequest(ctx context.Context, organizationID string, data SSOConfigurationModel, secret types.String) (*v1.CreateSSOConfigurationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateSSOConfigurationModel(ctx, data, &diags)
	if !isKnownString(secret) {
		diags.AddAttributeError(path.Root("client_secret"), "Missing SSO Client Secret", "Set client_secret when creating ona_sso_configuration resources.")
	}
	if diags.HasError() {
		return nil, diags
	}

	req := &v1.CreateSSOConfigurationRequest{
		OrganizationId:   organizationID,
		ClientId:         data.ClientID.ValueString(),
		ClientSecret:     secret.ValueString(),
		IssuerUrl:        data.IssuerURL.ValueString(),
		EmailDomains:     stringSliceFromSet(ctx, data.EmailDomains, &diags),
		DisplayName:      data.DisplayName.ValueString(),
		AdditionalScopes: stringSliceFromSet(ctx, data.AdditionalScopes, &diags),
	}
	if isKnownString(data.ClaimsExpression) {
		req.ClaimsExpression = ptr(data.ClaimsExpression.ValueString())
	}
	if diags.HasError() {
		return nil, diags
	}
	return req, diags
}

func updateSSOConfigurationRequestFromConfig(ctx context.Context, plan SSOConfigurationModel, prior SSOConfigurationModel, cfg tfsdk.Config, secret types.String) (*v1.UpdateSSOConfigurationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateSSOConfigurationModel(ctx, plan, &diags)
	if diags.HasError() {
		return nil, diags
	}

	req := &v1.UpdateSSOConfigurationRequest{
		SsoConfigurationId: plan.ID.ValueString(),
	}

	if value, ok := stringFromConfig(ctx, cfg, path.Root("client_id"), plan.ClientID, &diags); ok {
		req.ClientId = value
	}
	if value, ok := stringFromConfig(ctx, cfg, path.Root("issuer_url"), plan.IssuerURL, &diags); ok {
		req.IssuerUrl = value
	}
	if value, ok := stringFromConfig(ctx, cfg, path.Root("display_name"), plan.DisplayName, &diags); ok {
		req.DisplayName = value
	}
	if !plan.EmailDomains.IsNull() && !plan.EmailDomains.IsUnknown() {
		req.EmailDomains = stringSliceFromSet(ctx, plan.EmailDomains, &diags)
	}
	if !plan.AdditionalScopes.IsNull() && !plan.AdditionalScopes.IsUnknown() {
		req.AdditionalScopes = &v1.AdditionalScopesUpdate{
			Scopes: stringSliceFromSet(ctx, plan.AdditionalScopes, &diags),
		}
	}
	if value, ok := stringFromConfig(ctx, cfg, path.Root("claims_expression"), plan.ClaimsExpression, &diags); ok {
		req.ClaimsExpression = value
	}
	if isKnownString(plan.State) {
		state, ok := ssoStateFromString(plan.State.ValueString())
		if !ok {
			diags.AddAttributeError(path.Root("state"), "Invalid SSO Configuration State", "Supported values are \"active\" and \"inactive\".")
		} else {
			req.State = &state
		}
	}
	if ssoSecretRequiredForUpdate(plan, prior) {
		if !isKnownString(secret) {
			diags.AddAttributeError(path.Root("client_secret"), "Missing SSO Client Secret", "Set client_secret when changing client_id or client_secret_version.")
			return nil, diags
		}
		req.ClientSecret = ptr(secret.ValueString())
	}
	if diags.HasError() {
		return nil, diags
	}
	return req, diags
}

func populateSSOConfigurationModel(ctx context.Context, data *SSOConfigurationModel, sso *v1.SSOConfiguration, prior SSOConfigurationModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(sso.GetId())
	data.ClientID = types.StringValue(sso.GetClientId())
	data.ClientSecret = types.StringNull()
	data.ClientSecretVersion = prior.ClientSecretVersion
	data.IssuerURL = types.StringValue(sso.GetIssuerUrl())
	data.DisplayName = types.StringValue(sso.GetDisplayName())
	data.EmailDomains = stringSetValue(ctx, sso.GetEmailDomains(), prior.EmailDomains, true, diags)
	data.AdditionalScopes = optionalStringSetValue(ctx, sso.GetAdditionalScopes(), prior.AdditionalScopes, diags)
	data.ClaimsExpression = optionalStringValue(sso.GetClaimsExpression(), prior.ClaimsExpression)
	data.State = types.StringValue(ssoStateToString(sso.GetState()))
	data.ProviderType = types.StringValue(ssoProviderTypeToString(sso.GetProviderType()))
}

func preserveSSOConfigurationPlannedInputs(data *SSOConfigurationModel, planned SSOConfigurationModel) {
	data.ClientID = preserveString(data.ClientID, planned.ClientID)
	data.ClientSecret = types.StringNull()
	data.ClientSecretVersion = preserveString(data.ClientSecretVersion, planned.ClientSecretVersion)
	data.IssuerURL = preserveString(data.IssuerURL, planned.IssuerURL)
	data.DisplayName = preserveString(data.DisplayName, planned.DisplayName)
	data.EmailDomains = preserveSet(data.EmailDomains, planned.EmailDomains)
	data.AdditionalScopes = preserveOptionalSet(data.AdditionalScopes, planned.AdditionalScopes)
	data.ClaimsExpression = preserveOptionalString(data.ClaimsExpression, planned.ClaimsExpression)
	data.State = preserveString(data.State, planned.State)
}

func validateSSOConfigurationModel(ctx context.Context, data SSOConfigurationModel, diags *diag.Diagnostics) {
	if isKnownString(data.State) {
		if _, ok := ssoStateFromString(data.State.ValueString()); !ok {
			diags.AddAttributeError(path.Root("state"), "Invalid SSO Configuration State", "Supported values are \"active\" and \"inactive\".")
		}
	}
	if isKnownString(data.DisplayName) && len(data.DisplayName.ValueString()) > 128 {
		diags.AddAttributeError(path.Root("display_name"), "Invalid SSO Display Name", "display_name must be at most 128 characters.")
	}
	if !data.EmailDomains.IsNull() && !data.EmailDomains.IsUnknown() && len(data.EmailDomains.Elements()) == 0 {
		diags.AddAttributeError(path.Root("email_domains"), "Cannot Clear SSO Email Domains", "The Ona API requires at least one email domain when email_domains is configured. Omit this attribute to leave it unmanaged.")
	}

	if !data.EmailDomains.IsNull() && !data.EmailDomains.IsUnknown() {
		var emailDomains []string
		diags.Append(data.EmailDomains.ElementsAs(ctx, &emailDomains, false)...)
	}
}

func readSSOClientSecret(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) types.String {
	var secret types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("client_secret"), &secret)...)
	return secret
}

func ssoStateFromString(value string) (v1.SSOConfigurationState, bool) {
	switch value {
	case ssoStateActive:
		return v1.SSOConfigurationState_SSO_CONFIGURATION_STATE_ACTIVE, true
	case ssoStateInactive:
		return v1.SSOConfigurationState_SSO_CONFIGURATION_STATE_INACTIVE, true
	default:
		return v1.SSOConfigurationState_SSO_CONFIGURATION_STATE_UNSPECIFIED, false
	}
}

func ssoStateToString(value v1.SSOConfigurationState) string {
	switch value {
	case v1.SSOConfigurationState_SSO_CONFIGURATION_STATE_INACTIVE:
		return ssoStateInactive
	case v1.SSOConfigurationState_SSO_CONFIGURATION_STATE_ACTIVE:
		return ssoStateActive
	default:
		return ssoStateInactive
	}
}

func ssoProviderTypeToString(value v1.SSOConfiguration_ProviderType) string {
	switch value {
	case v1.SSOConfiguration_PROVIDER_TYPE_BUILTIN:
		return ssoProviderTypeBuiltin
	case v1.SSOConfiguration_PROVIDER_TYPE_CUSTOM:
		return ssoProviderTypeCustom
	default:
		return ""
	}
}

func ssoSecretRequiredForUpdate(plan SSOConfigurationModel, prior SSOConfigurationModel) bool {
	return stringValueChanged(plan.ClientID, prior.ClientID) ||
		secretVersionChanged(plan.ClientSecretVersion, prior.ClientSecretVersion)
}

func optionalStringValue(value string, prior types.String) types.String {
	if prior.IsNull() {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func optionalStringSetValue(ctx context.Context, values []string, prior types.Set, diags *diag.Diagnostics) types.Set {
	if prior.IsNull() {
		return types.SetNull(types.StringType)
	}
	return stringSetValue(ctx, values, prior, true, diags)
}

func preserveOptionalString(current types.String, planned types.String) types.String {
	if planned.IsNull() {
		return types.StringNull()
	}
	return preserveString(current, planned)
}

func preserveOptionalSet(current types.Set, planned types.Set) types.Set {
	if planned.IsNull() {
		return types.SetNull(types.StringType)
	}
	return preserveSet(current, planned)
}

func preserveSet(current types.Set, planned types.Set) types.Set {
	if planned.IsNull() || planned.IsUnknown() {
		return current
	}
	return planned
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
