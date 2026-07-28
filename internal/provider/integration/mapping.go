// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"context"
	"fmt"
	"sort"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/tfvalue"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func createIntegrationRequest(ctx context.Context, plan Model, config Model) (*v1.CreateIntegrationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	capabilities, capabilityDiags := capabilitiesFromObject(ctx, config.Capabilities)
	diags.Append(capabilityDiags...)
	auth, authDiags := authFromObject(ctx, config.Auth, config.Credentials)
	diags.Append(authDiags...)
	categories, categoryDiags := categoriesFromSet(ctx, config.Categories, path.Root("categories"))
	diags.Append(categoryDiags...)
	if diags.HasError() {
		return nil, diags
	}

	req := &v1.CreateIntegrationRequest{
		IntegrationDefinitionId: knownString(plan.IntegrationDefinitionID),
		RunnerId:                knownString(plan.RunnerID),
		Enabled:                 plan.Enabled.ValueBool(),
		Capabilities:            capabilities,
		Auth:                    auth,
		Host:                    knownString(config.Host),
		Name:                    knownString(config.Name),
		Description:             knownString(config.Description),
		Categories:              categories,
	}
	return req, diags
}

func updateIntegrationRequest(ctx context.Context, plan Model, state Model, config Model) (*v1.UpdateIntegrationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	req := &v1.UpdateIntegrationRequest{Id: plan.ID.ValueString()}

	if !plan.Enabled.Equal(state.Enabled) && tfvalue.IsKnownBool(plan.Enabled) {
		enabled := plan.Enabled.ValueBool()
		req.Enabled = &enabled
	}

	if !plan.Capabilities.Equal(state.Capabilities) {
		capabilities, capabilityDiags := capabilitiesFromObject(ctx, config.Capabilities)
		diags.Append(capabilityDiags...)
		if capabilities == nil && !diags.HasError() {
			capabilities = &v1.IntegrationCapabilities{}
		}
		req.Capabilities = capabilities
	}

	if !plan.Auth.Equal(state.Auth) {
		validateAuthUpdateSecrets(ctx, plan.Auth, state.Auth, config.Credentials, &diags)
		auth, authDiags := authFromObject(ctx, config.Auth, config.Credentials)
		diags.Append(authDiags...)
		if auth == nil && !diags.HasError() {
			auth = &v1.IntegrationAuthentication{}
		}
		req.Auth = auth
	}

	if !plan.Categories.Equal(state.Categories) {
		categories, categoryDiags := categoriesFromSet(ctx, config.Categories, path.Root("categories"))
		diags.Append(categoryDiags...)
		if len(categories) == 0 && !diags.HasError() {
			diags.AddAttributeError(path.Root("categories"), "Unable to Clear Integration Categories", "The Ona API cannot clear integration categories in place. Terraform should replace the integration; rerun the plan and report this issue if replacement was not proposed.")
		} else {
			req.Categories = categories
		}
	}

	if diags.HasError() {
		return nil, diags
	}
	return req, diags
}

func populateModel(ctx context.Context, data *Model, integration *v1.Integration, priorAuth types.Object) diag.Diagnostics {
	var diags diag.Diagnostics
	if integration == nil {
		diags.AddError("Unable to Read Ona Integration", "The Ona API returned an empty integration.")
		return diags
	}

	capabilities, capabilityDiags := capabilitiesObjectFromAPI(integration.GetCapabilities())
	diags.Append(capabilityDiags...)
	auth, authDiags := authResourceObjectFromAPI(ctx, integration.GetAuth(), priorAuth)
	diags.Append(authDiags...)
	categories, categoryDiags := categoriesSetFromAPI(ctx, integration.GetCategories())
	diags.Append(categoryDiags...)
	externalInstallation, externalDiags := externalInstallationObjectFromAPI(ctx, integration.GetExternalInstallation())
	diags.Append(externalDiags...)
	if diags.HasError() {
		return diags
	}

	data.ID = stringValue(integration.GetId())
	data.OrganizationID = stringValue(integration.GetOrganizationId())
	data.IntegrationDefinitionID = tfvalue.OptionalStringValue(integration.GetIntegrationDefinitionId())
	data.RunnerID = tfvalue.OptionalStringValue(integration.GetRunnerId())
	data.Enabled = types.BoolValue(integration.GetEnabled())
	data.Capabilities = capabilities
	data.Auth = auth
	data.Credentials = types.ObjectNull(credentialsAttributeTypes)
	data.Host = tfvalue.OptionalStringValue(integration.GetHost())
	data.Name = tfvalue.OptionalStringValue(integration.GetName())
	data.Description = tfvalue.OptionalStringValue(integration.GetDescription())
	data.IconURL = tfvalue.OptionalStringValue(integration.GetIconUrl())
	data.Categories = categories
	data.ExternalInstallation = externalInstallation
	return diags
}

