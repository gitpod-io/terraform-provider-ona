// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package githubapp

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Model struct {
	ID                 types.String `tfsdk:"id"`
	AppID              types.String `tfsdk:"app_id"`
	Enabled            types.Bool   `tfsdk:"enabled"`
	Credentials        types.Object `tfsdk:"credentials"`
	CredentialsVersion types.Int64  `tfsdk:"credentials_version"`
	AppSlug            types.String `tfsdk:"app_slug"`
	ClientID           types.String `tfsdk:"client_id"`
	Setup              types.Object `tfsdk:"setup"`
	Installation       types.Object `tfsdk:"installation"`
}

type credentialsModel struct {
	PrivateKey    types.String `tfsdk:"private_key"`
	ClientSecret  types.String `tfsdk:"client_secret"`
	WebhookSecret types.String `tfsdk:"webhook_secret"`
}

type IdentityModel struct {
	IntegrationID types.String `tfsdk:"integration_id"`
}

var credentialsTypes = map[string]attr.Type{
	"private_key": types.StringType, "client_secret": types.StringType, "webhook_secret": types.StringType,
}

var setupTypes = map[string]attr.Type{
	"callback_url": types.StringType, "webhook_url": types.StringType, "installation_url": types.StringType,
}

var installationTypes = map[string]attr.Type{
	"id": types.StringType, "account_name": types.StringType, "account_type": types.StringType,
}

func emptyModel() Model {
	return Model{
		Credentials:  types.ObjectNull(credentialsTypes),
		Setup:        types.ObjectNull(setupTypes),
		Installation: types.ObjectNull(installationTypes),
	}
}
