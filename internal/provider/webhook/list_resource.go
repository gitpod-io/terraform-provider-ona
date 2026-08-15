// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package webhook

import (
	"context"
	"fmt"
	"sort"

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
		MarkdownDescription: "Lists Ona webhooks that can be imported and managed by Terraform. Webhook signing secrets are never read or returned.",
	}
}

func (r *Resource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_webhook resources")))
			return
		}

		var token string
		seenTokens := make(map[string]struct{})
		var emitted int64
		displayNames := listutil.NewDisplayNames()
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.WebhookService().ListWebhooks(ctx, connect.NewRequest(&v1.ListWebhooksRequest{
				Pagination: &v1.PaginationRequest{
					PageSize: listutil.PageSize(req.Limit, emitted),
					Token:    token,
				},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Webhooks", fmt.Errorf("list webhooks: %w", err)))
				return
			}

			webhooks := result.Msg.GetWebhooks()
			sort.SliceStable(webhooks, func(i, j int) bool {
				return webhooks[i].GetId() < webhooks[j].GetId()
			})
			for _, remote := range webhooks {
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}

				item := req.NewListResult(ctx)
				item.DisplayName = displayNames.Unique(remote.GetMetadata().GetName(), remote.GetId(), "webhook")
				item.Diagnostics.Append(item.Identity.Set(ctx, IdentityModel{ID: types.StringValue(remote.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model Model
					populateModel(ctx, &model, remote, &item.Diagnostics)
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
				push(listutil.Error("Unable to List Ona Webhooks", err))
				return
			}
			if nextToken == "" {
				return
			}
			token = nextToken
		}
	}
}
