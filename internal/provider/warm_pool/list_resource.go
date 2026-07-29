// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package warmpool

import (
	"context"
	"fmt"
	"sort"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &WarmPoolResource{}

func NewWarmPoolListResource() list.ListResource {
	return &WarmPoolResource{}
}

type warmPoolListModel struct {
	ProjectIDs          types.List `tfsdk:"project_ids"`
	EnvironmentClassIDs types.List `tfsdk:"environment_class_ids"`
}

func (r *WarmPoolResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists importable Ona warm pools.",
		Attributes: map[string]listschema.Attribute{
			"project_ids": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Project IDs to include.",
			},
			"environment_class_ids": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Environment class IDs to include.",
			},
		},
	}
}

func (r *WarmPoolResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_warm_pool resources")))
			return
		}

		var config warmPoolListModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &config)) {
			return
		}
		projectIDs, diags := listutil.StringList(ctx, config.ProjectIDs)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}
		environmentClassIDs, diags := listutil.StringList(ctx, config.EnvironmentClassIDs)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}

		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.PrebuildService().ListWarmPools(ctx, connect.NewRequest(&v1.ListWarmPoolsRequest{
				Pagination: &v1.PaginationRequest{
					PageSize: listutil.PageSize(req.Limit, emitted),
					Token:    token,
				},
				Filter: &v1.ListWarmPoolsRequest_Filter{
					ProjectIds:          projectIDs,
					EnvironmentClassIds: environmentClassIDs,
				},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Warm Pools", fmt.Errorf("list warm pools: %w", err)))
				return
			}

			warmPools := result.Msg.GetWarmPools()
			sort.SliceStable(warmPools, func(i, j int) bool {
				return warmPools[i].GetId() < warmPools[j].GetId()
			})
			for _, remote := range warmPools {
				if warmPoolIsDeleted(remote) {
					continue
				}
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}

				item := req.NewListResult(ctx)
				item.DisplayName = remote.GetId()
				item.Diagnostics.Append(item.Identity.Set(ctx, IdentityModel{
					ID: types.StringValue(remote.GetId()),
				})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model WarmPoolModel
					populateWarmPoolModel(&model, remote)
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
