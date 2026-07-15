// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID                      types.String `tfsdk:"id"`
	OrganizationID          types.String `tfsdk:"organization_id"`
	IntegrationDefinitionID types.String `tfsdk:"integration_definition_id"`
	RunnerID                types.String `tfsdk:"runner_id"`
	Enabled                 types.Bool   `tfsdk:"enabled"`
	Capabilities            types.Object `tfsdk:"capabilities"`
	Auth                    types.Object `tfsdk:"auth"`
	Credentials             types.Object `tfsdk:"credentials"`
	Host                    types.String `tfsdk:"host"`
	Name                    types.String `tfsdk:"name"`
	Description             types.String `tfsdk:"description"`
	IconURL                 types.String `tfsdk:"icon_url"`
	Categories              types.Set    `tfsdk:"categories"`
	ExternalInstallation    types.Object `tfsdk:"external_installation"`
}

type DefinitionsDataSourceModel struct {
	ID          types.String      `tfsdk:"id"`
	Definitions []DefinitionModel `tfsdk:"definitions"`
}

type DefinitionModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	IconURL      types.String `tfsdk:"icon_url"`
	Host         types.String `tfsdk:"host"`
	Experimental types.Bool   `tfsdk:"experimental"`
	Categories   types.Set    `tfsdk:"categories"`
	Capabilities types.Object `tfsdk:"capabilities"`
	Auth         types.Object `tfsdk:"auth"`
}

type capabilitiesModel struct {
	MCP              types.Object `tfsdk:"mcp"`
	ContextParsing   types.Object `tfsdk:"context_parsing"`
	SourceCodeAccess types.Object `tfsdk:"source_code_access"`
	Login            types.Object `tfsdk:"login"`
	AgentClient      types.Object `tfsdk:"agent_client"`
	SCMPREvents      types.Object `tfsdk:"scm_pr_events"`
}

type mcpModel struct {
	URL types.String `tfsdk:"url"`
}

type agentClientModel struct {
	SeverityThreshold types.String `tfsdk:"severity_threshold"`
	DefaultProjectID  types.String `tfsdk:"default_project_id"`
}

type authModel struct {
	RequiresAuth   types.Bool   `tfsdk:"requires_auth"`
	APIKey         types.Object `tfsdk:"api_key"`
	OAuth          types.Object `tfsdk:"oauth"`
	ProprietaryApp types.Object `tfsdk:"proprietary_app"`
}

type oauthModel struct {
	AuthURL             types.String `tfsdk:"auth_url"`
	TokenURL            types.String `tfsdk:"token_url"`
	Scopes              types.Set    `tfsdk:"scopes"`
	ClientID            types.String `tfsdk:"client_id"`
	ClientSecretVersion types.String `tfsdk:"client_secret_version"`
	RedirectURL         types.String `tfsdk:"redirect_url"`
	DynamicRegistration types.Bool   `tfsdk:"dynamic_registration"`
	AuthParams          types.Map    `tfsdk:"auth_params"`
}

type proprietaryAppModel struct {
	ClientID             types.String `tfsdk:"client_id"`
	ClientSecretVersion  types.String `tfsdk:"client_secret_version"`
	WebhookSecretVersion types.String `tfsdk:"webhook_secret_version"`
	AuthParams           types.Map    `tfsdk:"auth_params"`
	AppScopes            types.Set    `tfsdk:"app_scopes"`
	TokenURL             types.String `tfsdk:"token_url"`
	AppID                types.String `tfsdk:"app_id"`
	PrivateKeyVersion    types.String `tfsdk:"private_key_version"`
	AppSlug              types.String `tfsdk:"app_slug"`
	APIKeyVersion        types.String `tfsdk:"api_key_version"`
}

type credentialsModel struct {
	OAuthClientSecret        types.String `tfsdk:"oauth_client_secret"`
	ProprietaryClientSecret  types.String `tfsdk:"proprietary_client_secret"`
	ProprietaryWebhookSecret types.String `tfsdk:"proprietary_webhook_secret"`
	ProprietaryPrivateKey    types.String `tfsdk:"proprietary_private_key"`
	ProprietaryAPIKey        types.String `tfsdk:"proprietary_api_key"`
}

