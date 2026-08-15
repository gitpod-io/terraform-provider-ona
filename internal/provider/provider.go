// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"

	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/accesscontrol"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/billing"
	gitauthentication "github.com/gitpod-io/terraform-provider-ona/internal/provider/git_authentication"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/integration"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/organization"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/project"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/runner"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/secret"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/security"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/serviceaccount"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/skill"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/user"
	warmpool "github.com/gitpod-io/terraform-provider-ona/internal/provider/warm_pool"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/webhook"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/workflow"
	providerversion "github.com/gitpod-io/terraform-provider-ona/version"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure OnaProvider satisfies various provider interfaces.
var _ provider.Provider = &OnaProvider{}
var _ provider.ProviderWithEphemeralResources = &OnaProvider{}
var _ provider.ProviderWithListResources = &OnaProvider{}

// OnaProvider defines the provider implementation.
type OnaProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// OnaProviderModel describes the provider data model.
type OnaProviderModel struct {
	Host                   types.String `tfsdk:"host"`
	Token                  types.String `tfsdk:"token"`
	RateLimitMaxRetries    types.Int64  `tfsdk:"rate_limit_max_retries"`
	RateLimitMaxRetryDelay types.String `tfsdk:"rate_limit_max_retry_delay"`
}

func (p *OnaProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ona"
	resp.Version = p.version
}

func (p *OnaProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Ona provider manages Ona resources with Terraform.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				MarkdownDescription: "Ona application host, including scheme when a custom host is used. Defaults to `ONA_HOST` when set, otherwise `https://app.gitpod.io`. Most configurations should omit this attribute.",
				Optional:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "Ona API token used by the provider. Defaults to `ONA_TOKEN` when set. Use a personal access token for Terraform write workflows unless Ona has confirmed service-account-token permissions for your organization and use case. Avoid committing this value to configuration.",
				Optional:            true,
				Sensitive:           true,
			},
			"rate_limit_max_retries": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of retries after an Ona API request is rejected by the rate limiter. Defaults to `5`; set to `0` to disable rate-limit retries.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"rate_limit_max_retry_delay": schema.StringAttribute{
				MarkdownDescription: "Maximum server-provided delay before retrying a rate-limited request, as a positive Go duration. Defaults to `30s`.",
				Optional:            true,
				Validators: []validator.String{
					positiveDurationValidator{},
				},
			},
		},
	}
}

func (p *OnaProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data OnaProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			pathRoot("host"),
			"Unknown Ona Host",
			"The provider cannot configure the Ona API client with an unknown host.",
		)
	}
	if data.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			pathRoot("token"),
			"Unknown Ona Token",
			"The provider cannot configure the Ona API client with an unknown token.",
		)
	}
	if data.RateLimitMaxRetries.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			pathRoot("rate_limit_max_retries"),
			"Unknown Rate Limit Retry Count",
			"The provider cannot configure rate-limit retries with an unknown maximum retry count.",
		)
	}
	if data.RateLimitMaxRetryDelay.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			pathRoot("rate_limit_max_retry_delay"),
			"Unknown Rate Limit Retry Delay",
			"The provider cannot configure rate-limit retries with an unknown maximum retry delay.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	maxRetries, err := configuredRateLimitMaxRetries(data.RateLimitMaxRetries)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			pathRoot("rate_limit_max_retries"),
			"Invalid Rate Limit Retry Count",
			err.Error(),
		)
	}
	maxRetryDelay, err := configuredRateLimitMaxRetryDelay(data.RateLimitMaxRetryDelay)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			pathRoot("rate_limit_max_retry_delay"),
			"Invalid Rate Limit Retry Delay",
			err.Error(),
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	var host, token string
	if !data.Host.IsNull() {
		host = data.Host.ValueString()
	}
	if !data.Token.IsNull() {
		token = data.Token.ValueString()
	}

	api, apiBaseURL, err := newManagementPlane(host, token, providerversion.UserAgentFor(p.version), managementclient.RateLimitRetryConfig{
		MaxRetries:    maxRetries,
		MaxRetryDelay: maxRetryDelay,
	})
	if err != nil && !errors.Is(err, errMissingToken) {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Configure Ona API Client", "configuring the Ona API client", err)
		return
	}

	providerData := &providerdata.Data{
		Client:     api,
		APIBaseURL: apiBaseURL,
		UserAgent:  providerversion.UserAgentFor(p.version),
	}

	resp.DataSourceData = providerData
	resp.EphemeralResourceData = providerData
	resp.ListResourceData = providerData
	resp.ResourceData = providerData
}

