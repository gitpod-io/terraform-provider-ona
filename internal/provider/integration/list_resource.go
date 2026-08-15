// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package integration

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &Resource{}

func NewListResource() list.ListResource { return &Resource{} }

func (r *Resource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists all Ona organization integrations, including runner-scoped and externally installed integrations. Credential values are omitted.",
	}
}

func (r *Resource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_integration resources")))
			return
		}

		var token string
		seenTokens := make(map[string]struct{})
		var emitted int64
		displayNames := listutil.NewDisplayNames()
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.IntegrationService().ListIntegrations(ctx, connect.NewRequest(&v1.ListIntegrationsRequest{
				Pagination: &v1.PaginationRequest{
					PageSize: listutil.PageSize(req.Limit, emitted),
					Token:    token,
				},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Integrations", fmt.Errorf("list integrations: %w", err)))
				return
			}
			if result == nil || result.Msg == nil {
				push(listutil.Error("Unable to List Ona Integrations", fmt.Errorf("the Ona API returned an empty response")))
				return
			}

			for _, integration := range result.Msg.GetIntegrations() {
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				if integration.GetId() == "" {
					push(listutil.Error("Unable to List Ona Integrations", fmt.Errorf("the Ona API returned an integration without an ID")))
					return
				}

				item := req.NewListResult(ctx)
				item.DisplayName = displayNames.Unique(integration.GetName(), integration.GetId(), "integration")
				item.Diagnostics.Append(item.Identity.Set(ctx, IdentityModel{IntegrationID: types.StringValue(integration.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model Model
					item.Diagnostics.Append(populateModel(ctx, &model, integration, types.ObjectNull(authResourceAttributeTypes))...)
					if !item.Diagnostics.HasError() {
						item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
					}
				}
				if !push(item) || item.Diagnostics.HasError() {
					return
				}
				emitted++
			}

			nextToken := result.Msg.GetPagination().GetNextToken()
			if err := listutil.NextPageToken(seenTokens, nextToken); err != nil {
				push(listutil.Error("Unable to List Ona Integrations", err))
				return
			}
			if nextToken == "" {
				return
			}
			token = nextToken
		}
	}
}