func preservePlannedInputs(data *Model, planned Model) {
	data.IntegrationDefinitionID = tfvalue.PreserveString(data.IntegrationDefinitionID, planned.IntegrationDefinitionID)
	data.RunnerID = tfvalue.PreserveString(data.RunnerID, planned.RunnerID)
	data.Enabled = tfvalue.PreserveBool(data.Enabled, planned.Enabled)
	data.Host = tfvalue.PreserveString(data.Host, planned.Host)
	data.Name = tfvalue.PreserveString(data.Name, planned.Name)
	data.Description = tfvalue.PreserveString(data.Description, planned.Description)
	if !planned.Categories.IsNull() && !planned.Categories.IsUnknown() {
		data.Categories = planned.Categories
	}
}

func definitionModelFromAPI(ctx context.Context, definition *v1.IntegrationDefinition) (DefinitionModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if definition == nil {
		diags.AddError("Unable to Read Ona Integration Definitions", "The Ona API returned an empty integration definition.")
		return DefinitionModel{}, diags
	}

	capabilities, capabilityDiags := capabilitiesObjectFromAPI(definition.GetCapabilities())
	diags.Append(capabilityDiags...)
	auth, authDiags := authDataSourceObjectFromAPI(ctx, definition.GetAuth())
	diags.Append(authDiags...)
	categories, categoryDiags := categoriesSetFromAPI(ctx, definition.GetCategories())
	diags.Append(categoryDiags...)
	if diags.HasError() {
		return DefinitionModel{}, diags
	}

	return DefinitionModel{
		ID:           stringValue(definition.GetId()),
		Name:         stringValue(definition.GetName()),
		Description:  tfvalue.OptionalStringValue(definition.GetDescription()),
		IconURL:      tfvalue.OptionalStringValue(definition.GetIconUrl()),
		Host:         tfvalue.OptionalStringValue(definition.GetHost()),
		Experimental: types.BoolValue(definition.GetExperimental()),
		Categories:   categories,
		Capabilities: capabilities,
		Auth:         auth,
	}, diags
}

func capabilitiesFromObject(ctx context.Context, value types.Object) (*v1.IntegrationCapabilities, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}
	var model capabilitiesModel
	diags.Append(value.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	result := &v1.IntegrationCapabilities{}
	if !model.MCP.IsNull() && !model.MCP.IsUnknown() {
		var mcp mcpModel
		diags.Append(model.MCP.As(ctx, &mcp, basetypes.ObjectAsOptions{})...)
		result.Mcp = &v1.IntegrationMCPCapability{Url: knownString(mcp.URL)}
	}
	if presentObject(model.ContextParsing) {
		result.ContextParsing = &v1.IntegrationContextParsingCapability{}
	}
	if presentObject(model.SourceCodeAccess) {
		result.SourceCodeAccess = &v1.IntegrationSourceCodeAccessCapability{}
	}
	if presentObject(model.Login) {
		result.Login = &v1.IntegrationLoginCapability{}
	}
	if !model.AgentClient.IsNull() && !model.AgentClient.IsUnknown() {
		var agent agentClientModel
		diags.Append(model.AgentClient.As(ctx, &agent, basetypes.ObjectAsOptions{})...)
		result.AgentClient = &v1.IntegrationAgentClientCapability{
			SeverityThreshold: knownString(agent.SeverityThreshold),
			DefaultProjectId:  knownString(agent.DefaultProjectID),
		}
	}
	if presentObject(model.SCMPREvents) {
		result.ScmPrEvents = &v1.IntegrationScmPrEventsCapability{}
	}
	return result, diags
}