func (p *OnaProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		accesscontrol.NewAutomationRoleAssignmentResource,
		accesscontrol.NewGroupMembershipResource,
		accesscontrol.NewGroupResource,
		accesscontrol.NewOrganizationRoleAssignmentResource,
		accesscontrol.NewProjectRoleAssignmentResource,
		accesscontrol.NewRunnerRoleAssignmentResource,
		accesscontrol.NewTeamMembershipResource,
		accesscontrol.NewTeamResource,
		billing.NewOrganizationAIBudgetResource,
		billing.NewTeamAIBudgetResource,
		billing.NewUserAIBudgetResource,
		gitauthentication.NewResource,
		integration.NewResource,
		organization.NewAnnouncementBannerResource,
		organization.NewCustomDomainResource,
		organization.NewOIDCConfigResource,
		organization.NewPoliciesResource,
		organization.NewSCIMConfigurationResource,
		organization.NewSSOConfigurationResource,
		organization.NewTermsOfServiceResource,
		project.NewResource,
		runner.NewEnvironmentClassResource,
		runner.NewLLMIntegrationResource,
		runner.NewResource,
		runner.NewSCMIntegrationResource,
		runner.NewTokenResource,
		secret.NewResource,
		security.NewPolicyResource,
		serviceaccount.NewResource,
		skill.NewResource,
		warmpool.NewWarmPoolResource,
		webhook.NewResource,
		workflow.NewResource,
	}
}

func (p *OnaProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		serviceaccount.NewTokenEphemeralResource,
		webhook.NewSecretEphemeralResource,
	}
}

// ListResources returns the managed-resource discovery implementations
// registered by the provider. Resource-specific PRs add constructors here.
func (p *OnaProvider) ListResources(ctx context.Context) []func() list.ListResource {
	return []func() list.ListResource{
		gitauthentication.NewListResource,
		accesscontrol.NewGroupListResource,
		accesscontrol.NewGroupMembershipListResource,
		accesscontrol.NewOrganizationRoleAssignmentListResource,
		integration.NewListResource,
		accesscontrol.NewTeamListResource,
		accesscontrol.NewTeamMembershipListResource,
		organization.NewAnnouncementBannerListResource,
		organization.NewCustomDomainListResource,
		organization.NewOIDCConfigListResource,
		organization.NewPoliciesListResource,
		organization.NewSCIMConfigurationListResource,
		organization.NewSSOConfigurationListResource,
		organization.NewTermsOfServiceListResource,
		project.NewListResource,
		runner.NewEnvironmentClassListResource,
		runner.NewLLMIntegrationListResource,
		runner.NewRunnerListResource,
		runner.NewSCMIntegrationListResource,
		security.NewPolicyListResource,
		secret.NewListResource,
		serviceaccount.NewListResource,
		skill.NewListResource,
		warmpool.NewWarmPoolListResource,
		webhook.NewListResource,
		workflow.NewListResource,
	}
}

func pathRoot(name string) path.Path {
	return path.Root(name)
}

func (p *OnaProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		integration.NewDefinitionsDataSource,
		project.NewProjectDataSource,
		runner.NewCollectionDataSource,
		runner.NewSingularDataSource,
		security.NewPolicyCollectionDataSource,
		skill.NewDataSource,
		user.NewUserCollectionDataSource,
		user.NewUserDataSource,
		warmpool.NewWarmPoolCollectionDataSource,
		warmpool.NewWarmPoolDataSource,
		workflow.NewCollectionDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &OnaProvider{
			version: version,
		}
	}
}
