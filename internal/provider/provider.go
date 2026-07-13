// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	onaclient "github.com/gitpod-io/terraform-provider-ona/internal/client"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/accesscontrol"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/organization"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/project"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/runner"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/security"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/serviceaccount"
	warmpool "github.com/gitpod-io/terraform-provider-ona/internal/provider/warm_pool"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure OnaProvider satisfies various provider interfaces.
var _ provider.Provider = &OnaProvider{}
var _ provider.ProviderWithEphemeralResources = &OnaProvider{}

// OnaProvider defines the provider implementation.
type OnaProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// OnaProviderModel describes the provider data model.
type OnaProviderModel struct {
	Host  types.String `tfsdk:"host"`
	Token types.String `tfsdk:"token"`
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
				MarkdownDescription: "Ona API token used by the provider. Defaults to `ONA_TOKEN` when set. Use a service-account token for automation and avoid committing this value to configuration.",
				Optional:            true,
				Sensitive:           true,
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
	if resp.Diagnostics.HasError() {
		return
	}

	var cfg onaclient.Config
	if !data.Host.IsNull() {
		cfg.Host = data.Host.ValueString()
	}
	if !data.Token.IsNull() {
		cfg.Token = data.Token.ValueString()
	}
	cfg.UserAgent = fmt.Sprintf("terraform-provider-ona/%s", p.version)

	api, apiBaseURL, err := onaclient.NewManagementPlane(cfg)
	if err != nil {
		if !errors.Is(err, onaclient.ErrMissingToken) {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Configure Ona API Client", "configuring the Ona API client", err)
			return
		}
	}

	providerData := &providerdata.Data{
		Client:     api,
		APIBaseURL: apiBaseURL,
		UserAgent:  cfg.UserAgent,
	}

	resp.DataSourceData = providerData
	resp.EphemeralResourceData = providerData
	resp.ResourceData = providerData
}

func (p *OnaProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		runner.NewResource,
		runner.NewSCMIntegrationResource,
		runner.NewEnvironmentClassResource,
		project.NewResource,
		project.NewInsightsResource,
		security.NewPolicyResource,
		organization.NewPoliciesResource,
		warmpool.NewWarmPoolResource,
		serviceaccount.NewResource,
		accesscontrol.NewGroupResource,
		accesscontrol.NewGroupMembershipResource,
		accesscontrol.NewOrganizationRoleAssignmentResource,
	}
}

func (p *OnaProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		runner.NewTokenEphemeralResource,
		serviceaccount.NewTokenEphemeralResource,
	}
}

func pathRoot(name string) path.Path {
	return path.Root(name)
}

func (p *OnaProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		runner.NewSingularDataSource,
		runner.NewCollectionDataSource,
		warmpool.NewWarmPoolDataSource,
		warmpool.NewWarmPoolCollectionDataSource,
		security.NewPolicyCollectionDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &OnaProvider{
			version: version,
		}
	}
}
