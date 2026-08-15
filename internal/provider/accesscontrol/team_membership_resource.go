// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/tfvalue"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &TeamMembershipResource{}
var _ resource.ResourceWithConfigure = &TeamMembershipResource{}
var _ resource.ResourceWithIdentity = &TeamMembershipResource{}
var _ resource.ResourceWithImportState = &TeamMembershipResource{}

func NewTeamMembershipResource() resource.Resource {
	return &TeamMembershipResource{}
}

type TeamMembershipResource struct {
	client *managementclient.ManagementPlane
}

type TeamMembershipModel struct {
	ID     types.String `tfsdk:"id"`
	TeamID types.String `tfsdk:"team_id"`
	UserID types.String `tfsdk:"user_id"`
}

func (r *TeamMembershipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_membership"
}

func (r *TeamMembershipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona team membership. Use this resource to assign one existing user to one team for usage attribution and budgeting.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Team membership ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"team_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Team ID to assign the user to. Changing this value replaces the membership after removing the old one.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Existing user ID to assign to the team. Resolve users by email and login provider with the ona_user data source. Changing this value replaces the membership.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *TeamMembershipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
}

func (r *TeamMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", "ona_team_membership") {
		return
	}

	result, err := r.client.TeamService().AddTeamMember(ctx, connect.NewRequest(&v1.AddTeamMemberRequest{
		TeamId: data.TeamID.ValueString(),
		UserId: data.UserID.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Team Membership", err.Error())
		return
	}

	var member *v1.TeamMember
	if result != nil && result.Msg != nil {
		member = result.Msg.GetMember()
	}
	// The add may have committed despite a malformed response. Preserve its
	// lookup identity before validating the returned member.
	if member == nil || member.GetId() == "" {
		data.ID = types.StringNull()
	} else {
		data.ID = types.StringValue(member.GetId())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	identity, err := teamMembershipIdentity(data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Team Membership", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if member == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Team Membership", "The Ona API returned an empty team membership.")
		return
	}
	if member.GetId() == "" {
		resp.Diagnostics.AddError("Unable to Create Ona Team Membership", "The Ona API returned a team membership without an ID.")
		return
	}
	if err := validateTeamMembership(member, data.TeamID.ValueString(), data.UserID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Team Membership", err.Error())
		return
	}

	populateTeamMembershipModel(&data, member)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *TeamMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", "ona_team_membership") {
		return
	}

	member, err := r.findTeamMembership(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Team Membership", err.Error())
		return
	}
	if member == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data = TeamMembershipModel{}
	populateTeamMembershipModel(&data, member)
	identity, err := teamMembershipIdentity(data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Team Membership", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *TeamMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unable to Update Ona Team Membership", "Team memberships are immutable. Change team_id or user_id by replacing the resource.")
}

