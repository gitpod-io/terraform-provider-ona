// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/tfvalue"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &TeamResource{}
var _ resource.ResourceWithConfigure = &TeamResource{}
var _ resource.ResourceWithImportState = &TeamResource{}

func NewTeamResource() resource.Resource {
	return &TeamResource{}
}

type TeamResource struct {
	client *managementclient.ManagementPlane
}

type TeamModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func (r *TeamResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (r *TeamResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona team in the organization associated with the authenticated provider token. Team membership is managed separately.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Team ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Team name shown in Ona. Must be between 3 and 80 characters.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(3, 80),
				},
			},
			"created_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the team was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *TeamResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
}

func (r *TeamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", "ona_team") {
		return
	}

	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}

	result, err := r.client.TeamService().CreateTeam(ctx, connect.NewRequest(&v1.CreateTeamRequest{
		OrganizationId: organizationID,
		Name:           data.Name.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Team", err.Error())
		return
	}
	if result.Msg.GetTeam() == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Team", "The Ona API returned an empty team.")
		return
	}

	data.ID = types.StringValue(result.Msg.GetTeam().GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	team, err := r.getTeam(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Created Ona Team", err.Error())
		return
	}
	if team == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	populateTeamModel(&data, team)
	preserveTeamPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", "ona_team") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Team", "Team ID is empty.")
		return
	}

	team, err := r.getTeam(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Team", err.Error())
		return
	}
	if team == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data = TeamModel{}
	populateTeamModel(&data, team)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TeamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "updating", "ona_team") {
		return
	}

	name := data.Name.ValueString()
	result, err := r.client.TeamService().UpdateTeam(ctx, connect.NewRequest(&v1.UpdateTeamRequest{
		TeamId: data.ID.ValueString(),
		Name:   &name,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update Ona Team", err.Error())
		return
	}
	if result.Msg.GetTeam() == nil {
		resp.Diagnostics.AddError("Unable to Update Ona Team", "The Ona API returned an empty team.")
		return
	}

	planned := data
	populateTeamModel(&data, result.Msg.GetTeam())
	preserveTeamPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", "ona_team") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.TeamService().DeleteTeam(ctx, connect.NewRequest(&v1.DeleteTeamRequest{TeamId: data.ID.ValueString()}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		resp.Diagnostics.AddError("Unable to Delete Ona Team", err.Error())
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *TeamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *TeamResource) getTeam(ctx context.Context, id string) (*v1.Team, error) {
	result, err := r.client.TeamService().GetTeam(ctx, connect.NewRequest(&v1.GetTeamRequest{TeamId: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get team: %w", err)
	}
	team := result.Msg.GetTeam()
	if team == nil {
		return nil, fmt.Errorf("get team: the Ona API returned an empty team")
	}
	return team, nil
}

func populateTeamModel(data *TeamModel, team *v1.Team) {
	data.ID = types.StringValue(team.GetId())
	data.Name = types.StringValue(team.GetName())
	data.CreatedAt = timestampString(team.GetCreatedAt())
}

func preserveTeamPlannedInputs(data *TeamModel, planned TeamModel) {
	data.Name = tfvalue.PreserveString(data.Name, planned.Name)
}
