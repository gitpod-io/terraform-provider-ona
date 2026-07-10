// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package warmpool

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/api/go/client"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &WarmPoolResource{}
var _ resource.ResourceWithConfigure = &WarmPoolResource{}
var _ resource.ResourceWithImportState = &WarmPoolResource{}
var _ resource.ResourceWithValidateConfig = &WarmPoolResource{}

func NewWarmPoolResource() resource.Resource {
	return &WarmPoolResource{}
}

type WarmPoolResource struct {
	client *managementclient.ManagementPlane
}

func (r *WarmPoolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warm_pool"
}

func (r *WarmPoolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = warmPoolResourceSchema()
}

func (r *WarmPoolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerdata.Data)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *providerdata.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = data.Client
}

func (r *WarmPoolResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data WarmPoolModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateWarmPoolConfig(data, &resp.Diagnostics)
}

func (r *WarmPoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WarmPoolModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before creating ona_warm_pool resources.",
		)
		return
	}

	createReq, diags := createWarmPoolRequest(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.PrebuildService().CreateWarmPool(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Warm Pool", "creating the Ona warm pool", err)
		return
	}
	if result.Msg.GetWarmPool() == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Warm Pool", "The Ona API returned an empty warm pool.")
		return
	}

	data.ID = types.StringValue(result.Msg.GetWarmPool().GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	warmPool, err := r.getWarmPool(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Created Ona Warm Pool", "reading the created Ona warm pool", err)
		return
	}
	if warmPoolIsDeleted(warmPool) {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	populateWarmPoolModel(&data, warmPool)
	preserveWarmPoolPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WarmPoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WarmPoolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_warm_pool resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Warm Pool", "Warm pool ID is empty.")
		return
	}

	warmPool, err := r.getWarmPool(ctx, id)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Warm Pool", "reading the Ona warm pool", err)
		return
	}
	if warmPoolIsDeleted(warmPool) {
		resp.State.RemoveResource(ctx)
		return
	}

	data = WarmPoolModel{}
	populateWarmPoolModel(&data, warmPool)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WarmPoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WarmPoolModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before updating ona_warm_pool resources.",
		)
		return
	}

	updateReq, diags := updateWarmPoolRequest(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.PrebuildService().UpdateWarmPool(ctx, connect.NewRequest(updateReq)); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Warm Pool", "updating the Ona warm pool", err)
		return
	}

	warmPool, err := r.getWarmPool(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona Warm Pool", "reading the updated Ona warm pool", err)
		return
	}
	if warmPoolIsDeleted(warmPool) {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	populateWarmPoolModel(&data, warmPool)
	preserveWarmPoolPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WarmPoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WarmPoolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before deleting ona_warm_pool resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.PrebuildService().DeleteWarmPool(ctx, connect.NewRequest(&v1.DeleteWarmPoolRequest{WarmPoolId: id}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Warm Pool", "deleting the Ona warm pool", err)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *WarmPoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *WarmPoolResource) getWarmPool(ctx context.Context, id string) (*v1.WarmPool, error) {
	result, err := r.client.PrebuildService().GetWarmPool(ctx, connect.NewRequest(&v1.GetWarmPoolRequest{WarmPoolId: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get warm pool: %w", err)
	}
	return result.Msg.GetWarmPool(), nil
}