func (r *TeamMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", "ona_team_membership") {
		return
	}

	membershipID := data.ID.ValueString()
	if data.ID.IsNull() || data.ID.IsUnknown() || membershipID == "" {
		member, err := r.findTeamMembership(ctx, data)
		if err != nil {
			resp.Diagnostics.AddError("Unable to Delete Ona Team Membership", err.Error())
			return
		}
		if member == nil {
			resp.State.RemoveResource(ctx)
			return
		}
		membershipID = member.GetId()
	}

	_, err := r.client.TeamService().RemoveTeamMember(ctx, connect.NewRequest(&v1.RemoveTeamMemberRequest{
		TeamMemberId: membershipID,
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		resp.Diagnostics.AddError("Unable to Delete Ona Team Membership", err.Error())
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *TeamMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		var identity TeamMembershipIdentityModel
		resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		data := TeamMembershipModel{TeamID: identity.TeamID, UserID: identity.UserID}
		if _, err := teamMembershipIdentity(data); err != nil {
			resp.Diagnostics.AddError("Invalid Team Membership Identity", err.Error())
			return
		}
		tfvalue.SetImportString(ctx, resp, "team_id", identity.TeamID.ValueString())
		if resp.Diagnostics.HasError() {
			return
		}
		tfvalue.SetImportString(ctx, resp, "user_id", identity.UserID.ValueString())
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		return
	}

	parts, diags := tfvalue.SplitImportID(req.ID, 2, "team_id/user_id")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tfvalue.SetImportString(ctx, resp, "team_id", parts[0])
	if resp.Diagnostics.HasError() {
		return
	}
	tfvalue.SetImportString(ctx, resp, "user_id", parts[1])
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *TeamMembershipResource) findTeamMembership(ctx context.Context, data TeamMembershipModel) (*v1.TeamMember, error) {
	teamID := data.TeamID.ValueString()
	userID := data.UserID.ValueString()
	if data.TeamID.IsNull() || data.TeamID.IsUnknown() || teamID == "" || data.UserID.IsNull() || data.UserID.IsUnknown() || userID == "" {
		return nil, fmt.Errorf("team_id and user_id must be known and configured")
	}

	membershipID := data.ID.ValueString()
	matchByID := !data.ID.IsNull() && !data.ID.IsUnknown() && membershipID != ""
	var matches []*v1.TeamMember
	var token string
	seenTokens := make(map[string]struct{})

	for {
		result, err := r.client.TeamService().ListTeamMembers(ctx, connect.NewRequest(&v1.ListTeamMembersRequest{
			TeamId: teamID,
			Pagination: &v1.PaginationRequest{
				PageSize: listutil.DefaultPageSize,
				Token:    token,
			},
		}))
		if err != nil {
			return nil, fmt.Errorf("list team members: %w", err)
		}
		if result == nil || result.Msg == nil {
			return nil, fmt.Errorf("list team members: the Ona API returned an empty response")
		}

		for _, member := range result.Msg.GetMembers() {
			if err := validateTeamMembership(member, teamID, ""); err != nil {
				return nil, fmt.Errorf("list team members: %w", err)
			}
			if matchByID {
				if member.GetId() != membershipID {
					continue
				}
				if err := validateTeamMembership(member, teamID, userID); err != nil {
					return nil, err
				}
				return member, nil
			}
			if member.GetUserId() == userID {
				matches = append(matches, member)
			}
		}

		nextToken := result.Msg.GetPagination().GetNextToken()
		if nextToken == "" {
			break
		}
		if err := listutil.NextPageToken(seenTokens, nextToken); err != nil {
			return nil, fmt.Errorf("list team members: %w", err)
		}
		token = nextToken
	}

	if matchByID || len(matches) == 0 {
		return nil, nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("the Ona API returned %d memberships for user %q in team %q; expected exactly one", len(matches), userID, teamID)
	}
	return matches[0], nil
}

func validateTeamMembership(member *v1.TeamMember, teamID, userID string) error {
	if member == nil {
		return fmt.Errorf("the Ona API returned an empty team membership")
	}
	if member.GetId() == "" {
		return fmt.Errorf("the Ona API returned a team membership without an ID")
	}
	if member.GetTeamId() == "" {
		return fmt.Errorf("the Ona API returned team membership %q without a team ID", member.GetId())
	}
	if member.GetUserId() == "" {
		return fmt.Errorf("the Ona API returned team membership %q without a user ID", member.GetId())
	}
	if teamID != "" && member.GetTeamId() != teamID {
		return fmt.Errorf("the Ona API returned team membership %q for team %q instead of %q", member.GetId(), member.GetTeamId(), teamID)
	}
	if userID != "" && member.GetUserId() != userID {
		return fmt.Errorf("the Ona API returned team membership %q for user %q instead of %q", member.GetId(), member.GetUserId(), userID)
	}
	return nil
}

func populateTeamMembershipModel(data *TeamMembershipModel, member *v1.TeamMember) {
	data.ID = types.StringValue(member.GetId())
	data.TeamID = types.StringValue(member.GetTeamId())
	data.UserID = types.StringValue(member.GetUserId())
}