var emptyObjectAttributeTypes = map[string]attr.Type{}

var mcpAttributeTypes = map[string]attr.Type{
	"url": types.StringType,
}

var agentClientAttributeTypes = map[string]attr.Type{
	"severity_threshold": types.StringType,
	"default_project_id": types.StringType,
}

var capabilitiesAttributeTypes = map[string]attr.Type{
	"mcp":                types.ObjectType{AttrTypes: mcpAttributeTypes},
	"context_parsing":    types.ObjectType{AttrTypes: emptyObjectAttributeTypes},
	"source_code_access": types.ObjectType{AttrTypes: emptyObjectAttributeTypes},
	"login":              types.ObjectType{AttrTypes: emptyObjectAttributeTypes},
	"agent_client":       types.ObjectType{AttrTypes: agentClientAttributeTypes},
	"scm_pr_events":      types.ObjectType{AttrTypes: emptyObjectAttributeTypes},
}

var oauthResourceAttributeTypes = map[string]attr.Type{
	"auth_url":              types.StringType,
	"token_url":             types.StringType,
	"scopes":                types.SetType{ElemType: types.StringType},
	"client_id":             types.StringType,
	"client_secret_version": types.StringType,
	"redirect_url":          types.StringType,
	"dynamic_registration":  types.BoolType,
	"auth_params":           types.MapType{ElemType: types.StringType},
}

var oauthDataSourceAttributeTypes = map[string]attr.Type{
	"auth_url":             types.StringType,
	"token_url":            types.StringType,
	"scopes":               types.SetType{ElemType: types.StringType},
	"client_id":            types.StringType,
	"redirect_url":         types.StringType,
	"dynamic_registration": types.BoolType,
	"auth_params":          types.MapType{ElemType: types.StringType},
}

var proprietaryAppResourceAttributeTypes = map[string]attr.Type{
	"client_id":              types.StringType,
	"client_secret_version":  types.StringType,
	"webhook_secret_version": types.StringType,
	"auth_params":            types.MapType{ElemType: types.StringType},
	"app_scopes":             types.SetType{ElemType: types.StringType},
	"token_url":              types.StringType,
	"app_id":                 types.StringType,
	"private_key_version":    types.StringType,
	"app_slug":               types.StringType,
	"api_key_version":        types.StringType,
}

var credentialsAttributeTypes = map[string]attr.Type{
	"oauth_client_secret":        types.StringType,
	"proprietary_client_secret":  types.StringType,
	"proprietary_webhook_secret": types.StringType,
	"proprietary_private_key":    types.StringType,
	"proprietary_api_key":        types.StringType,
}

var proprietaryAppDataSourceAttributeTypes = map[string]attr.Type{
	"client_id":   types.StringType,
	"auth_params": types.MapType{ElemType: types.StringType},
	"app_scopes":  types.SetType{ElemType: types.StringType},
	"token_url":   types.StringType,
	"app_id":      types.StringType,
	"app_slug":    types.StringType,
}

var authResourceAttributeTypes = map[string]attr.Type{
	"requires_auth":   types.BoolType,
	"api_key":         types.ObjectType{AttrTypes: emptyObjectAttributeTypes},
	"oauth":           types.ObjectType{AttrTypes: oauthResourceAttributeTypes},
	"proprietary_app": types.ObjectType{AttrTypes: proprietaryAppResourceAttributeTypes},
}

var authDataSourceAttributeTypes = map[string]attr.Type{
	"requires_auth":   types.BoolType,
	"api_key":         types.ObjectType{AttrTypes: emptyObjectAttributeTypes},
	"oauth":           types.ObjectType{AttrTypes: oauthDataSourceAttributeTypes},
	"proprietary_app": types.ObjectType{AttrTypes: proprietaryAppDataSourceAttributeTypes},
}

var externalInstallationAttributeTypes = map[string]attr.Type{
	"id":           types.StringType,
	"account_name": types.StringType,
	"account_type": types.StringType,
}
