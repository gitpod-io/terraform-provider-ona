// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

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
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &CollectionDataSource{}
var _ datasource.DataSourceWithConfigure = &CollectionDataSource{}

func NewCollectionDataSource() datasource.DataSource {
	return &CollectionDataSource{}
}

type CollectionDataSource struct {
	client *managementclient.ManagementPlane
}

type CollectionModel struct {
	ID      types.String      `tfsdk:"id"`
	Runners []DataSourceModel `tfsdk:"runners"`
}

func (d *CollectionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runners"
}

func (d *CollectionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = collectionDataSourceSchema()
}

func (d *CollectionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *CollectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_runners data sources.",
		)
		return
	}

	runners, err := d.listRunners(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Runners", "listing Ona runners", err)
		return
	}

	data := CollectionModel{
		ID:      types.StringValue("runners"),
		Runners: make([]DataSourceModel, 0, len(runners)),
	}
	for _, runner := range runners {
		var model DataSourceModel
		populateDataSourceModelFromRunner(&model, runner)
		data.Runners = append(data.Runners, model)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *CollectionDataSource) listRunners(ctx context.Context) ([]*v1.Runner, error) {
	var runners []*v1.Runner
	var token string

	for {
		result, err := d.client.RunnerService().ListRunners(ctx, connect.NewRequest(&v1.ListRunnersRequest{
			Pagination: &v1.PaginationRequest{
				PageSize: 100,
				Token:    token,
			},
		}))
		if err != nil {
			return nil, fmt.Errorf("list runners: %w", err)
		}

		runners = append(runners, result.Msg.GetRunners()...)
		token = result.Msg.GetPagination().GetNextToken()
		if token == "" {
			break
		}
	}

	sort.SliceStable(runners, func(i, j int) bool {
		return runners[i].GetRunnerId() < runners[j].GetRunnerId()
	})

	return runners, nil
}

func collectionDataSourceSchema() datasourceschema.Schema {
	return datasourceschema.Schema{
		MarkdownDescription: "Fetches Ona runners.",
		Attributes: map[string]datasourceschema.Attribute{
			"id": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform data source ID.",
			},
			"runners": datasourceschema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Ona runners.",
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: dataSourceRunnerAttributes(datasourceschema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Runner ID.",
					}),
				},
			},
		},
	}
}
