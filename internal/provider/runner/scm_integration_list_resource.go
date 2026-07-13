// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"
	"fmt"
	"sort"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &SCMIntegrationResource{}

func NewSCMIntegrationListResource() list.ListResource {
	return &SCMIntegrationResource{}
}

type scmIntegrationListModel struct {
	RunnerIDs types.List `tfsdk:"runner_ids"`
}

func (r *SCMIntegrationResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists Ona runner SCM integrations without retrieving OAuth or PAT secret values.",
		Attributes: map[string]listschema.Attribute{
			"runner_ids": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Runner IDs to include.",
			},
		},
	}
}

func (r *SCMIntegrationResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_scm_integration resources")))
			return
		}

		var config scmIntegrationListModel
		diags := req.Config.Get(ctx, &config)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}
		runnerIDs, diags := listutil.StringList(ctx, config.RunnerIDs)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}

		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.RunnerConfigurationService().ListSCMIntegrations(ctx, connect.NewRequest(&v1.ListSCMIntegrationsRequest{
				Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token},
				Filter:     &v1.ListSCMIntegrationsRequest_Filter{RunnerIds: runnerIDs},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona SCM Integrations", fmt.Errorf("list SCM integrations: %w", err)))
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
				item := req.NewListResult(ctx)
				item.DisplayName = scmIntegrationDisplayName(integration)
				item.Diagnostics.Append(item.Identity.Set(ctx, SCMIntegrationIdentityModel{ID: types.StringValue(integration.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model SCMIntegrationModel
					populateSCMIntegrationModel(&model, integration)
					item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
				}
				if !push(item) || item.Diagnostics.HasError() {
					return
				}
				emitted++
			}

			token = result.Msg.GetPagination().GetNextToken()
			if token == "" {
				return
			}
		}
	}
}

func scmIntegrationDisplayName(integration *v1.SCMIntegration) string {
	if integration.GetHost() == "" {
		return integration.GetId()
	}
	if integration.GetScmId() == "" {
		return integration.GetHost()
	}
	return fmt.Sprintf("%s (%s)", integration.GetHost(), integration.GetScmId())
}
