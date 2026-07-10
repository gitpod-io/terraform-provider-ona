// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &GroupResource{}
var _ resource.ResourceWithConfigure = &GroupResource{}
var _ resource.ResourceWithImportState = &GroupResource{}
var _ resource.ResourceWithValidateConfig = &GroupResource{}

func NewGroupResource() resource.Resource {
	return &GroupResource{}
}

type GroupResource struct {
	clientHolder
}

type GroupModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	SystemManaged  types.Bool   `tfsdk:"system_managed"`
	DirectShare    types.Bool   `tfsdk:"direct_share"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	MemberCount    types.Int64  `tfsdk:"member_count"`
}

func (r *GroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *GroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona custom group for organization access control. The group is created in the organization associated with the authenticated provider token.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Group ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization ID that owns the group. This is resolved from the authenticated provider token.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group name shown in Ona. Must be between 3 and 80 characters.",
			},
			"description": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Group description. Must be at most 255 characters. Omit to leave the description empty.",
			},
			"system_managed": resourceschema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this group is system-managed by Ona rather than customer-managed.",
				PlanModifiers: []planmodifier.Bool{
					// Existing imports should not show unknown for stable API metadata during planning.
					boolUseStateForUnknown(),
				},
			},
			"direct_share": resourceschema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this group is used by Ona for direct resource sharing.",
				PlanModifiers: []planmodifier.Bool{
					boolUseStateForUnknown(),
				},
			},
			"created_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the group was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the group was last updated.",
			},
			"member_count": resourceschema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of members in the group.",
			},
		},
	}
}

func (r *GroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configure(req, resp)
}

func (r *GroupResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data GroupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateGroupModel(data, &resp.Diagnostics)
}

func (r *GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "creating", "ona_group") {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}

	createReq := &v1.CreateGroupRequest{
		OrganizationId: organizationID,
		Name:           data.Name.ValueString(),
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		createReq.Description = data.Description.ValueString()
	}

	result, err := r.client.GroupService().CreateGroup(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Group", err.Error())
		return
	}
	if result.Msg.GetGroup() == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Group", "The Ona API returned an empty group.")
		return
	}

	data.ID = types.StringValue(result.Msg.GetGroup().GetId())
	data.OrganizationID = types.StringValue(organizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.getGroup(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Created Ona Group", err.Error())
		return
	}
	if group == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	populateGroupModel(&data, group)
	preserveGroupPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "reading", "ona_group") {
		return
	}

	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Group", "Group ID is empty.")
		return
	}

	group, err := r.getGroup(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Group", err.Error())
		return
	}
	if group == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data = GroupModel{}
	populateGroupModel(&data, group)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "updating", "ona_group") {
		return
	}

	updateReq := &v1.UpdateGroupRequest{
		GroupId: data.ID.ValueString(),
		Name:    data.Name.ValueString(),
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		updateReq.Description = data.Description.ValueString()
	}

	result, err := r.client.GroupService().UpdateGroup(ctx, connect.NewRequest(updateReq))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update Ona Group", err.Error())
		return
	}
	if result.Msg.GetGroup() == nil {
		resp.Diagnostics.AddError("Unable to Update Ona Group", "The Ona API returned an empty group.")
		return
	}

	planned := data
	populateGroupModel(&data, result.Msg.GetGroup())
	preserveGroupPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "deleting", "ona_group") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.GroupService().DeleteGroup(ctx, connect.NewRequest(&v1.DeleteGroupRequest{GroupId: data.ID.ValueString()}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		resp.Diagnostics.AddError("Unable to Delete Ona Group", err.Error())
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *GroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *GroupResource) getGroup(ctx context.Context, id string) (*v1.Group, error) {
	result, err := r.client.GroupService().GetGroup(ctx, connect.NewRequest(&v1.GetGroupRequest{
		Group: &v1.GetGroupRequest_Id{Id: id},
	}))
	if err != nil {
		if groupNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get group: %w", err)
	}
	return result.Msg.GetGroup(), nil
}

func populateGroupModel(data *GroupModel, group *v1.Group) {
	data.ID = types.StringValue(group.GetId())
	data.OrganizationID = types.StringValue(group.GetOrganizationId())
	data.Name = types.StringValue(group.GetName())
	data.Description = types.StringValue(group.GetDescription())
	data.SystemManaged = types.BoolValue(group.GetSystemManaged())
	data.DirectShare = types.BoolValue(group.GetDirectShare())
	data.CreatedAt = timestampString(group.GetCreatedAt())
	data.UpdatedAt = timestampString(group.GetUpdatedAt())
	data.MemberCount = types.Int64Value(int64(group.GetMemberCount()))
}

func preserveGroupPlannedInputs(data *GroupModel, planned GroupModel) {
	data.Name = preserveKnownString(data.Name, planned.Name)
	data.Description = preserveKnownString(data.Description, planned.Description)
}

func validateGroupModel(data GroupModel, diags *diag.Diagnostics) {
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		name := data.Name.ValueString()
		if len(name) < 3 || len(name) > 80 {
			diags.AddAttributeError(path.Root("name"), "Invalid Group Name", "Group name must be between 3 and 80 characters.")
		}
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() && len(data.Description.ValueString()) > 255 {
		diags.AddAttributeError(path.Root("description"), "Invalid Group Description", "Group description must be at most 255 characters.")
	}
}
