// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &Resource{}

func NewListResource() list.ListResource { return &Resource{} }

type listModel struct {
	Search              types.String `tfsdk:"search"`
	RepositoryCloneURLs types.List   `tfsdk:"repository_clone_urls"`
}

func (r *Resource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists Ona projects that can be managed by Terraform. Projects must use a Git repository initializer with a non-empty clone URL and branch, and must have at least one environment class; other projects are excluded.", Attributes: map[string]listschema.Attribute{
		"search":                listschema.StringAttribute{Optional: true, MarkdownDescription: "Optional case-insensitive search across project names, project IDs, and repository names. Set this to a free-text value inside the list block's `config` block; the provider passes the value to the Ona Projects API."},
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
		urls, diags := listutil.StringList(ctx, data.RepositoryCloneURLs)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}
		var token string
		seenTokens := make(map[string]struct{})
		var emitted int64
		displayNames := newProjectDisplayNames()
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.ProjectService().ListProjects(ctx, connect.NewRequest(&v1.ListProjectsRequest{Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token}, Filter: &v1.ListProjectsRequest_Filter{Search: data.Search.ValueString(), SpecRemoteUris: urls}, Sort: &v1.Sort{Field: "id", Order: v1.SortOrder_SORT_ORDER_ASC}}))
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
				_, repositoryDiags := repositoryFromInitializer(remote.GetInitializer())
				if isUnsupportedProjectRepository(repositoryDiags) {
					continue
				}
				if repositoryDiags.HasError() {
					push(list.ListResult{Diagnostics: repositoryDiags})
					return
				}
				environmentClasses, err := r.listProjectEnvironmentClasses(ctx, remote.GetId())
				if err != nil {
					push(listutil.Error("Unable to List Ona Project Environment Classes", fmt.Errorf("list environment classes for project %q: %w", remote.GetId(), err)))
					return
				}
				if len(environmentClasses) == 0 {
					continue
				}
				remote.EnvironmentClasses = environmentClasses
				model, mappingDiags := projectModelFromProto(ctx, remote)
				if mappingDiags.HasError() {
					push(list.ListResult{Diagnostics: mappingDiags})
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = displayNames.forProject(remote)
				item.Diagnostics.Append(item.Identity.Set(ctx, IdentityModel{ID: types.StringValue(remote.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					model.InsightsEnabled, err = r.insightsEnabled(ctx, remote.GetId())
					if err != nil {
						push(listutil.Error("Unable to Read Ona Project Insights", fmt.Errorf("read project %q Insights status: %w", remote.GetId(), err)))
						return
					}
					item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
				}
				if !push(item) || item.Diagnostics.HasError() {
					return
				}
				emitted++
			}
			nextToken := result.Msg.GetPagination().GetNextToken()
			if err := listutil.NextPageToken(seenTokens, nextToken); err != nil {
				push(listutil.Error("Unable to List Ona Projects", err))
				return
			}
			if nextToken == "" {
				return
			}
			token = nextToken
		}
	}
}

func (r *Resource) listProjectEnvironmentClasses(ctx context.Context, projectID string) ([]*v1.ProjectEnvironmentClass, error) {
	var classes []*v1.ProjectEnvironmentClass
	var token string
	seenTokens := make(map[string]struct{})
	for {
		result, err := r.client.ProjectService().ListProjectEnvironmentClasses(ctx, connect.NewRequest(&v1.ListProjectEnvironmentClassesRequest{
			Pagination: &v1.PaginationRequest{PageSize: listutil.DefaultPageSize, Token: token},
			ProjectId:  projectID,
		}))
		if err != nil {
			return nil, fmt.Errorf("list project environment classes: %w", err)
		}
		classes = append(classes, result.Msg.GetProjectEnvironmentClasses()...)
		nextToken := result.Msg.GetPagination().GetNextToken()
		if err := listutil.NextPageToken(seenTokens, nextToken); err != nil {
			return nil, fmt.Errorf("list project environment classes: %w", err)
		}
		if nextToken == "" {
			return classes, nil
		}
		token = nextToken
	}
}

func isUnsupportedProjectRepository(diags diag.Diagnostics) bool {
	if len(diags) != 1 {
		return false
	}
	_, ok := diags[0].(unsupportedProjectRepositoryDiagnostic)
	return ok
}

type projectDisplayNames struct {
	used map[string]struct{}
}

func newProjectDisplayNames() projectDisplayNames {
	return projectDisplayNames{used: map[string]struct{}{}}
}

func (n projectDisplayNames) forProject(project *v1.Project) string {
	preferred := project.GetId()
	if project.GetMetadata() != nil {
		preferred = project.GetMetadata().GetName()
	}
	base := projectDisplayNameLabel(preferred)
	if base == "" {
		id := project.GetId()
		base = "r_" + strings.ReplaceAll(id[:min(len(id), 8)], "-", "_")
	}

	candidate := base
	for i := 2; ; i++ {
		if _, ok := n.used[candidate]; !ok {
			break
		}
		candidate = fmt.Sprintf("%s_%d", base, i)
	}
	n.used[candidate] = struct{}{}
	return candidate
}

var projectDisplayNameInvalidChars = regexp.MustCompile(`[^a-z0-9_]+`)

func projectDisplayNameLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = projectDisplayNameInvalidChars.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return ""
	}
	if value[0] >= '0' && value[0] <= '9' {
		value = "r_" + value
	}
	return value
}
