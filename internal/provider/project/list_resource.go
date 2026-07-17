// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"sort"
)

var _ list.ListResource = &Resource{}

func NewListResource() list.ListResource { return &Resource{} }

type listModel struct {
	Search              types.String `tfsdk:"search"`
	ProjectIDs          types.List   `tfsdk:"project_ids"`
	RepositoryCloneURLs types.List   `tfsdk:"repository_clone_urls"`
}

func (r *Resource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists Terraform-compatible Ona projects.", Attributes: map[string]listschema.Attribute{
		"search":                listschema.StringAttribute{Optional: true, MarkdownDescription: "Search project names, IDs, and repositories."},
		"project_ids":           listschema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Project IDs to include."},
		"repository_clone_urls": listschema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Exact repository clone URLs to include."},
	}}
}
func (r *Resource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_project resources")))
			return
		}
		var data listModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &data)) {
			return
		}
		ids, diags := listutil.StringList(ctx, data.ProjectIDs)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}
		urls, diags := listutil.StringList(ctx, data.RepositoryCloneURLs)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}
		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.ProjectService().ListProjects(ctx, connect.NewRequest(&v1.ListProjectsRequest{Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token}, Filter: &v1.ListProjectsRequest_Filter{Search: data.Search.ValueString(), ProjectIds: ids, SpecRemoteUris: urls}, Sort: &v1.Sort{Field: "id"}}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Projects", fmt.Errorf("list projects: %w", err)))
				return
			}
			projects := result.Msg.GetProjects()
			sort.SliceStable(projects, func(i, j int) bool { return projects[i].GetId() < projects[j].GetId() })
			for _, remote := range projects {
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				model, mappingDiags := projectModelFromProto(ctx, remote)
				if isUnsupportedProjectRepository(mappingDiags) {
					continue
				}
				if mappingDiags.HasError() {
					push(list.ListResult{Diagnostics: mappingDiags})
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = remote.GetMetadata().GetName()
				if item.DisplayName == "" {
					item.DisplayName = remote.GetId()
				}
				item.Diagnostics.Append(item.Identity.Set(ctx, IdentityModel{ID: types.StringValue(remote.GetId())})...)
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

func isUnsupportedProjectRepository(diags diag.Diagnostics) bool {
	if len(diags) != 1 {
		return false
	}
	_, ok := diags[0].(unsupportedProjectRepositoryDiagnostic)
	return ok
}
