// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"fmt"
	"sort"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

var _ datasource.DataSource = &UserCollectionDataSource{}
var _ datasource.DataSourceWithConfigure = &UserCollectionDataSource{}

func NewUserCollectionDataSource() datasource.DataSource {
	return &UserCollectionDataSource{}
}

type UserCollectionDataSource struct {
	clientHolder
}

func (d *UserCollectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *UserCollectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = userCollectionDataSourceSchema()
}

func (d *UserCollectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.configure(req, resp)
}

func (d *UserCollectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserCollectionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !d.requireClient(&resp.Diagnostics, "ona_users") {
		return
	}

	filter := userCollectionFilter(data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	organizationID, err := d.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "resolving the organization for the Ona users data source", err)
		return
	}
	members, err := d.listMembers(ctx, organizationID, filter)
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

func (d *UserCollectionDataSource) listMembers(ctx context.Context, organizationID string, filter *v1.ListMembersRequest_Filter) ([]*v1.OrganizationMember, error) {
	var members []*v1.OrganizationMember
	var token string
	seenTokens := make(map[string]struct{})

	for {
		result, err := d.client.OrganizationService().ListMembers(ctx, connect.NewRequest(&v1.ListMembersRequest{
			Pagination:     &v1.PaginationRequest{PageSize: 100, Token: token},
			OrganizationId: organizationID,
			Filter:         filter,
			Sort: &v1.ListMembersRequest_Sort{
				Field: v1.ListMembersRequest_SORT_FIELD_NAME,
				Order: v1.SortOrder_SORT_ORDER_ASC,
			},
		}))
		if err != nil {
			return nil, fmt.Errorf("list organization members: %w", err)
		}
		if result == nil || result.Msg == nil {
			return nil, fmt.Errorf("list organization members: Ona returned an empty response")
		}
		members = append(members, result.Msg.GetMembers()...)
		nextToken := result.Msg.GetPagination().GetNextToken()
		if nextToken == "" {
			return members, nil
		}
		if _, ok := seenTokens[nextToken]; ok {
			return nil, fmt.Errorf("list organization members: Ona returned repeated pagination token %q", nextToken)
		}
		seenTokens[nextToken] = struct{}{}
		token = nextToken
	}
}
