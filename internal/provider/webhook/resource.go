// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package webhook

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithConfigure = &Resource{}
var _ resource.ResourceWithImportState = &Resource{}
var _ resource.ResourceWithValidateConfig = &Resource{}

func NewResource() resource.Resource {
	return &Resource{}
}

type Resource struct {
	client *managementclient.ManagementPlane
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
}

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateModel(ctx, data, false, &resp.Diagnostics)
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", "ona_webhook") {
		return
	}

	createReq, diags := createWebhookRequest(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.WebhookService().CreateWebhook(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Webhook", "creating the Ona webhook", err)
		return
	}
	webhook := result.Msg.GetWebhook()
	if webhook.GetId() == "" {
		resp.Diagnostics.AddError("Unable to Create Ona Webhook", "The Ona API returned a created webhook without an ID.")
		return
	}

	data.ID = types.StringValue(webhook.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planned := data
	populateModel(ctx, &data, webhook, &resp.Diagnostics)
	preservePlannedInputs(&data, planned)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data Model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", "ona_webhook") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Webhook", "Webhook ID is empty.")
		return
	}

	webhook, err := r.getWebhook(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Webhook", "reading the Ona webhook", err)
		return
	}
	if webhook == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	prior := data
	data = Model{}
	populateModel(ctx, &data, webhook, &resp.Diagnostics)
	preserveTerraformOnlyState(&data, prior)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var prior Model
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "updating", "ona_webhook") {
		return
	}

	updateReq, diags := updateWebhookRequest(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.WebhookService().UpdateWebhook(ctx, connect.NewRequest(updateReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Webhook", "updating the Ona webhook", err)
		return
	}
	webhook := result.Msg.GetWebhook()
	if webhook == nil {
		resp.Diagnostics.AddError("Unable to Update Ona Webhook", "The Ona API returned an empty updated webhook.")
		return
	}

	if !data.SecretVersion.Equal(prior.SecretVersion) {
		if _, err := r.client.WebhookService().RotateWebhookSecret(ctx, connect.NewRequest(&v1.RotateWebhookSecretRequest{
			WebhookId: data.ID.ValueString(),
		})); err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Rotate Ona Webhook Secret", "rotating the Ona webhook signing secret", err)
			return
		}
	}

	planned := data
	populateModel(ctx, &data, webhook, &resp.Diagnostics)
	preservePlannedInputs(&data, planned)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data Model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", "ona_webhook") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	result, err := r.client.WebhookService().DeleteWebhook(ctx, connect.NewRequest(&v1.DeleteWebhookRequest{
		WebhookId: data.ID.ValueString(),
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Webhook", "deleting the Ona webhook", err)
		return
	}
	if err == nil && len(result.Msg.GetAffectedWorkflowIds()) > 0 {
		resp.Diagnostics.AddWarning(
			"Deleted Ona Webhook With Bound Workflows",
			"Ona converted triggers on these workflows to manual because they referenced the deleted webhook: "+strings.Join(result.Msg.GetAffectedWorkflowIds(), ", ")+".",
		)
	}
	resp.State.RemoveResource(ctx)
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *Resource) getWebhook(ctx context.Context, id string) (*v1.Webhook, error) {
	result, err := r.client.WebhookService().GetWebhook(ctx, connect.NewRequest(&v1.GetWebhookRequest{WebhookId: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get webhook: %w", err)
	}
	return result.Msg.GetWebhook(), nil
}
