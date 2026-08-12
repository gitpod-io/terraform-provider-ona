// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package billing

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/tfvalue"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const organizationAIBudgetResourceType = "ona_organization_ai_budget"

var _ resource.Resource = &OrganizationAIBudgetResource{}
var _ resource.ResourceWithConfigure = &OrganizationAIBudgetResource{}
var _ resource.ResourceWithImportState = &OrganizationAIBudgetResource{}
var _ resource.ResourceWithValidateConfig = &OrganizationAIBudgetResource{}

func NewOrganizationAIBudgetResource() resource.Resource { return &OrganizationAIBudgetResource{} }

type OrganizationAIBudgetResource struct {
	client *managementclient.ManagementPlane
}

func (r *OrganizationAIBudgetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_ai_budget"
}

func (r *OrganizationAIBudgetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = organizationAIBudgetSchema()
}

func (r *OrganizationAIBudgetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
}

func (r *OrganizationAIBudgetResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data OrganizationAIBudgetModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateOrganizationBudget(data, false, &resp.Diagnostics)
}

func (r *OrganizationAIBudgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationAIBudgetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateOrganizationBudget(data, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", organizationAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	setReq, err := organizationPolicySetRequest(organizationID, data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Map Ona Organization AI Budget", err.Error())
		return
	}
	result, err := r.client.BillingService().SetEnterpriseAIUserBudgetPolicy(ctx, connect.NewRequest(setReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Organization AI Budget", "setting the organization AI budget", err)
		return
	}
	if result == nil || result.Msg == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Organization AI Budget", "The Ona API returned an empty response.")
		return
	}
	if err := populateOrganizationBudget(&data, result.Msg.GetPolicy(), organizationID); err != nil {
		resp.Diagnostics.AddError("Unable to Map Created Ona Organization AI Budget", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.refresh(ctx, &data, organizationID, &resp.Diagnostics, func() { resp.State.RemoveResource(ctx) })
	if resp.Diagnostics.HasError() || data.ID.IsNull() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationAIBudgetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationAIBudgetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", organizationAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	if !guardOrganizationScope(&resp.Diagnostics, data.OrganizationID, organizationID, organizationAIBudgetResourceType) {
		return
	}
	r.refresh(ctx, &data, organizationID, &resp.Diagnostics, func() { resp.State.RemoveResource(ctx) })
	if resp.Diagnostics.HasError() {
		return
	}
	if data.ID.IsNull() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationAIBudgetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationAIBudgetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateOrganizationBudget(data, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "updating", organizationAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	setReq, err := organizationPolicySetRequest(organizationID, data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Map Ona Organization AI Budget", err.Error())
		return
	}
	if _, err := r.client.BillingService().SetEnterpriseAIUserBudgetPolicy(ctx, connect.NewRequest(setReq)); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Organization AI Budget", "updating the organization AI budget", err)
		return
	}
	r.refresh(ctx, &data, organizationID, &resp.Diagnostics, func() { resp.State.RemoveResource(ctx) })
	if resp.Diagnostics.HasError() || data.ID.IsNull() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationAIBudgetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrganizationAIBudgetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", organizationAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	if !guardOrganizationScope(&resp.Diagnostics, data.OrganizationID, organizationID, organizationAIBudgetResourceType) {
		return
	}
	mode, err := modeToProto(data.Mode.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Delete Ona Organization AI Budget", err.Error())
		return
	}
	_, err = r.client.BillingService().DeleteEnterpriseAIUserBudgetPolicy(ctx, connect.NewRequest(&v1.DeleteEnterpriseAIUserBudgetPolicyRequest{OrganizationId: organizationID, Mode: mode}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Organization AI Budget", "deleting the organization AI budget", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *OrganizationAIBudgetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "importing", organizationAIBudgetResourceType) {
		return
	}
	parts, diags := tfvalue.SplitImportID(req.ID, 2, "organization_id/mode")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	if parts[0] != organizationID {
		resp.Diagnostics.AddError("Invalid Import Organization", "The import organization must match the organization authenticated by the provider token.")
		return
	}
	if _, err := modeToProto(parts[1]); err != nil {
		resp.Diagnostics.AddError("Invalid Import Mode", err.Error())
		return
	}
	tfvalue.SetImportString(ctx, resp, "organization_id", parts[0])
	tfvalue.SetImportString(ctx, resp, "mode", parts[1])
}

func (r *OrganizationAIBudgetResource) refresh(ctx context.Context, data *OrganizationAIBudgetModel, organizationID string, diags *diag.Diagnostics, remove func()) {
	mode, err := modeToProto(data.Mode.ValueString())
	if err != nil {
		diags.AddError("Unable to Read Ona Organization AI Budget", err.Error())
		return
	}
	result, err := r.client.BillingService().GetEnterpriseAIUserBudgetPolicy(ctx, connect.NewRequest(&v1.GetEnterpriseAIUserBudgetPolicyRequest{OrganizationId: organizationID, Mode: mode}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			*data = OrganizationAIBudgetModel{}
			remove()
			return
		}
		providerdiag.AddAPIError(diags, "Unable to Read Ona Organization AI Budget", "reading the organization AI budget", err)
		return
	}
	if result == nil || result.Msg == nil || result.Msg.GetPolicy() == nil {
		*data = OrganizationAIBudgetModel{}
		remove()
		return
	}
	if err := populateOrganizationBudget(data, result.Msg.GetPolicy(), organizationID); err != nil {
		diags.AddError("Unable to Map Ona Organization AI Budget", err.Error())
	}
}
