// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package secret

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	scopeOrganization   = "organization"
	scopeProject        = "project"
	scopeUser           = "user"
	scopeServiceAccount = "service_account"

	principalUser           = "user"
	principalServiceAccount = "service_account"
)

type Model struct {
	ID                             types.String           `tfsdk:"id"`
	Scope                          types.String           `tfsdk:"scope"`
	OrganizationID                 types.String           `tfsdk:"organization_id"`
	ProjectID                      types.String           `tfsdk:"project_id"`
	UserID                         types.String           `tfsdk:"user_id"`
	ServiceAccountID               types.String           `tfsdk:"service_account_id"`
	Name                           types.String           `tfsdk:"name"`
	Value                          types.String           `tfsdk:"value"`
	ValueVersion                   types.String           `tfsdk:"value_version"`
	CreatedAt                      types.String           `tfsdk:"created_at"`
	UpdatedAt                      types.String           `tfsdk:"updated_at"`
	Creator                        types.Object           `tfsdk:"creator"`
	EnvironmentVariable            types.Bool             `tfsdk:"environment_variable"`
	FilePath                       types.String           `tfsdk:"file_path"`
	ContainerRegistryBasicAuthHost types.String           `tfsdk:"container_registry_basic_auth_host"`
	APIOnly                        types.Bool             `tfsdk:"api_only"`
	CredentialProxy                []CredentialProxyModel `tfsdk:"credential_proxy"`
}

type CredentialProxyModel struct {
	TargetHosts types.Set    `tfsdk:"target_hosts"`
	Header      types.String `tfsdk:"header"`
}

var subjectObjectAttributeTypes = map[string]attr.Type{
	"id":        types.StringType,
	"principal": types.StringType,
}
