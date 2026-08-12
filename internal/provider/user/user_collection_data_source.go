// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"sort"

	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

var _ datasource.DataSource = &UserCollectionDataSource{}
var _ datasource.DataSourceWithConfigure = &UserCollectionDataSource{}

func NewUserCollectionDataSource() datasource.DataSource {
	return &UserCollectionDataSource{}
}

type UserCollectionDataSource struct {
	client *managementclient.ManagementPlane
}

func (d *UserCollectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *UserCollectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = userCollectionDataSourceSchema()
}

func (d *UserCollectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = providerdata.DataSourceClient(req.ProviderData, d.client, &resp.Diagnostics)
}

func (d *UserCollectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserCollectionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireDataSourceClient(d.client, &resp.Diagnostics, "ona_users") {
		return
	}

	filter := userCollectionFilter(data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	organizationID, err := authenticatedOrganizationID(ctx, d.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the organization for the Ona users data source", err)
		return
	}
	members, err := listOrganizationMembers(ctx, d.client, organizationID, filter)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to List Ona Users", "listing Ona users", err)
		return
	}

	data.Users = make([]UserModel, 0, len(members))
	for _, member := range members {
		model, err := userModelFromMember(member)
		if err != nil {
			resp.Diagnostics.AddError("Unable to List Ona Users", err.Error())
			continue
		}
		data.Users = append(data.Users, model)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	sort.SliceStable(data.Users, func(i, j int) bool {
		return data.Users[i].UserID.ValueString() < data.Users[j].UserID.ValueString()
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
