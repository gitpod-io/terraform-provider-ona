// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	onaclient "github.com/ona/terraform-provider-ona/internal/client"
)

// Ensure OnaProvider satisfies various provider interfaces.
var _ provider.Provider = &OnaProvider{}

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
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				MarkdownDescription: "Ona host. Defaults to `ONA_HOST` when set, otherwise `https://app.gitpod.io`.",
				Optional:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "Ona API token. Defaults to `ONA_TOKEN` when set. Use service-account tokens for Terraform automation.",
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

	api, _, err := onaclient.NewSDK(cfg)
	if err != nil {
		if !errors.Is(err, onaclient.ErrMissingToken) {
			resp.Diagnostics.AddError("Unable to Configure Ona API Client", err.Error())
			return
		}
	}

	resp.DataSourceData = http.DefaultClient
	resp.ResourceData = api
}

func (p *OnaProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRunnerResource,
	}
}

func pathRoot(name string) path.Path {
	return path.Root(name)
}

func (p *OnaProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return nil
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &OnaProvider{
			version: version,
		}
	}
}
