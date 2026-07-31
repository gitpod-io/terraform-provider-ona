// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package billing

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
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

const teamAIBudgetResourceType = "ona_team_ai_budget"

var _ resource.Resource = &TeamAIBudgetResource{}
var _ resource.ResourceWithConfigure = &TeamAIBudgetResource{}
var _ resource.ResourceWithImportState = &TeamAIBudgetResource{}
var _ resource.ResourceWithValidateConfig = &TeamAIBudgetResource{}

func NewTeamAIBudgetResource() resource.Resource { return &TeamAIBudgetResource{} }

type TeamAIBudgetResource struct {
	client *managementclient.ManagementPlane
	raw    *onaclient.Client
}

func (r *TeamAIBudgetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_ai_budget"
}

func (r *TeamAIBudgetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = teamAIBudgetSchema()
}

func (r *TeamAIBudgetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
	r.raw = providerdata.ResourceRawClient(req.ProviderData, r.raw, &resp.Diagnostics)
}

func (r *TeamAIBudgetResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data TeamAIBudgetModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateTeamBudget(data, false, &resp.Diagnostics)
}

func (r *TeamAIBudgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamAIBudgetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateTeamBudget(data, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() ||
		!providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", teamAIBudgetResourceType) ||
		!providerdata.RequireResourceRawClient(r.raw, &resp.Diagnostics, "creating", teamAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}

	allocation, err := r.getTeamAllocation(ctx, organizationID, data.TeamID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Inspect Ona Team AI Budget", "checking for an existing team allocation", err)
		return
	}
	if teamModeConfigured(allocation, data.Mode.ValueString()) {
		resp.Diagnostics.AddError("Ona Team AI Budget Already Exists", fmt.Sprintf("A %s budget already exists for this team. Import it with organization_id/team_id/%s instead of creating another resource.", data.Mode.ValueString(), data.Mode.ValueString()))
		return
	}

	mutated, err := r.addTeamBudget(ctx, organizationID, data, allocation)
	if connect.CodeOf(err) == connect.CodeAlreadyExists && allocation == nil {
		allocation, err = r.getTeamAllocation(ctx, organizationID, data.TeamID.ValueString())
		if err == nil && teamModeConfigured(allocation, data.Mode.ValueString()) {
			resp.Diagnostics.AddError("Ona Team AI Budget Already Exists", fmt.Sprintf("A concurrent request created the %s budget. Import it with organization_id/team_id/%s.", data.Mode.ValueString(), data.Mode.ValueString()))
			return
		}
		if err == nil {
			mutated, err = r.addTeamBudget(ctx, organizationID, data, allocation)
		}
	}
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Team AI Budget", "creating the team AI budget", err)
		return
	}
	if mutated == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Team AI Budget", "The Ona API returned an empty team allocation.")
		return
	}
	if exists, err := populateTeamBudget(&data, mutated, organizationID, data.TeamID.ValueString(), data.Mode.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to Map Created Ona Team AI Budget", err.Error())
		return
	} else if !exists {
		resp.Diagnostics.AddError("Unable to Map Created Ona Team AI Budget", "The Ona API response did not contain the selected budget mode.")
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

func (r *TeamAIBudgetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamAIBudgetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() ||
		!providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", teamAIBudgetResourceType) ||
		!providerdata.RequireResourceRawClient(r.raw, &resp.Diagnostics, "reading", teamAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	if !guardOrganizationScope(&resp.Diagnostics, data.OrganizationID, organizationID, teamAIBudgetResourceType) {
		return
	}
	r.refresh(ctx, &data, organizationID, &resp.Diagnostics, func() { resp.State.RemoveResource(ctx) })
	if resp.Diagnostics.HasError() || data.ID.IsNull() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamAIBudgetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TeamAIBudgetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateTeamBudget(data, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() ||
		!providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "updating", teamAIBudgetResourceType) ||
		!providerdata.RequireResourceRawClient(r.raw, &resp.Diagnostics, "updating", teamAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	updateReq, err := updateTeamBudgetRequest(organizationID, data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Map Ona Team AI Budget", err.Error())
		return
	}
	if _, err := updateTeamCreditAllocation(ctx, r.raw, updateReq); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Team AI Budget", "updating the team AI budget", err)
		return
	}
	r.refresh(ctx, &data, organizationID, &resp.Diagnostics, func() { resp.State.RemoveResource(ctx) })
	if resp.Diagnostics.HasError() || data.ID.IsNull() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamAIBudgetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamAIBudgetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() ||
		!providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", teamAIBudgetResourceType) ||
		!providerdata.RequireResourceRawClient(r.raw, &resp.Diagnostics, "deleting", teamAIBudgetResourceType) {
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the authenticated organization", err)
		return
	}
	if !guardOrganizationScope(&resp.Diagnostics, data.OrganizationID, organizationID, teamAIBudgetResourceType) {
		return
	}

	teamID := data.TeamID.ValueString()
	switch data.Mode.ValueString() {
	case modeCredits:
		err = deleteTeamCreditAllocation(ctx, r.raw, &teamCreditAllocationRequest{OrganizationID: organizationID, TeamID: teamID})
	case modeBYOK:
		var allocation *v1.TeamCreditAllocationInfo
		allocation, err = r.getTeamAllocation(ctx, organizationID, teamID)
		if err == nil && (allocation == nil || allocation.CostBudgetMicrounits == nil) {
			resp.State.RemoveResource(ctx)
			return
		}
		if err == nil && allocation.GetCreditBudget() > 0 {
			_, err = updateTeamCreditAllocation(ctx, r.raw, clearTeamBYOKRequest(organizationID, teamID))
		} else if err == nil {
			err = deleteTeamCreditAllocation(ctx, r.raw, &teamCreditAllocationRequest{OrganizationID: organizationID, TeamID: teamID})
		}
	default:
		resp.Diagnostics.AddError("Unable to Delete Ona Team AI Budget", fmt.Sprintf("Unsupported mode %q.", data.Mode.ValueString()))
		return
	}
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Team AI Budget", "deleting the selected team AI budget", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *TeamAIBudgetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "importing", teamAIBudgetResourceType) {
		return
	}
	parts, diags := tfvalue.SplitImportID(req.ID, 3, "organization_id/team_id/mode")
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
	var validation diag.Diagnostics
	validateUUID(types.StringValue(parts[1]), path.Root("team_id"), true, &validation)
	knownMode(types.StringValue(parts[2]), true, &validation)
	resp.Diagnostics.Append(validation...)
	if resp.Diagnostics.HasError() {
		return
	}
	tfvalue.SetImportString(ctx, resp, "organization_id", parts[0])
	tfvalue.SetImportString(ctx, resp, "team_id", parts[1])
	tfvalue.SetImportString(ctx, resp, "mode", parts[2])
}

func (r *TeamAIBudgetResource) getTeamAllocation(ctx context.Context, organizationID, teamID string) (*v1.TeamCreditAllocationInfo, error) {
	allocation, err := getTeamCreditAllocation(ctx, r.raw, &teamCreditAllocationRequest{OrganizationID: organizationID, TeamID: teamID})
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, err
	}
	if allocation == nil {
		return nil, fmt.Errorf("API returned an empty team allocation response")
	}
	return allocation, nil
}

