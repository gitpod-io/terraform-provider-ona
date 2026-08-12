// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &TokenResource{}
var _ resource.ResourceWithConfigure = &TokenResource{}

func NewTokenResource() resource.Resource {
	return &TokenResource{}
}

type TokenResource struct {
	client *managementclient.ManagementPlane
}

type TokenModel struct {
	ID           types.String `tfsdk:"id"`
	RunnerID     types.String `tfsdk:"runner_id"`
	TokenVersion types.String `tfsdk:"token_version"`
	Token        types.String `tfsdk:"token"`
}

func (r *TokenResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_token"
}

func (r *TokenResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Creates an Ona runner registration token and stores it in Terraform state. The token expires after 24 hours and can only be used once. Terraform does not rotate it automatically; change `token_version` deliberately when consumers need a new token. The token is sensitive, which redacts it from normal CLI output but does not remove it from state. Use an encrypted, access-controlled remote state backend.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Stable local Terraform resource ID. Ona does not expose a durable token object ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"runner_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID for which to create a registration exchange token. Changing this value replaces the resource and mints a new token.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"token_version": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "User-managed rotation marker. Changing this value replaces the resource and mints a new runner registration token. Ona does not rotate the token automatically when its 24-hour lifetime expires.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"token": resourceschema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Runner registration exchange token. This sensitive value is redacted from normal Terraform output but persists in Terraform state so ordinary resources and module inputs can consume it. It expires after 24 hours and can only be used once. Store state in an encrypted, access-controlled remote backend.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *TokenResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
}

func (r *TokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", "ona_runner_token") {
		return
	}

	result, err := r.client.RunnerService().CreateRunnerToken(ctx, connect.NewRequest(&v1.CreateRunnerTokenRequest{
		RunnerId: data.RunnerID.ValueString(),
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Runner Token", "creating an Ona runner registration token", err)
		return
	}
	if result.Msg.GetExchangeToken() == "" {
		resp.Diagnostics.AddError("Unable to Create Ona Runner Token", "The Ona API returned an empty runner registration token.")
		return
	}

	data.ID = types.StringValue(uuid.NewString())
	data.Token = types.StringValue(result.Msg.GetExchangeToken())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Unable to Update Ona Runner Token",
		"Runner tokens cannot be updated. Changes to runner_id or token_version must replace the resource.",
	)
}

func (r *TokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}
