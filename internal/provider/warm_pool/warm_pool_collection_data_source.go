// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package warmpool

import (
	"context"
	"fmt"
	"sort"

	"connectrpc.com/connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/api/go/client"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &WarmPoolCollectionDataSource{}
var _ datasource.DataSourceWithConfigure = &WarmPoolCollectionDataSource{}

func NewWarmPoolCollectionDataSource() datasource.DataSource {
	return &WarmPoolCollectionDataSource{}
}

type WarmPoolCollectionDataSource struct {
	client *managementclient.ManagementPlane
}

func (d *WarmPoolCollectionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warm_pools"
}

func (d *WarmPoolCollectionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = warmPoolCollectionDataSourceSchema()
}

func (d *WarmPoolCollectionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerdata.Data)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *providerdata.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = data.Client
}

func (d *WarmPoolCollectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WarmPoolCollectionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_warm_pools data sources.",
		)
		return
	}

	warmPools, err := d.listWarmPools(ctx, data)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Warm Pools", "listing Ona warm pools", err)
		return
	}

	result := WarmPoolCollectionModel{
		ID:                  types.StringValue("warm_pools"),
		ProjectIDs:          data.ProjectIDs,
		EnvironmentClassIDs: data.EnvironmentClassIDs,
		PageSize:            data.PageSize,
		WarmPools:           make([]WarmPoolDataSourceModel, 0, len(warmPools)),
	}
	for _, warmPool := range warmPools {
		var model WarmPoolDataSourceModel
		populateWarmPoolDataSourceModel(&model, warmPool)
		result.WarmPools = append(result.WarmPools, model)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}

func (d *WarmPoolCollectionDataSource) listWarmPools(ctx context.Context, data WarmPoolCollectionModel) ([]*v1.WarmPool, error) {
	filter, diags := warmPoolCollectionFilters(ctx, data)
	if diags.HasError() {
		return nil, invalidMappingError(diags[0].Summary())
	}

	var warmPools []*v1.WarmPool
	var token string
	for {
		result, err := d.client.PrebuildService().ListWarmPools(ctx, connect.NewRequest(&v1.ListWarmPoolsRequest{
			Pagination: &v1.PaginationRequest{
				PageSize: pageSize(data),
				Token:    token,
			},
			Filter: filter,
		}))
		if err != nil {
			return nil, fmt.Errorf("list warm pools: %w", err)
		}

		warmPools = append(warmPools, result.Msg.GetWarmPools()...)
		token = result.Msg.GetPagination().GetNextToken()
		if token == "" {
			break
		}
	}

	sort.SliceStable(warmPools, func(i, j int) bool {
		return warmPools[i].GetId() < warmPools[j].GetId()
	})
	return warmPools, nil
}
