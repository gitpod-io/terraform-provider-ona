// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/api/go/client"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithConfigure = &Resource{}
var _ resource.ResourceWithImportState = &Resource{}
var _ resource.ResourceWithModifyPlan = &Resource{}
var _ resource.ResourceWithValidateConfig = &Resource{}

func NewResource() resource.Resource {
	return &Resource{}
}

type Resource struct {
	client *managementclient.ManagementPlane
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (r *Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateConfig(ctx, data, &resp.Diagnostics)
}

func (r *Resource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var state Model
	var plan Model
	var config Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	isCustom := state.IntegrationDefinitionID.IsNull() || state.IntegrationDefinitionID.ValueString() == ""
	if isCustom && !plan.Capabilities.Equal(state.Capabilities) {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("capabilities"))
	}
	if isCustom && !plan.Auth.Equal(state.Auth) {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("auth"))
	}
	if !config.Categories.IsNull() && !config.Categories.IsUnknown() && len(config.Categories.Elements()) == 0 && !state.Categories.IsNull() && len(state.Categories.Elements()) > 0 {
		if isCustom {
			resp.RequiresReplace = append(resp.RequiresReplace, path.Root("categories"))
		} else {
			resp.Diagnostics.AddAttributeError(path.Root("categories"), "Unable to Clear Definition-Backed Integration Categories", "The Ona API cannot clear category overrides in place, and recreating a definition-backed integration immediately re-inherits its definition categories. Configure a non-empty category set or omit the change.")
		}
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data Model
	var config Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "creating") {
		return
	}

	createReq, diags := createIntegrationRequest(ctx, data, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.IntegrationService().CreateIntegration(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Integration", "creating the Ona integration", err)
		return
	}
	integration := result.Msg.GetIntegration()
	if integration == nil || integration.GetId() == "" {
		resp.Diagnostics.AddError("Unable to Create Ona Integration", "The Ona API returned a created integration without an ID.")
		return
	}

	data.ID = types.StringValue(integration.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err = r.getIntegration(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Created Ona Integration", "reading the created Ona integration", err)
		return
	}
	if integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	planned := data
	resp.Diagnostics.Append(populateModel(ctx, &data, integration, planned.Auth)...)
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
	if !r.requireClient(&resp.Diagnostics, "reading") {
		return
	}
	if !isKnownString(data.ID) {
		resp.Diagnostics.AddError("Unable to Read Ona Integration", "Integration ID is empty.")
		return
	}

	integration, err := r.getIntegration(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Integration", "reading the Ona integration", err)
		return
	}
	if integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	priorAuth := data.Auth
	data = Model{}
	resp.Diagnostics.Append(populateModel(ctx, &data, integration, priorAuth)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data Model
	var state Model
	var config Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "updating") {
		return
	}

	updateReq, diags := updateIntegrationRequest(ctx, data, state, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.IntegrationService().UpdateIntegration(ctx, connect.NewRequest(updateReq)); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Integration", "updating the Ona integration", err)
		return
	}

	integration, err := r.getIntegration(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona Integration", "reading the updated Ona integration", err)
		return
	}
	if integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	planned := data
	resp.Diagnostics.Append(populateModel(ctx, &data, integration, planned.Auth)...)
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
	if !r.requireClient(&resp.Diagnostics, "deleting") {
		return
	}
	if !isKnownString(data.ID) {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.IntegrationService().DeleteIntegration(ctx, connect.NewRequest(&v1.DeleteIntegrationRequest{Id: data.ID.ValueString()}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Integration", "deleting the Ona integration", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *Resource) getIntegration(ctx context.Context, id string) (*v1.Integration, error) {
	result, err := r.client.IntegrationService().GetIntegration(ctx, connect.NewRequest(&v1.GetIntegrationRequest{Id: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get integration: %w", err)
	}
	return result.Msg.GetIntegration(), nil
}

func (r *Resource) requireClient(diags *diag.Diagnostics, action string) bool {
	if r.client != nil {
		return true
	}
	diags.AddError("Ona API Client Is Not Configured", fmt.Sprintf("Set the provider token argument or ONA_TOKEN before %s ona_integration resources.", action))
	return false
}
