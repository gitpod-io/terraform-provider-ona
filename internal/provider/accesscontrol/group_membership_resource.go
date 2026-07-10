// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &GroupMembershipResource{}
var _ resource.ResourceWithConfigure = &GroupMembershipResource{}
var _ resource.ResourceWithImportState = &GroupMembershipResource{}

func NewGroupMembershipResource() resource.Resource {
	return &GroupMembershipResource{}
}

type GroupMembershipResource struct {
	clientHolder
}

type GroupMembershipModel struct {
	ID               types.String `tfsdk:"id"`
	GroupID          types.String `tfsdk:"group_id"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
	Principal        types.String `tfsdk:"principal"`
	Name             types.String `tfsdk:"name"`
	AvatarURL        types.String `tfsdk:"avatar_url"`
}

func (r *GroupMembershipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_membership"
}

func (r *GroupMembershipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona service-account group membership.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Group membership ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_account_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Service account ID to add to the group.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"principal": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Principal type for this membership. This resource supports only `service_account`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Display name of the member.",
			},
			"avatar_url": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Avatar URL of the member when available.",
			},
		},
	}
}

func (r *GroupMembershipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configure(req, resp)
}

func (r *GroupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GroupMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "creating", "ona_group_membership") {
		return
	}

	result, err := r.client.GroupService().CreateMembership(ctx, connect.NewRequest(&v1.CreateMembershipRequest{
		GroupId: data.GroupID.ValueString(),
		Subject: &v1.Subject{
			Id:        data.ServiceAccountID.ValueString(),
			Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT,
		},
	}))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Group Membership", err.Error())
		return
	}
	if result.Msg.GetMember() == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Group Membership", "The Ona API returned an empty membership.")
		return
	}

	populateGroupMembershipModel(&data, result.Msg.GetMember())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GroupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "reading", "ona_group_membership") {
		return
	}

	member, err := r.getMembership(ctx, data.GroupID.ValueString(), data.ServiceAccountID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Group Membership", err.Error())
		return
	}
	if member == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data = GroupMembershipModel{}
	populateGroupMembershipModel(&data, member)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unable to Update Ona Group Membership", "Group memberships are immutable. Change group_id or service_account_id by replacing the resource.")
}

func (r *GroupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GroupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "deleting", "ona_group_membership") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.GroupService().DeleteMembership(ctx, connect.NewRequest(&v1.DeleteMembershipRequest{
		MembershipId: data.ID.ValueString(),
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		resp.Diagnostics.AddError("Unable to Delete Ona Group Membership", err.Error())
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *GroupMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, diags := splitImportID(req.ID, 2, "group_id/service_account_id")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	setImportString(ctx, resp, "group_id", parts[0])
	setImportString(ctx, resp, "service_account_id", parts[1])
	setImportString(ctx, resp, "principal", principalServiceAccount)
}

func (r *GroupMembershipResource) getMembership(ctx context.Context, groupID string, serviceAccountID string) (*v1.GroupMembership, error) {
	if groupID == "" || serviceAccountID == "" {
		return nil, fmt.Errorf("group_id and service_account_id must be set")
	}
	result, err := r.client.GroupService().GetMembership(ctx, connect.NewRequest(&v1.GetMembershipRequest{
		GroupId: groupID,
		Subject: &v1.Subject{
			Id:        serviceAccountID,
			Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT,
		},
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get membership: %w", err)
	}
	return result.Msg.GetMember(), nil
}

func populateGroupMembershipModel(data *GroupMembershipModel, member *v1.GroupMembership) {
	subject := member.GetSubject()
	data.ID = types.StringValue(member.GetId())
	data.GroupID = types.StringValue(member.GetGroupId())
	data.ServiceAccountID = types.StringValue(subject.GetId())
	data.Principal = types.StringValue(principalServiceAccount)
	data.Name = types.StringValue(member.GetName())
	data.AvatarURL = types.StringValue(member.GetAvatarUrl())
}
