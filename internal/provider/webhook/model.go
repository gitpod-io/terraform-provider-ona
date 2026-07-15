// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package webhook

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	webhookTypeRepository   = "repository"
	webhookTypeOrganization = "organization"

	webhookProviderGitHub    = "github"
	webhookProviderGitLab    = "gitlab"
	webhookProviderBitbucket = "bitbucket"
)

type Model struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	Type              types.String `tfsdk:"type"`
	Provider          types.String `tfsdk:"scm_provider"`
	RepositoryScopes  types.Set    `tfsdk:"repository_scopes"`
	OrganizationScope types.Object `tfsdk:"organization_scope"`
	SecretVersion     types.String `tfsdk:"secret_version"`
	URL               types.String `tfsdk:"url"`
	Creator           types.Object `tfsdk:"creator"`
	CreatedAt         types.String `tfsdk:"created_at"`
}

type RepositoryScopeModel struct {
	Host  types.String `tfsdk:"host"`
	Owner types.String `tfsdk:"owner"`
	Name  types.String `tfsdk:"name"`
}

type OrganizationScopeModel struct {
	Host types.String `tfsdk:"host"`
	Name types.String `tfsdk:"name"`
}

type SecretModel struct {
	WebhookID types.String `tfsdk:"webhook_id"`
	Secret    types.String `tfsdk:"secret"`
}

var repositoryScopeAttributeTypes = map[string]attr.Type{
	"host":  types.StringType,
	"owner": types.StringType,
	"name":  types.StringType,
}

var organizationScopeAttributeTypes = map[string]attr.Type{
	"host": types.StringType,
	"name": types.StringType,
}

var creatorAttributeTypes = map[string]attr.Type{
	"id":        types.StringType,
	"principal": types.StringType,
}
