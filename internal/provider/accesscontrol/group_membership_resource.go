// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/tfvalue"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &GroupMembershipResource{}
var _ resource.ResourceWithConfigure = &GroupMembershipResource{}
var _ resource.ResourceWithConfigValidators = &GroupMembershipResource{}
var _ resource.ResourceWithIdentity = &GroupMembershipResource{}
var _ resource.ResourceWithImportState = &GroupMembershipResource{}

func NewGroupMembershipResource() resource.Resource {
	return &GroupMembershipResource{}
}

type GroupMembershipResource struct {
	client *managementclient.ManagementPlane
}

type GroupMembershipModel struct {
	ID               types.String `tfsdk:"id"`
	GroupID          types.String `tfsdk:"group_id"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
	UserID           types.String `tfsdk:"user_id"`
}

func (r *GroupMembershipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_membership"
}

func (r *GroupMembershipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona group membership. Use this resource to add one user or service account to one group.",
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
				MarkdownDescription: "Group ID to add the member to. Changing this value replaces the membership.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_account_id": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Service account ID to add to the group. Set exactly one of service_account_id or user_id. Changing this value replaces the membership.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user_id": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "User ID to add to the group. Set exactly one of user_id or service_account_id. Changing this value replaces the membership.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *GroupMembershipResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("service_account_id"),
			path.MatchRoot("user_id"),
		),
	}
}

func (r *GroupMembershipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
}

func (r *GroupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GroupMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", "ona_group_membership") {
		return
	}
	subject, err := groupMembershipSubject(data.UserID, data.ServiceAccountID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Group Membership", err.Error())
		return
	}

	result, err := r.client.GroupService().CreateMembership(ctx, connect.NewRequest(&v1.CreateMembershipRequest{
		GroupId: data.GroupID.ValueString(),
		Subject: subject,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Group Membership", err.Error())
		return
	}
	member := result.Msg.GetMember()
	if member == nil {
		// The create RPC may have committed despite the empty response. Preserve
		// its lookup identity so refresh can recover the remote membership ID.
		data.ID = types.StringNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		if resp.Diagnostics.HasError() {
			return
		}
		identity, err := groupMembershipIdentity(data)
		if err != nil {
			resp.Diagnostics.AddError("Unable to Create Ona Group Membership", err.Error())
			return
		}
		resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.AddError("Unable to Create Ona Group Membership", "The Ona API returned an empty membership.")
		return
	}

	data.ID = types.StringValue(member.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := populateGroupMembershipModel(&data, member); err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Group Membership", err.Error())
		return
	}
	identity, err := groupMembershipIdentity(data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Group Membership", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GroupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", "ona_group_membership") {
		return
	}

	subject, err := groupMembershipSubject(data.UserID, data.ServiceAccountID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Group Membership", err.Error())
		return
	}
	member, err := r.getMembership(ctx, data.GroupID.ValueString(), subject)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Group Membership", err.Error())
		return
	}
	if member == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data = GroupMembershipModel{}
	if err := populateGroupMembershipModel(&data, member); err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Group Membership", err.Error())
		return
	}
	identity, err := groupMembershipIdentity(data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Group Membership", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unable to Update Ona Group Membership", "Group memberships are immutable. Change group_id, service_account_id, or user_id by replacing the resource.")
}

func (r *GroupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GroupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", "ona_group_membership") {
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
	if req.ID == "" {
		var identity GroupMembershipIdentityModel
		resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		subject, err := groupMembershipSubject(identity.UserID, identity.ServiceAccountID)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Group Membership Identity", err.Error())
			return
		}
		setGroupMembershipImportState(ctx, resp, identity.GroupID.ValueString(), subject)
		return
	}
	groupID, subject, err := parseGroupMembershipImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", err.Error())
		return
	}
	setGroupMembershipImportState(ctx, resp, groupID, subject)
}

func (r *GroupMembershipResource) getMembership(ctx context.Context, groupID string, subject *v1.Subject) (*v1.GroupMembership, error) {
	if groupID == "" || subject == nil || subject.GetId() == "" {
		return nil, fmt.Errorf("group_id and the member ID must be set")
	}
	result, err := r.client.GroupService().GetMembership(ctx, connect.NewRequest(&v1.GetMembershipRequest{
		GroupId: groupID,
		Subject: subject,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get membership: %w", err)
	}
	return result.Msg.GetMember(), nil
}

func groupMembershipSubject(userID, serviceAccountID types.String) (*v1.Subject, error) {
	if userID.IsUnknown() || serviceAccountID.IsUnknown() {
		return nil, fmt.Errorf("exactly one of user_id or service_account_id must be known and configured")
	}
	hasUser := !userID.IsNull()
	hasServiceAccount := !serviceAccountID.IsNull()
	if hasUser == hasServiceAccount {
		return nil, fmt.Errorf("exactly one of user_id or service_account_id must be configured")
	}
	if hasUser {
		return &v1.Subject{Id: userID.ValueString(), Principal: v1.Principal_PRINCIPAL_USER}, nil
	}
	return &v1.Subject{Id: serviceAccountID.ValueString(), Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}, nil
}

func populateGroupMembershipModel(data *GroupMembershipModel, member *v1.GroupMembership) error {
	if member == nil {
		return fmt.Errorf("the Ona API returned an empty membership")
	}
	subject := member.GetSubject()
	if subject == nil {
		return fmt.Errorf("the Ona API returned a membership without a subject")
	}
	data.ID = types.StringValue(member.GetId())
	data.GroupID = types.StringValue(member.GetGroupId())
	data.ServiceAccountID = types.StringNull()
	data.UserID = types.StringNull()
	switch subject.GetPrincipal() {
	case v1.Principal_PRINCIPAL_USER:
		data.UserID = types.StringValue(subject.GetId())
	case v1.Principal_PRINCIPAL_SERVICE_ACCOUNT:
		data.ServiceAccountID = types.StringValue(subject.GetId())
	default:
		return fmt.Errorf("the Ona API returned unsupported membership principal %q", subject.GetPrincipal().String())
	}
	return nil
}

func groupMembershipIdentity(data GroupMembershipModel) (GroupMembershipIdentityModel, error) {
	subject, err := groupMembershipSubject(data.UserID, data.ServiceAccountID)
	if err != nil {
		return GroupMembershipIdentityModel{}, err
	}
	identity := GroupMembershipIdentityModel{
		GroupID:          data.GroupID,
		ServiceAccountID: types.StringNull(),
		UserID:           types.StringNull(),
	}
	if subject.GetPrincipal() == v1.Principal_PRINCIPAL_USER {
		identity.UserID = types.StringValue(subject.GetId())
	} else {
		identity.ServiceAccountID = types.StringValue(subject.GetId())
	}
	return identity, nil
}

func parseGroupMembershipImportID(id string) (string, *v1.Subject, error) {
	const formats = "group_id/service_account_id, group_id/service_account/service_account_id, or group_id/user/user_id"
	parts := strings.Split(id, "/")
	if len(parts) != 2 && len(parts) != 3 {
		return "", nil, fmt.Errorf("expected import ID format: %s", formats)
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return "", nil, fmt.Errorf("expected import ID format: %s", formats)
		}
	}
	if len(parts) == 2 {
		return parts[0], &v1.Subject{Id: parts[1], Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}, nil
	}
	switch parts[1] {
	case "service_account":
		return parts[0], &v1.Subject{Id: parts[2], Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT}, nil
	case "user":
		return parts[0], &v1.Subject{Id: parts[2], Principal: v1.Principal_PRINCIPAL_USER}, nil
	default:
		return "", nil, fmt.Errorf("expected import ID format: %s", formats)
	}
}

func setGroupMembershipImportState(ctx context.Context, resp *resource.ImportStateResponse, groupID string, subject *v1.Subject) {
	tfvalue.SetImportString(ctx, resp, "group_id", groupID)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_account_id"), types.StringNull())...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), types.StringNull())...)
	if resp.Diagnostics.HasError() {
		return
	}
	if subject.GetPrincipal() == v1.Principal_PRINCIPAL_USER {
		tfvalue.SetImportString(ctx, resp, "user_id", subject.GetId())
		return
	}
	tfvalue.SetImportString(ctx, resp, "service_account_id", subject.GetId())
}
