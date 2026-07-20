// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package serviceaccount

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	ephemeralschema "github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
)

var _ ephemeral.EphemeralResource = &TokenEphemeralResource{}
var _ ephemeral.EphemeralResourceWithConfigure = &TokenEphemeralResource{}
var _ ephemeral.EphemeralResourceWithClose = &TokenEphemeralResource{}
var _ ephemeral.EphemeralResourceWithValidateConfig = &TokenEphemeralResource{}

func NewTokenEphemeralResource() ephemeral.EphemeralResource {
	return &TokenEphemeralResource{}
}

type TokenEphemeralResource struct {
	data *providerdata.Data
}

type TokenEphemeralModel struct {
	ServiceAccountID types.String `tfsdk:"service_account_id"`
	Description      types.String `tfsdk:"description"`
	ValidFor         types.String `tfsdk:"valid_for"`
	Token            types.String `tfsdk:"token"`
	ExpiresAt        types.String `tfsdk:"expires_at"`
}

func (r *TokenEphemeralResource) Metadata(ctx context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account_token"
}

func (r *TokenEphemeralResource) Schema(ctx context.Context, req ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = ephemeralschema.Schema{
		MarkdownDescription: "Creates an Ona service-account token without storing the token value in Terraform plan or state. Use this in bootstrap or rotation workflows with a human or admin token, then store the returned token in an external secret target. Use the token as `ONA_TOKEN` only for supported service-account-token workflows or when Ona has confirmed write permissions for your organization and use case. Reference the token only from Terraform ephemeral contexts such as provider configurations, write-only arguments, or child module ephemeral outputs.",
		Attributes: map[string]ephemeralschema.Attribute{
			"service_account_id": ephemeralschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Service account ID for which to create a token.",
			},
			"description": ephemeralschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Service account token description shown in Ona token metadata.",
			},
			"valid_for": ephemeralschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "How long the token should be valid, as a non-negative Go duration string such as `720h`, `2160h`, or `8760h`. Omit to use the Ona API default. The API caps validity to the service account expiry.",
			},
			"token": ephemeralschema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Service-account token value. This value is returned once and is not stored in Terraform plan or state.",
			},
			"expires_at": ephemeralschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the service-account token expires.",
			},
		},
	}
}

func (r *TokenEphemeralResource) Configure(ctx context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerdata.Data)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Ephemeral Resource Configure Type",
			fmt.Sprintf("Expected *providerdata.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.data = data
}

func (r *TokenEphemeralResource) ValidateConfig(ctx context.Context, req ephemeral.ValidateConfigRequest, resp *ephemeral.ValidateConfigResponse) {
	var validFor types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("valid_for"), &validFor)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateServiceAccountTokenValidFor(validFor, &resp.Diagnostics)
}

func (r *TokenEphemeralResource) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var data TokenEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.data == nil || r.data.Client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before opening ona_service_account_token ephemeral resources.",
		)
		return
	}
	if r.data.APIBaseURL == "" {
		resp.Diagnostics.AddError(
			"Ona API Base URL Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before opening ona_service_account_token ephemeral resources.",
		)
		return
	}

	createReq, diags := createServiceAccountTokenRequest(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accessToken, err := r.data.Client.ServiceAccountService().CreateServiceAccountAccessToken(ctx, connect.NewRequest(&v1.CreateServiceAccountAccessTokenRequest{
		ServiceAccountId: data.ServiceAccountID.ValueString(),
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Service Account Access Token", "creating a temporary access token for service-account token bootstrap", err)
		return
	}

	impersonated, err := managementclient.New(r.data.APIBaseURL,
		managementclient.WithAccessToken(accessToken.Msg.GetToken()),
		managementclient.WithUserAgent(r.data.UserAgent),
	)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Configure Impersonated Ona API Client", "configuring the impersonated service-account API client", err)
		return
	}

	result, err := impersonated.ServiceAccountService().CreateServiceAccountToken(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Service Account Token", "creating the Ona service-account token with the impersonated service account", err)
		return
	}

	data.Token = types.StringValue(result.Msg.GetToken())
	if token := result.Msg.GetServiceAccountToken(); token != nil {
		data.ExpiresAt = timestampValue(token.GetExpiresAt())
	}
	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}

func (r *TokenEphemeralResource) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
}

func createServiceAccountTokenRequest(data TokenEphemeralModel) (*v1.CreateServiceAccountTokenRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	req := &v1.CreateServiceAccountTokenRequest{}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		req.Description = data.Description.ValueString()
	}
	if !data.ValidFor.IsNull() && !data.ValidFor.IsUnknown() {
		duration, err := serviceAccountTokenValidFor(data.ValidFor)
		if err != nil {
			diags.AddAttributeError(path.Root("valid_for"), "Invalid Service Account Token Validity", err.Error())
			return nil, diags
		}
		req.ValidFor = durationpb.New(duration)
	}
	return req, diags
}

func validateServiceAccountTokenValidFor(value types.String, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	if _, err := serviceAccountTokenValidFor(value); err != nil {
		diags.AddAttributeError(path.Root("valid_for"), "Invalid Service Account Token Validity", err.Error())
	}
}

func serviceAccountTokenValidFor(value types.String) (time.Duration, error) {
	duration, err := time.ParseDuration(value.ValueString())
	if err != nil {
		return 0, fmt.Errorf("valid_for must be a Go duration string: %w", err)
	}
	if duration < 0 {
		return 0, fmt.Errorf("valid_for must not be negative")
	}
	return duration, nil
}
