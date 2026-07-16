// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package webhook

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	ephemeralschema "github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ ephemeral.EphemeralResource = &SecretEphemeralResource{}
var _ ephemeral.EphemeralResourceWithConfigure = &SecretEphemeralResource{}

func NewSecretEphemeralResource() ephemeral.EphemeralResource {
	return &SecretEphemeralResource{}
}

type SecretEphemeralResource struct {
	client *managementclient.ManagementPlane
}

func (r *SecretEphemeralResource) Metadata(ctx context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook_secret"
}

func (r *SecretEphemeralResource) Schema(ctx context.Context, req ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = ephemeralschema.Schema{
		MarkdownDescription: "Retrieves an Ona webhook signing secret without storing it in Terraform plan or state. Secret access is audited by Ona and requires permission to update the webhook. Reference `secret` only from ephemeral contexts, write-only arguments, or child module ephemeral outputs.",
		Attributes: map[string]ephemeralschema.Attribute{
			"webhook_id": ephemeralschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Webhook ID whose current signing secret should be retrieved.",
			},
			"secret": ephemeralschema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Current webhook signing secret. This value is not stored in Terraform plan or state.",
			},
		},
	}
}

func (r *SecretEphemeralResource) Configure(ctx context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
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
	r.client = data.Client
}

func (r *SecretEphemeralResource) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var data SecretModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before opening ona_webhook_secret ephemeral resources.",
		)
		return
	}
	if data.WebhookID.IsNull() || data.WebhookID.IsUnknown() || data.WebhookID.ValueString() == "" {
		resp.Diagnostics.AddError("Missing Ona Webhook ID", "webhook_id must be known and non-empty before opening ona_webhook_secret.")
		return
	}

	result, err := r.client.WebhookService().GetWebhookSecret(ctx, connect.NewRequest(&v1.GetWebhookSecretRequest{
		WebhookId: data.WebhookID.ValueString(),
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Webhook Secret", "retrieving the Ona webhook signing secret", err)
		return
	}
	data.Secret = types.StringValue(result.Msg.GetSecret())
	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}
