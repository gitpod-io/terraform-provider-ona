// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	ephemeralschema "github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ ephemeral.EphemeralResource = &TokenEphemeralResource{}
var _ ephemeral.EphemeralResourceWithConfigure = &TokenEphemeralResource{}
var _ ephemeral.EphemeralResourceWithClose = &TokenEphemeralResource{}

func NewTokenEphemeralResource() ephemeral.EphemeralResource {
	return &TokenEphemeralResource{}
}

type TokenEphemeralResource struct {
	client *managementclient.ManagementPlane
}

type TokenEphemeralModel struct {
	RunnerID types.String `tfsdk:"runner_id"`
	Token    types.String `tfsdk:"token"`
}

func (r *TokenEphemeralResource) Metadata(ctx context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_token"
}

func (r *TokenEphemeralResource) Schema(ctx context.Context, req ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = ephemeralschema.Schema{
		MarkdownDescription: "Creates a short-lived Ona runner registration token without storing it in Terraform plan or state. Use it only from Terraform ephemeral contexts such as provider configurations, write-only arguments, or child module ephemeral outputs.",
		Attributes: map[string]ephemeralschema.Attribute{
			"runner_id": ephemeralschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID for which to create a registration exchange token.",
			},
			"token": ephemeralschema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Short-lived runner exchange token. This token expires after 24 hours and is not stored in Terraform plan or state.",
			},
		},
	}
}

func (r *TokenEphemeralResource) Configure(ctx context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	data := providerdata.EphemeralResourceData(req.ProviderData, nil, &resp.Diagnostics)
	if data != nil {
		r.client = data.Client
	}
}

func (r *TokenEphemeralResource) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var data TokenEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !providerdata.RequireEphemeralResourceClient(r.client, &resp.Diagnostics, "ona_runner_token") {
		return
	}

	result, err := r.client.RunnerService().CreateRunnerToken(ctx, connect.NewRequest(&v1.CreateRunnerTokenRequest{
		RunnerId: data.RunnerID.ValueString(),
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Runner Token", "creating a short-lived Ona runner registration token", err)
		return
	}

	data.Token = types.StringValue(result.Msg.GetExchangeToken())
	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}

func (r *TokenEphemeralResource) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
}
