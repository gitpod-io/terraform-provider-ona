// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

var _ datasource.DataSource = &UserDataSource{}
var _ datasource.DataSourceWithConfigure = &UserDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

type UserDataSource struct {
	clientHolder
}

func (d *UserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = userDataSourceSchema()
}

func (d *UserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.configure(req, resp)
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !d.requireClient(&resp.Diagnostics, "ona_user") {
		return
	}
	if data.UserID.IsNull() || data.UserID.IsUnknown() || data.UserID.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("user_id"), "Missing Ona User ID", "user_id must be known before reading the data source.")
		return
	}
	userID := data.UserID.ValueString()
	if _, err := uuid.Parse(userID); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("user_id"), "Invalid Ona User ID", "user_id must be a valid UUID.")
		return
	}

	organizationID, err := d.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the organization for the Ona user data source", err)
		return
	}

	userResult, err := d.client.UserService().GetUser(ctx, connect.NewRequest(&v1.GetUserRequest{UserId: userID}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.Diagnostics.AddAttributeError(
				path.Root("user_id"),
				"Ona User Not Found or Not Visible",
				fmt.Sprintf("No Ona user visible to the configured token was found for user_id %q.", userID),
			)
			return
		}
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona User", "reading the Ona user data source", err)
		return
	}
	if userResult == nil || userResult.Msg == nil || userResult.Msg.GetUser() == nil {
		resp.Diagnostics.AddError("Unable to Read Ona User", "The Ona API returned an empty user response.")
		return
	}
	apiUser := userResult.Msg.GetUser()
	if apiUser.GetOrganizationId() != organizationID {
		resp.Diagnostics.AddError(
			"Ona User Organization Mismatch",
			fmt.Sprintf("Ona returned user %q for organization %q, but the configured token belongs to organization %q.", userID, apiUser.GetOrganizationId(), organizationID),
		)
		return
	}

	membersResult, err := d.client.OrganizationService().ListMembers(ctx, connect.NewRequest(&v1.ListMembersRequest{
		Pagination:     &v1.PaginationRequest{PageSize: 100},
		OrganizationId: organizationID,
		Filter:         &v1.ListMembersRequest_Filter{UserIds: []string{userID}},
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona User Membership", "reading organization membership for the Ona user data source", err)
		return
	}
	if membersResult == nil || membersResult.Msg == nil {
		resp.Diagnostics.AddError("Unable to Read Ona User Membership", "The Ona API returned an empty organization-members response.")
		return
	}
	members := membersResult.Msg.GetMembers()
	if len(members) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("user_id"),
			"Ona User Not Found or Not Visible",
			fmt.Sprintf("User %q is not visible as a member of the configured token's organization.", userID),
		)
		return
	}
	if len(members) > 1 {
		resp.Diagnostics.AddError("Unable to Read Ona User Membership", fmt.Sprintf("The Ona API returned %d memberships for user %q; expected exactly one.", len(members), userID))
		return
	}

	data, err = userModelFromResponses(apiUser, members[0])
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona User", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
