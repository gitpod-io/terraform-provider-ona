// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package billing

import (
	"context"

	"connectrpc.com/connect"
	onaclient "github.com/gitpod-io/terraform-provider-ona/internal/client"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/tfvalue"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const userAIBudgetResourceType = "ona_user_ai_budget"

var _ resource.Resource = &UserAIBudgetResource{}
var _ resource.ResourceWithConfigure = &UserAIBudgetResource{}
var _ resource.ResourceWithImportState = &UserAIBudgetResource{}
var _ resource.ResourceWithValidateConfig = &UserAIBudgetResource{}

func NewUserAIBudgetResource() resource.Resource { return &UserAIBudgetResource{} }

type UserAIBudgetResource struct {
	client *managementclient.ManagementPlane
	raw    *onaclient.Client
}

func (r *UserAIBudgetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_ai_budget"
}

func (r *UserAIBudgetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = userAIBudgetSchema()
}

func (r *UserAIBudgetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
	r.raw = providerdata.ResourceRawClient(req.ProviderData, r.raw, &resp.Diagnostics)
}

func (r *UserAIBudgetResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data UserAIBudgetModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateUserBudget(data, false, &resp.Diagnostics)
}

func (r *UserAIBudgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserAIBudgetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateUserBudget(data, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() ||
		!providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", userAIBudgetResourceType) ||
		!providerdata.RequireResourceRawClient(r.raw, &resp.Diagnostics, "creating", userAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	setReq, err := userPolicySetRequest(organizationID, data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Map Ona User AI Budget", err.Error())
		return
	}
	policy, err := setEnterpriseAIUserBudgetPolicy(ctx, r.raw, setReq)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona User AI Budget", "setting the user AI budget override", err)
		return
	}
	if policy == nil {
		resp.Diagnostics.AddError("Unable to Create Ona User AI Budget", "The Ona API returned an empty response.")
		return
	}
	if err := populateUserBudget(&data, policy, organizationID, data.UserID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to Map Created Ona User AI Budget", err.Error())
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

func (r *UserAIBudgetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserAIBudgetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() ||
		!providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", userAIBudgetResourceType) ||
		!providerdata.RequireResourceRawClient(r.raw, &resp.Diagnostics, "reading", userAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	if !guardOrganizationScope(&resp.Diagnostics, data.OrganizationID, organizationID, userAIBudgetResourceType) {
		return
	}
	r.refresh(ctx, &data, organizationID, &resp.Diagnostics, func() { resp.State.RemoveResource(ctx) })
	if resp.Diagnostics.HasError() || data.ID.IsNull() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserAIBudgetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserAIBudgetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateUserBudget(data, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() ||
		!providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "updating", userAIBudgetResourceType) ||
		!providerdata.RequireResourceRawClient(r.raw, &resp.Diagnostics, "updating", userAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	setReq, err := userPolicySetRequest(organizationID, data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Map Ona User AI Budget", err.Error())
		return
	}
	if _, err := setEnterpriseAIUserBudgetPolicy(ctx, r.raw, setReq); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona User AI Budget", "updating the user AI budget override", err)
		return
	}
	r.refresh(ctx, &data, organizationID, &resp.Diagnostics, func() { resp.State.RemoveResource(ctx) })
	if resp.Diagnostics.HasError() || data.ID.IsNull() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserAIBudgetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserAIBudgetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() ||
		!providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", userAIBudgetResourceType) ||
		!providerdata.RequireResourceRawClient(r.raw, &resp.Diagnostics, "deleting", userAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	if !guardOrganizationScope(&resp.Diagnostics, data.OrganizationID, organizationID, userAIBudgetResourceType) {
		return
	}
	mode, err := modeToProto(data.Mode.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Delete Ona User AI Budget", err.Error())
		return
	}
	userID := data.UserID.ValueString()
	err = deleteEnterpriseAIUserBudgetPolicy(ctx, r.raw, &enterpriseAIUserBudgetPolicyRequest{OrganizationID: organizationID, Mode: mode.String(), UserID: &userID})
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona User AI Budget", "deleting the user AI budget override", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *UserAIBudgetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "importing", userAIBudgetResourceType) {
		return
	}
	parts, diags := tfvalue.SplitImportID(req.ID, 3, "organization_id/user_id/mode")
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
	if _, err := modeToProto(parts[2]); err != nil {
		resp.Diagnostics.AddError("Invalid Import Mode", err.Error())
		return
	}
	var validation diag.Diagnostics
	validateUUID(types.StringValue(parts[1]), path.Root("user_id"), true, &validation)
	resp.Diagnostics.Append(validation...)
	if resp.Diagnostics.HasError() {
		return
	}
	tfvalue.SetImportString(ctx, resp, "organization_id", parts[0])
	tfvalue.SetImportString(ctx, resp, "user_id", parts[1])
	tfvalue.SetImportString(ctx, resp, "mode", parts[2])
}

func (r *UserAIBudgetResource) refresh(ctx context.Context, data *UserAIBudgetModel, organizationID string, diags *diag.Diagnostics, remove func()) {
	mode, err := modeToProto(data.Mode.ValueString())
	if err != nil {
		diags.AddError("Unable to Read Ona User AI Budget", err.Error())
		return
	}
	userID := data.UserID.ValueString()
	policy, err := getEnterpriseAIUserBudgetPolicy(ctx, r.raw, &enterpriseAIUserBudgetPolicyRequest{OrganizationID: organizationID, Mode: mode.String(), UserID: &userID})
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			*data = UserAIBudgetModel{}
			remove()
			return
		}
		providerdiag.AddAPIError(diags, "Unable to Read Ona User AI Budget", "reading the user AI budget override", err)
		return
	}
	if policy == nil {
		*data = UserAIBudgetModel{}
		remove()
		return
	}
	if err := populateUserBudget(data, policy, organizationID, userID); err != nil {
		diags.AddError("Unable to Map Ona User AI Budget", err.Error())
	}
}