func capabilitiesObjectFromAPI(capabilities *v1.IntegrationCapabilities) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if capabilities == nil {
		return types.ObjectNull(capabilitiesAttributeTypes), diags
	}

	values := map[string]attr.Value{
		"mcp":                types.ObjectNull(mcpAttributeTypes),
		"context_parsing":    types.ObjectNull(emptyObjectAttributeTypes),
		"source_code_access": types.ObjectNull(emptyObjectAttributeTypes),
		"login":              types.ObjectNull(emptyObjectAttributeTypes),
		"agent_client":       types.ObjectNull(agentClientAttributeTypes),
		"scm_pr_events":      types.ObjectNull(emptyObjectAttributeTypes),
	}
	if mcp := capabilities.GetMcp(); mcp != nil {
		values["mcp"] = objectValue(mcpAttributeTypes, map[string]attr.Value{"url": tfvalue.OptionalStringValue(mcp.GetUrl())}, &diags)
	}
	if capabilities.GetContextParsing() != nil {
		values["context_parsing"] = emptyObjectValue(&diags)
	}
	if capabilities.GetSourceCodeAccess() != nil {
		values["source_code_access"] = emptyObjectValue(&diags)
	}
	if capabilities.GetLogin() != nil {
		values["login"] = emptyObjectValue(&diags)
	}
	if agent := capabilities.GetAgentClient(); agent != nil {
		values["agent_client"] = objectValue(agentClientAttributeTypes, map[string]attr.Value{
			"severity_threshold": tfvalue.OptionalStringValue(agent.GetSeverityThreshold()),
			"default_project_id": tfvalue.OptionalStringValue(agent.GetDefaultProjectId()),
		}, &diags)
	}
	if capabilities.GetScmPrEvents() != nil {
		values["scm_pr_events"] = emptyObjectValue(&diags)
	}
	return objectValue(capabilitiesAttributeTypes, values, &diags), diags
}

func authFromObject(ctx context.Context, value types.Object, credentialValue types.Object) (*v1.IntegrationAuthentication, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}
	var model authModel
	diags.Append(value.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}
	credentials := credentialsFromObject(ctx, credentialValue, &diags)

	result := &v1.IntegrationAuthentication{RequiresAuth: knownBool(model.RequiresAuth)}
	if presentObject(model.APIKey) {
		result.ApiKey = &v1.IntegrationAPIKeyConfig{}
	}
	if !model.OAuth.IsNull() && !model.OAuth.IsUnknown() {
		var oauth oauthModel
		diags.Append(model.OAuth.As(ctx, &oauth, basetypes.ObjectAsOptions{})...)
		scopes, scopeDiags := stringsFromSet(ctx, oauth.Scopes)
		diags.Append(scopeDiags...)
		authParams, mapDiags := stringsFromMap(ctx, oauth.AuthParams)
		diags.Append(mapDiags...)
		result.Oauth = &v1.IntegrationOAuthConfig{
			AuthUrl:             knownString(oauth.AuthURL),
			TokenUrl:            knownString(oauth.TokenURL),
			Scopes:              scopes,
			ClientId:            knownString(oauth.ClientID),
			ClientSecret:        knownString(credentials.OAuthClientSecret),
			RedirectUrl:         knownString(oauth.RedirectURL),
			DynamicRegistration: knownBool(oauth.DynamicRegistration),
			AuthParams:          authParams,
		}
	}
	if !model.ProprietaryApp.IsNull() && !model.ProprietaryApp.IsUnknown() {
		var app proprietaryAppModel
		diags.Append(model.ProprietaryApp.As(ctx, &app, basetypes.ObjectAsOptions{})...)
		authParams, mapDiags := stringsFromMap(ctx, app.AuthParams)
		diags.Append(mapDiags...)
		appScopes, scopeDiags := stringsFromSet(ctx, app.AppScopes)
		diags.Append(scopeDiags...)
		result.ProprietaryApp = &v1.IntegrationProprietaryAppConfig{
			ClientId:      knownString(app.ClientID),
			ClientSecret:  knownString(credentials.ProprietaryClientSecret),
			WebhookSecret: knownString(credentials.ProprietaryWebhookSecret),
			AuthParams:    authParams,
			AppScopes:     appScopes,
			TokenUrl:      knownString(app.TokenURL),
			AppId:         knownString(app.AppID),
			PrivateKey:    knownString(credentials.ProprietaryPrivateKey),
			AppSlug:       knownString(app.AppSlug),
			ApiKey:        knownString(credentials.ProprietaryAPIKey),
		}
	}
	return result, diags
}

