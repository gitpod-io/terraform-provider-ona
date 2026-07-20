// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package warmpool

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

var _ datasource.DataSource = &WarmPoolDataSource{}
var _ datasource.DataSourceWithConfigure = &WarmPoolDataSource{}

func NewWarmPoolDataSource() datasource.DataSource {
	return &WarmPoolDataSource{}
}

type WarmPoolDataSource struct {
	client *managementclient.ManagementPlane
}

func (d *WarmPoolDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_warm_pool"
}

func (d *WarmPoolDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = warmPoolDataSourceSchema()
}

func (d *WarmPoolDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WarmPoolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WarmPoolDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_warm_pool data sources.",
		)
		return
	}

	id := dataSourceWarmPoolID(data)
	if id == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("warm_pool_id"),
			"Missing Warm Pool ID",
			"Set warm_pool_id before reading an Ona warm pool data source.",
		)
		return
	}

	result, err := d.client.PrebuildService().GetWarmPool(ctx, connect.NewRequest(&v1.GetWarmPoolRequest{WarmPoolId: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.Diagnostics.AddAttributeError(
				path.Root("warm_pool_id"),
				"Warm Pool Not Found",
				fmt.Sprintf("No Ona warm pool exists with warm_pool_id %q.", id),
			)
			return
		}
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Warm Pool", "reading the Ona warm pool data source", err)
		return
	}
	if result.Msg.GetWarmPool() == nil {
		resp.Diagnostics.AddError("Unable to Read Ona Warm Pool", "The Ona API returned an empty warm pool.")
		return
	}

	var resultData WarmPoolDataSourceModel
	populateWarmPoolDataSourceModel(&resultData, result.Msg.GetWarmPool())
	resp.Diagnostics.Append(resp.State.Set(ctx, &resultData)...)
}

func dataSourceWarmPoolID(data WarmPoolDataSourceModel) string {
	if !data.WarmPoolID.IsNull() && !data.WarmPoolID.IsUnknown() && data.WarmPoolID.ValueString() != "" {
		return data.WarmPoolID.ValueString()
	}
	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		return data.ID.ValueString()
	}
	return ""
}
