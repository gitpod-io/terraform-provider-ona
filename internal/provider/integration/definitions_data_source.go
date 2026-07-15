// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package integration

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

var _ datasource.DataSource = &DefinitionsDataSource{}
var _ datasource.DataSourceWithConfigure = &DefinitionsDataSource{}

func NewDefinitionsDataSource() datasource.DataSource {
	return &DefinitionsDataSource{}
}

type DefinitionsDataSource struct {
	client *managementclient.ManagementPlane
}

func (d *DefinitionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_definitions"
}

func (d *DefinitionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = definitionsDataSourceSchema()
}

func (d *DefinitionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DefinitionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Ona API Client Is Not Configured", "Set the provider token argument or ONA_TOKEN before reading ona_integration_definitions data sources.")
		return
	}

	definitions, err := d.listDefinitions(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Integration Definitions", "listing Ona integration definitions", err)
		return
	}
	data := DefinitionsDataSourceModel{
		ID:          types.StringValue("integration_definitions"),
		Definitions: make([]DefinitionModel, 0, len(definitions)),
	}
	for _, definition := range definitions {
		model, diags := definitionModelFromAPI(ctx, definition)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Definitions = append(data.Definitions, model)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *DefinitionsDataSource) listDefinitions(ctx context.Context) ([]*v1.IntegrationDefinition, error) {
	var definitions []*v1.IntegrationDefinition
	token := ""
	seenTokens := map[string]struct{}{}
	for {
		result, err := d.client.IntegrationService().ListIntegrationDefinitions(ctx, connect.NewRequest(&v1.ListIntegrationDefinitionsRequest{
			Pagination: &v1.PaginationRequest{PageSize: 100, Token: token},
		}))
		if err != nil {
			return nil, fmt.Errorf("list integration definitions: %w", err)
		}
		definitions = append(definitions, result.Msg.GetDefinitions()...)
		nextToken := result.Msg.GetPagination().GetNextToken()
		if nextToken == "" {
			break
		}
		if _, ok := seenTokens[nextToken]; ok {
			return nil, fmt.Errorf("list integration definitions: API repeated pagination token %q", nextToken)
		}
		seenTokens[nextToken] = struct{}{}
		token = nextToken
	}
	sort.SliceStable(definitions, func(i, j int) bool {
		return definitions[i].GetId() < definitions[j].GetId()
	})
	return definitions, nil
}