func authResourceObjectFromAPI(ctx context.Context, auth *v1.IntegrationAuthentication, prior types.Object) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if auth == nil {
		return types.ObjectNull(authResourceAttributeTypes), diags
	}

	priorOAuth, priorApp := authVersionModels(ctx, prior, &diags)
	values := map[string]attr.Value{
		"requires_auth":   types.BoolValue(auth.GetRequiresAuth()),
		"api_key":         types.ObjectNull(emptyObjectAttributeTypes),
		"oauth":           types.ObjectNull(oauthResourceAttributeTypes),
		"proprietary_app": types.ObjectNull(proprietaryAppResourceAttributeTypes),
	}
	if auth.GetApiKey() != nil {
		values["api_key"] = emptyObjectValue(&diags)
	}
	if oauth := auth.GetOauth(); oauth != nil {
		scopes := stringSetValue(ctx, oauth.GetScopes(), &diags)
		authParams := stringMapValue(ctx, oauth.GetAuthParams(), &diags)
		values["oauth"] = objectValue(oauthResourceAttributeTypes, map[string]attr.Value{
			"auth_url":              tfvalue.OptionalStringValue(oauth.GetAuthUrl()),
			"token_url":             tfvalue.OptionalStringValue(oauth.GetTokenUrl()),
			"scopes":                scopes,
			"client_id":             tfvalue.OptionalStringValue(oauth.GetClientId()),
			"client_secret_version": priorOAuth.ClientSecretVersion,
			"redirect_url":          tfvalue.OptionalStringValue(oauth.GetRedirectUrl()),
			"dynamic_registration":  types.BoolValue(oauth.GetDynamicRegistration()),
			"auth_params":           authParams,
		}, &diags)
	}
	if app := auth.GetProprietaryApp(); app != nil {
		values["proprietary_app"] = objectValue(proprietaryAppResourceAttributeTypes, map[string]attr.Value{
			"client_id":              tfvalue.OptionalStringValue(app.GetClientId()),
			"client_secret_version":  priorApp.ClientSecretVersion,
			"webhook_secret_version": priorApp.WebhookSecretVersion,
			"auth_params":            stringMapValue(ctx, app.GetAuthParams(), &diags),
			"app_scopes":             stringSetValue(ctx, app.GetAppScopes(), &diags),
			"token_url":              tfvalue.OptionalStringValue(app.GetTokenUrl()),
			"app_id":                 tfvalue.OptionalStringValue(app.GetAppId()),
			"private_key_version":    priorApp.PrivateKeyVersion,
			"app_slug":               tfvalue.OptionalStringValue(app.GetAppSlug()),
			"api_key_version":        priorApp.APIKeyVersion,
		}, &diags)
	}
	return objectValue(authResourceAttributeTypes, values, &diags), diags
}

