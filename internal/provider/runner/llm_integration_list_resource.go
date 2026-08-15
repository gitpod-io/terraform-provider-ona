// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

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

var _ list.ListResource = &LLMIntegrationResource{}

func NewLLMIntegrationListResource() list.ListResource {
	return &LLMIntegrationResource{}
}

type llmIntegrationListModel struct {
	RunnerIDs types.List `tfsdk:"runner_ids"`
}

func (r *LLMIntegrationResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists importable Ona runner LLM integrations without exposing API key values.",
		Attributes: map[string]listschema.Attribute{
			"runner_ids": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Runner IDs to include.",
			},
		},
	}
}

func (r *LLMIntegrationResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_runner_llm_integration resources")))
			return
		}

		var config llmIntegrationListModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &config)) {
			return
		}
		runnerIDs, diags := listutil.StringList(ctx, config.RunnerIDs)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}

		var token string
		seenTokens := make(map[string]struct{})
		var emitted int64
		displayNames := listutil.NewDisplayNames()
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.RunnerConfigurationService().ListLLMIntegrations(ctx, connect.NewRequest(&v1.ListLLMIntegrationsRequest{
				Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token},
				Filter:     &v1.ListLLMIntegrationsRequest_Filter{RunnerIds: runnerIDs},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Runner LLM Integrations", fmt.Errorf("list runner LLM integrations: %w", err)))
				return
			}

			integrations := result.Msg.GetIntegrations()
			sort.SliceStable(integrations, func(i, j int) bool {
				return integrations[i].GetId() < integrations[j].GetId()
			})
			for _, integration := range integrations {
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				if integration.GetId() == "" {
					push(listutil.Error("Unable to List Ona Runner LLM Integrations", fmt.Errorf("received a runner LLM integration without an ID")))
					return
				}

				item := req.NewListResult(ctx)
				item.DisplayName = displayNames.Unique(llmIntegrationDisplayName(integration), integration.GetId(), "runner_llm_integration")
				item.Diagnostics.Append(item.Identity.Set(ctx, LLMIntegrationIdentityModel{ID: types.StringValue(integration.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model LLMIntegrationModel
					item.Diagnostics.Append(populateLLMIntegrationModel(ctx, &model, integration)...)
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
				push(listutil.Error("Unable to List Ona Runner LLM Integrations", err))
				return
			}
			if nextToken == "" {
				return
			}
			token = nextToken
		}
	}
}

func llmIntegrationDisplayName(integration *v1.LLMIntegration) string {
	if provider := llmProviderToString(integration.GetProvider()); provider != "" {
		return provider
	}
	if integration.GetEndpoint() != "" {
		return integration.GetEndpoint()
	}
	return integration.GetRunnerId()
}
