// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

var _ datasource.DataSource = &UserDataSource{}
var _ datasource.DataSourceWithConfigure = &UserDataSource{}
var _ datasource.DataSourceWithValidateConfig = &UserDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

type UserDataSource struct {
	client *managementclient.ManagementPlane
}

func (d *UserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = userDataSourceSchema()
}

func (d *UserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = providerdata.DataSourceClient(req.ProviderData, d.client, &resp.Diagnostics)
}

func (d *UserDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data UserModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateUserSelectorConfig(data, &resp.Diagnostics)
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	selector, ok := userSelectorFromModel(data, &resp.Diagnostics)
	if !ok {
		return
	}
	if selector.byUserID {
		if _, err := uuid.Parse(selector.userID); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("user_id"), "Invalid Ona User ID", "user_id must be a valid UUID.")
			return
		}
	}
	if !providerdata.RequireDataSourceClient(d.client, &resp.Diagnostics, "ona_user") {
		return
	}

	organizationID, err := authenticatedOrganizationID(ctx, d.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the organization for the Ona user data source", err)
		return
	}

	members, err := listOrganizationMembers(ctx, d.client, organizationID, selector.listFilter())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona User Membership", "reading organization membership for the Ona user data source", err)
		return
	}
	matches := selector.exactMatches(members)
	if len(matches) == 0 {
		if selector.byUserID {
			resp.Diagnostics.AddAttributeError(
				path.Root("user_id"),
				"Ona User Not Found or Not Visible",
				fmt.Sprintf("User %q is not visible as a member of the configured token's organization.", selector.userID),
			)
		} else {
			resp.Diagnostics.AddAttributeError(
				path.Root("email"),
				"Ona User Not Found or Not Visible",
				fmt.Sprintf("No Ona user in the configured token's organization matched email %q and login_provider %q. The user might not exist, belong to another organization, or be hidden from this token.", selector.email, selector.loginProvider),
			)
		}
		return
	}
	if len(matches) > 1 {
		if selector.byUserID {
			resp.Diagnostics.AddError("Unable to Read Ona User Membership", fmt.Sprintf("The Ona API returned %d memberships for user %q; expected exactly one.", len(matches), selector.userID))
		} else {
			resp.Diagnostics.AddAttributeError(
				path.Root("email"),
				"Multiple Ona Users Matched",
				fmt.Sprintf("Found %d Ona users matching email %q and login_provider %q: %s. Use user_id to select one, or remove the duplicate identities.", len(matches), selector.email, selector.loginProvider, describeUserMatches(matches)),
			)
		}
		return
	}
	member := matches[0]
	userID := member.GetUserId()
	if _, err := uuid.Parse(userID); err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona User", fmt.Sprintf("The Ona API returned invalid user ID %q.", userID))
		return
	}

	userResult, err := d.client.UserService().GetUser(ctx, connect.NewRequest(&v1.GetUserRequest{UserId: userID}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			if selector.byUserID {
				resp.Diagnostics.AddAttributeError(
					path.Root("user_id"),
					"Ona User Not Found or Not Visible",
					fmt.Sprintf("No Ona user visible to the configured token was found for user_id %q.", userID),
				)
			} else {
				resp.Diagnostics.AddAttributeError(
					path.Root("email"),
					"Ona User Not Found or Not Visible",
					fmt.Sprintf("Organization member %q matched email %q and login_provider %q, but its user record is not visible to the configured token. The user might have been removed or become hidden after the membership lookup.", userID, selector.email, selector.loginProvider),
				)
			}
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

	data, err = userModelFromResponses(apiUser, member)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona User", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