func authDataSourceObjectFromAPI(ctx context.Context, auth *v1.IntegrationAuthentication) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if auth == nil {
		return types.ObjectNull(authDataSourceAttributeTypes), diags
	}
	values := map[string]attr.Value{
		"requires_auth":   types.BoolValue(auth.GetRequiresAuth()),
		"api_key":         types.ObjectNull(emptyObjectAttributeTypes),
		"oauth":           types.ObjectNull(oauthDataSourceAttributeTypes),
		"proprietary_app": types.ObjectNull(proprietaryAppDataSourceAttributeTypes),
	}
	if auth.GetApiKey() != nil {
		values["api_key"] = emptyObjectValue(&diags)
	}
	if oauth := auth.GetOauth(); oauth != nil {
		values["oauth"] = objectValue(oauthDataSourceAttributeTypes, map[string]attr.Value{
			"auth_url":             tfvalue.OptionalStringValue(oauth.GetAuthUrl()),
			"token_url":            tfvalue.OptionalStringValue(oauth.GetTokenUrl()),
			"scopes":               stringSetValue(ctx, oauth.GetScopes(), &diags),
			"client_id":            tfvalue.OptionalStringValue(oauth.GetClientId()),
			"redirect_url":         tfvalue.OptionalStringValue(oauth.GetRedirectUrl()),
			"dynamic_registration": types.BoolValue(oauth.GetDynamicRegistration()),
			"auth_params":          stringMapValue(ctx, oauth.GetAuthParams(), &diags),
		}, &diags)
	}
	if app := auth.GetProprietaryApp(); app != nil {
		values["proprietary_app"] = objectValue(proprietaryAppDataSourceAttributeTypes, map[string]attr.Value{
			"client_id":   tfvalue.OptionalStringValue(app.GetClientId()),
			"auth_params": stringMapValue(ctx, app.GetAuthParams(), &diags),
			"app_scopes":  stringSetValue(ctx, app.GetAppScopes(), &diags),
			"token_url":   tfvalue.OptionalStringValue(app.GetTokenUrl()),
			"app_id":      tfvalue.OptionalStringValue(app.GetAppId()),
			"app_slug":    tfvalue.OptionalStringValue(app.GetAppSlug()),
		}, &diags)
	}
	return objectValue(authDataSourceAttributeTypes, values, &diags), diags
}

func externalInstallationObjectFromAPI(_ context.Context, installation *v1.IntegrationExternalInstallation) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if installation == nil {
		return types.ObjectNull(externalInstallationAttributeTypes), diags
	}
	return objectValue(externalInstallationAttributeTypes, map[string]attr.Value{
		"id":           tfvalue.OptionalStringValue(installation.GetId()),
		"account_name": tfvalue.OptionalStringValue(installation.GetAccountName()),
		"account_type": tfvalue.OptionalStringValue(installation.GetAccountType()),
	}, &diags), diags
}

func categoriesFromSet(ctx context.Context, value types.Set, p path.Path) ([]v1.IntegrationCategory, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}
	var raw []string
	diags.Append(value.ElementsAs(ctx, &raw, false)...)
	if diags.HasError() {
		return nil, diags
	}
	sort.Strings(raw)
	result := make([]v1.IntegrationCategory, 0, len(raw))
	for _, category := range raw {
		mapped, ok := categoryFromString(category)
		if !ok {
			diags.AddAttributeError(p, "Invalid Integration Category", fmt.Sprintf("Unsupported category %q. Supported values are: %v.", category, categoryNames()))
			continue
		}
		result = append(result, mapped)
	}
	return result, diags
}

func categoriesSetFromAPI(ctx context.Context, categories []v1.IntegrationCategory) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	values := make([]string, 0, len(categories))
	for _, category := range categories {
		name := categoryToString(category)
		if name == "" {
			diags.AddError("Unable to Read Ona Integration", fmt.Sprintf("The Ona API returned unsupported integration category %q.", category.String()))
			continue
		}
		values = append(values, name)
	}
	sort.Strings(values)
	result, setDiags := types.SetValueFrom(ctx, types.StringType, values)
	diags.Append(setDiags...)
	return result, diags
}