func (r *TeamAIBudgetResource) addTeamBudget(ctx context.Context, organizationID string, data TeamAIBudgetModel, existing *v1.TeamCreditAllocationInfo) (*v1.TeamCreditAllocationInfo, error) {
	if existing == nil {
		createReq, err := createTeamBudgetRequest(organizationID, data)
		if err != nil {
			return nil, err
		}
		allocation, err := createTeamCreditAllocation(ctx, r.raw, createReq)
		if err != nil {
			return nil, err
		}
		if allocation == nil {
			return nil, fmt.Errorf("API returned an empty create response")
		}
		return allocation, nil
	}
	updateReq, err := updateTeamBudgetRequest(organizationID, data)
	if err != nil {
		return nil, err
	}
	allocation, err := updateTeamCreditAllocation(ctx, r.raw, updateReq)
	if err != nil {
		return nil, err
	}
	if allocation == nil {
		return nil, fmt.Errorf("API returned an empty update response")
	}
	return allocation, nil
}

func (r *TeamAIBudgetResource) refresh(ctx context.Context, data *TeamAIBudgetModel, organizationID string, diags *diag.Diagnostics, remove func()) {
	teamID := data.TeamID.ValueString()
	mode := data.Mode.ValueString()
	allocation, err := r.getTeamAllocation(ctx, organizationID, teamID)
	if err != nil {
		providerdiag.AddAPIError(diags, "Unable to Read Ona Team AI Budget", "reading the team AI budget", err)
		return
	}
	refreshed := TeamAIBudgetModel{}
	exists, err := populateTeamBudget(&refreshed, allocation, organizationID, teamID, mode)
	if err != nil {
		diags.AddError("Unable to Map Ona Team AI Budget", err.Error())
		return
	}
	if !exists {
		*data = TeamAIBudgetModel{}
		remove()
		return
	}
	*data = refreshed
}

func teamModeConfigured(allocation *v1.TeamCreditAllocationInfo, mode string) bool {
	if allocation == nil {
		return false
	}
	switch mode {
	case modeCredits:
		return allocation.GetCreditBudget() > 0
	case modeBYOK:
		return allocation.CostBudgetMicrounits != nil
	default:
		return false
	}
}
