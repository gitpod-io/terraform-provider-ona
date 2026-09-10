// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package githubapp

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &Resource{}

func NewListResource() list.ListResource { return &Resource{} }

func (r *Resource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists organization-owned GitHub App integrations, including pending installations. Uses saved metadata only; credentials and the local credentials_version marker are omitted."}
}

func (r *Resource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("configure an Ona API token before listing GitHub App integrations")))
			return
		}
		definition, err := r.githubDefinition(ctx)
		if err != nil {
			push(listutil.Error("Unable to Resolve GitHub App Definition", err))
			return
		}
		var token string
		seen := make(map[string]struct{})
		var emitted int64
		names := listutil.NewDisplayNames()
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.IntegrationService().ListIntegrations(ctx, connect.NewRequest(&v1.ListIntegrationsRequest{Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token}}))
			if err != nil {
				push(listutil.Error("Unable to List GitHub App Integrations", safeAPIError("listing GitHub App integrations", err)))
				return
			}
			if result == nil || result.Msg == nil {
				push(listutil.Error("Unable to List GitHub App Integrations", fmt.Errorf("the Ona API returned an empty list response")))
				return
			}
			for _, remote := range result.Msg.GetIntegrations() {
				if !isOwnedGitHubApp(remote, definition) {
					continue
				}
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				if _, err := uuid.Parse(remote.GetId()); err != nil {
					push(listutil.Error("Unable to List GitHub App Integrations", fmt.Errorf("the Ona API returned an integration without a valid UUID")))
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = names.Unique(remote.GetAuth().GetProprietaryApp().GetAppSlug(), remote.GetId(), "github_app")
				item.Diagnostics.Append(item.Identity.Set(ctx, IdentityModel{IntegrationID: types.StringValue(remote.GetId())})...)
				if item.Diagnostics.HasError() {
					push(item)
					return
				}
				if req.IncludeResource {
					model := emptyModel()
					item.Diagnostics.Append(populateModel(&model, remote, definition, r.deployment)...)
					if item.Diagnostics.HasError() {
						push(item)
						return
					}
					item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
				}
				if !push(item) || item.Diagnostics.HasError() {
					return
				}
				emitted++
			}
			token = result.Msg.GetPagination().GetNextToken()
			if err := listutil.NextPageToken(seen, token); err != nil {
				push(listutil.Error("Unable to List GitHub App Integrations", fmt.Errorf("the Ona API repeated an integration pagination token")))
				return
			}
			if token == "" {
				return
			}
		}
	}
}