func categoryFromString(value string) (v1.IntegrationCategory, bool) {
	switch value {
	case "source_control":
		return v1.IntegrationCategory_INTEGRATION_CATEGORY_SOURCE_CONTROL, true
	case "communication":
		return v1.IntegrationCategory_INTEGRATION_CATEGORY_COMMUNICATION, true
	case "project_management":
		return v1.IntegrationCategory_INTEGRATION_CATEGORY_PROJECT_MANAGEMENT, true
	case "observability":
		return v1.IntegrationCategory_INTEGRATION_CATEGORY_OBSERVABILITY, true
	case "data_analytics":
		return v1.IntegrationCategory_INTEGRATION_CATEGORY_DATA_ANALYTICS, true
	case "knowledge":
		return v1.IntegrationCategory_INTEGRATION_CATEGORY_KNOWLEDGE, true
	case "mcp":
		return v1.IntegrationCategory_INTEGRATION_CATEGORY_MCP, true
	case "automation_triggers":
		return v1.IntegrationCategory_INTEGRATION_CATEGORY_AUTOMATION_TRIGGERS, true
	case "ai":
		return v1.IntegrationCategory_INTEGRATION_CATEGORY_AI, true
	default:
		return v1.IntegrationCategory_INTEGRATION_CATEGORY_UNSPECIFIED, false
	}
}

func categoryToString(value v1.IntegrationCategory) string {
	switch value {
	case v1.IntegrationCategory_INTEGRATION_CATEGORY_SOURCE_CONTROL:
		return "source_control"
	case v1.IntegrationCategory_INTEGRATION_CATEGORY_COMMUNICATION:
		return "communication"
	case v1.IntegrationCategory_INTEGRATION_CATEGORY_PROJECT_MANAGEMENT:
		return "project_management"
	case v1.IntegrationCategory_INTEGRATION_CATEGORY_OBSERVABILITY:
		return "observability"
	case v1.IntegrationCategory_INTEGRATION_CATEGORY_DATA_ANALYTICS:
		return "data_analytics"
	case v1.IntegrationCategory_INTEGRATION_CATEGORY_KNOWLEDGE:
		return "knowledge"
	case v1.IntegrationCategory_INTEGRATION_CATEGORY_MCP:
		return "mcp"
	case v1.IntegrationCategory_INTEGRATION_CATEGORY_AUTOMATION_TRIGGERS:
		return "automation_triggers"
	case v1.IntegrationCategory_INTEGRATION_CATEGORY_AI:
		return "ai"
	default:
		return ""
	}
}

func categoryNames() []string {
	return []string{"ai", "automation_triggers", "communication", "data_analytics", "knowledge", "mcp", "observability", "project_management", "source_control"}
}

func validateAuthUpdateSecrets(ctx context.Context, plan types.Object, state types.Object, credentialValue types.Object, diags *diag.Diagnostics) {
	planOAuth, planApp := authVersionModels(ctx, plan, diags)
	stateOAuth, stateApp := authVersionModels(ctx, state, diags)
	credentials := credentialsFromObject(ctx, credentialValue, diags)
	if diags.HasError() {
		return
	}

	if !tfvalue.IsKnownString(credentials.OAuthClientSecret) {
		if versionChanged(planOAuth.ClientSecretVersion, stateOAuth.ClientSecretVersion) {
			diags.AddAttributeError(path.Root("credentials").AtName("oauth_client_secret"), "Missing OAuth Client Secret", "Set credentials.oauth_client_secret when changing auth.oauth.client_secret_version.")
		} else if tfvalue.IsKnownString(planOAuth.ClientSecretVersion) {
			diags.AddAttributeError(path.Root("credentials").AtName("oauth_client_secret"), "Missing OAuth Client Secret", "Resupply credentials.oauth_client_secret when updating OAuth configuration with a non-null client_secret_version so the backend does not clear the stored secret.")
		}
	}
	secretVersionChecks := []struct {
		planned types.String
		prior   types.String
		secret  types.String
		name    string
	}{
		{planApp.ClientSecretVersion, stateApp.ClientSecretVersion, credentials.ProprietaryClientSecret, "proprietary_client_secret"},
		{planApp.WebhookSecretVersion, stateApp.WebhookSecretVersion, credentials.ProprietaryWebhookSecret, "proprietary_webhook_secret"},
		{planApp.PrivateKeyVersion, stateApp.PrivateKeyVersion, credentials.ProprietaryPrivateKey, "proprietary_private_key"},
		{planApp.APIKeyVersion, stateApp.APIKeyVersion, credentials.ProprietaryAPIKey, "proprietary_api_key"},
	}
	for _, check := range secretVersionChecks {
		if versionChanged(check.planned, check.prior) && !tfvalue.IsKnownString(check.secret) {
			diags.AddAttributeError(path.Root("credentials").AtName(check.name), "Missing Proprietary App Credential", fmt.Sprintf("Set credentials.%s when changing its version marker.", check.name))
		}
	}
}

