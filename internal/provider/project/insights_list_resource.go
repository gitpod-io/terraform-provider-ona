// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"sort"
)

var _ list.ListResource = &InsightsResource{}

func NewInsightsListResource() list.ListResource { return &InsightsResource{} }

type insightsListModel struct {
	ProjectIDs types.List `tfsdk:"project_ids"`
}

func (r *InsightsResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists Ona Project Insights status for existing projects.", Attributes: map[string]listschema.Attribute{"project_ids": listschema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Project IDs to inspect. Defaults to all projects."}}}
}
func (r *InsightsResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_project_insights resources")))
			return
		}
		var data insightsListModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &data)) {
			return
		}
		ids, diags := listutil.StringList(ctx, data.ProjectIDs)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}
		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.ProjectService().ListProjects(ctx, connect.NewRequest(&v1.ListProjectsRequest{Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token}, Filter: &v1.ListProjectsRequest_Filter{ProjectIds: ids}, Sort: &v1.Sort{Field: "id"}}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Projects", fmt.Errorf("list projects for Insights: %w", err)))
				return
			}
			projects := result.Msg.GetProjects()
			sort.SliceStable(projects, func(i, j int) bool { return projects[i].GetId() < projects[j].GetId() })
			for _, remote := range projects {
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				status, err := r.client.InsightsService().GetProjectInsightsStatus(ctx, connect.NewRequest(&v1.GetProjectInsightsStatusRequest{ProjectId: remote.GetId()}))
				if connect.CodeOf(err) == connect.CodeNotFound {
					continue
				}
				if err != nil {
					push(listutil.Error("Unable to Read Ona Project Insights", fmt.Errorf("get Insights status for project %q: %w", remote.GetId(), err)))
					return
				}
				model := ProjectInsightsModel{ID: types.StringValue(remote.GetId()), ProjectID: types.StringValue(remote.GetId()), Enabled: types.BoolValue(status.Msg.GetEnabled()), LastRanAt: projectInsightsTimestamp(status.Msg.GetLastRanAt()), DataCollectedThrough: projectInsightsTimestamp(status.Msg.GetDataCollectedThrough())}
				item := req.NewListResult(ctx)
				item.DisplayName = remote.GetMetadata().GetName()
				if item.DisplayName == "" {
					item.DisplayName = remote.GetId()
				}
				item.Diagnostics.Append(item.Identity.Set(ctx, InsightsIdentityModel{ProjectID: model.ProjectID})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
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
