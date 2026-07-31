// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package project

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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ resource.Resource = &InsightsResource{}
var _ resource.ResourceWithConfigure = &InsightsResource{}
var _ resource.ResourceWithIdentity = &InsightsResource{}
var _ resource.ResourceWithImportState = &InsightsResource{}

func NewInsightsResource() resource.Resource {
	return &InsightsResource{}
}

type InsightsResource struct {
	client *managementclient.ManagementPlane
}

type ProjectInsightsModel struct {
	ID                   types.String `tfsdk:"id"`
	ProjectID            types.String `tfsdk:"project_id"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	LastRanAt            types.String `tfsdk:"last_ran_at"`
	DataCollectedThrough types.String `tfsdk:"data_collected_through"`
}

func (r *InsightsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_insights"
}

func (r *InsightsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Project-level Ona Insights enablement. Deleting this resource disables Insights and removes its associated workflow.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project Insights resource ID, equal to the project ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the project whose Insights enablement is managed.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether Insights is enabled for the project.",
			},
			"last_ran_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the Insights workflow last completed successfully.",
			},
			"data_collected_through": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 upper bound of the most recent successfully collected Insights data.",
			},
		},
	}
}

func (r *InsightsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *InsightsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectInsightsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient("creating", &resp.Diagnostics) {
		return
	}

	if err := r.setEnabled(ctx, data.ProjectID.ValueString(), data.Enabled.ValueBool()); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Project Insights", "setting project Insights enablement", err)
		return
	}

	data.ID = types.StringValue(data.ProjectID.ValueString())
	resp.Diagnostics.Append(resp.Identity.Set(ctx, InsightsIdentityModel{ProjectID: data.ProjectID})...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readStatus(ctx, &data, &resp.State, &resp.Diagnostics)
}

func (r *InsightsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectInsightsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient("reading", &resp.Diagnostics) {
		return
	}
	if data.ProjectID.ValueString() == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Project Insights", "Project ID is empty.")
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, InsightsIdentityModel{ProjectID: data.ProjectID})...)
	r.readStatus(ctx, &data, &resp.State, &resp.Diagnostics)
}

func (r *InsightsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectInsightsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var prior ProjectInsightsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient("updating", &resp.Diagnostics) {
		return
	}

	if !data.Enabled.Equal(prior.Enabled) {
		if err := r.setEnabled(ctx, data.ProjectID.ValueString(), data.Enabled.ValueBool()); err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Project Insights", "setting project Insights enablement", err)
			return
		}
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, InsightsIdentityModel{ProjectID: data.ProjectID})...)
	r.readStatus(ctx, &data, &resp.State, &resp.Diagnostics)
}

func (r *InsightsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectInsightsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient("deleting", &resp.Diagnostics) {
		return
	}

	projectID := data.ProjectID.ValueString()
	if projectID == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	_, err := r.client.InsightsService().DisableProjectInsights(ctx, connect.NewRequest(&v1.DisableProjectInsightsRequest{ProjectId: projectID}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Project Insights", "disabling project Insights", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *InsightsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("project_id"), req, resp)
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("project_id"), path.Root("project_id"), req, resp)
}

func (r *InsightsResource) requireClient(operation string, diags *diag.Diagnostics) bool {
	if r.client != nil {
		return true
	}
	diags.AddError(
		"Ona API Client Is Not Configured",
		fmt.Sprintf("Set the provider token argument or ONA_TOKEN before %s ona_project_insights resources.", operation),
	)
	return false
}

func (r *InsightsResource) setEnabled(ctx context.Context, projectID string, enabled bool) error {
	if enabled {
		_, err := r.client.InsightsService().EnableProjectInsights(ctx, connect.NewRequest(&v1.EnableProjectInsightsRequest{ProjectId: projectID}))
		if err != nil {
			return fmt.Errorf("enable project insights: %w", err)
		}
		return nil
	}
	_, err := r.client.InsightsService().DisableProjectInsights(ctx, connect.NewRequest(&v1.DisableProjectInsightsRequest{ProjectId: projectID}))
	if err != nil {
		return fmt.Errorf("disable project insights: %w", err)
	}
	return nil
}

func (r *InsightsResource) readStatus(ctx context.Context, data *ProjectInsightsModel, state *tfsdk.State, diags *diag.Diagnostics) {
	result, err := r.client.InsightsService().GetProjectInsightsStatus(ctx, connect.NewRequest(&v1.GetProjectInsightsStatusRequest{
		ProjectId: data.ProjectID.ValueString(),
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			state.RemoveResource(ctx)
			return
		}
		providerdiag.AddAPIError(diags, "Unable to Read Ona Project Insights", "reading project Insights status", err)
		return
	}

	data.ID = types.StringValue(data.ProjectID.ValueString())
	data.Enabled = types.BoolValue(result.Msg.GetEnabled())
	data.LastRanAt = projectInsightsTimestamp(result.Msg.GetLastRanAt())
	data.DataCollectedThrough = projectInsightsTimestamp(result.Msg.GetDataCollectedThrough())
	diags.Append(state.Set(ctx, data)...)
}

func projectInsightsTimestamp(value *timestamppb.Timestamp) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(value.AsTime().Format(time.RFC3339))
}