func authVersionModels(ctx context.Context, value types.Object, diags *diag.Diagnostics) (oauthModel, proprietaryAppModel) {
	var oauth oauthModel
	var app proprietaryAppModel
	if value.IsNull() || value.IsUnknown() {
		return oauth, app
	}
	var auth authModel
	diags.Append(value.As(ctx, &auth, basetypes.ObjectAsOptions{})...)
	if !auth.OAuth.IsNull() && !auth.OAuth.IsUnknown() {
		diags.Append(auth.OAuth.As(ctx, &oauth, basetypes.ObjectAsOptions{})...)
	}
	if !auth.ProprietaryApp.IsNull() && !auth.ProprietaryApp.IsUnknown() {
		diags.Append(auth.ProprietaryApp.As(ctx, &app, basetypes.ObjectAsOptions{})...)
	}
	return oauth, app
}

func credentialsFromObject(ctx context.Context, value types.Object, diags *diag.Diagnostics) credentialsModel {
	var credentials credentialsModel
	if value.IsNull() || value.IsUnknown() {
		return credentials
	}
	diags.Append(value.As(ctx, &credentials, basetypes.ObjectAsOptions{})...)
	return credentials
}

func stringsFromSet(ctx context.Context, value types.Set) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}
	var result []string
	diags.Append(value.ElementsAs(ctx, &result, false)...)
	sort.Strings(result)
	return result, diags
}

func stringsFromMap(ctx context.Context, value types.Map) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}
	result := map[string]string{}
	diags.Append(value.ElementsAs(ctx, &result, false)...)
	return result, diags
}

func stringSetValue(ctx context.Context, values []string, diags *diag.Diagnostics) types.Set {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	result, valueDiags := types.SetValueFrom(ctx, types.StringType, sorted)
	diags.Append(valueDiags...)
	return result
}

func stringMapValue(ctx context.Context, values map[string]string, diags *diag.Diagnostics) types.Map {
	if values == nil {
		values = map[string]string{}
	}
	result, valueDiags := types.MapValueFrom(ctx, types.StringType, values)
	diags.Append(valueDiags...)
	return result
}

func objectValue(attributeTypes map[string]attr.Type, values map[string]attr.Value, diags *diag.Diagnostics) types.Object {
	result, valueDiags := types.ObjectValue(attributeTypes, values)
	diags.Append(valueDiags...)
	return result
}

func emptyObjectValue(diags *diag.Diagnostics) types.Object {
	return objectValue(emptyObjectAttributeTypes, map[string]attr.Value{}, diags)
}

func presentObject(value types.Object) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func knownString(value types.String) string {
	if !tfvalue.IsKnownString(value) {
		return ""
	}
	return value.ValueString()
}

func knownBool(value types.Bool) bool {
	return tfvalue.IsKnownBool(value) && value.ValueBool()
}

func stringValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func versionChanged(planned types.String, prior types.String) bool {
	return !planned.Equal(prior)
}
