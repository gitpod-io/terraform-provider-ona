// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

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

var _ list.ListResource = &GroupResource{}

func NewGroupListResource() list.ListResource { return &GroupResource{} }

type groupListModel struct {
	Search   types.String `tfsdk:"search"`
	GroupIDs types.List   `tfsdk:"group_ids"`
}

func (r *GroupResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists customer-managed Ona groups for the authenticated organization.", Attributes: map[string]listschema.Attribute{
		"search":    listschema.StringAttribute{Optional: true, MarkdownDescription: "Case-insensitive search across group name, description, and ID."},
		"group_ids": listschema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Group IDs to include."},
	}}
}

func (r *GroupResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_group resources")))
			return
		}
		var data groupListModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &data)) {
			return
		}
		ids, diags := listutil.StringList(ctx, data.GroupIDs)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}
		custom, direct := false, false
		filter := &v1.ListGroupsRequest_Filter{Search: data.Search.ValueString(), GroupIds: ids, SystemManaged: &custom, DirectShare: &direct}
		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.GroupService().ListGroups(ctx, connect.NewRequest(&v1.ListGroupsRequest{Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token}, Filter: filter}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Groups", fmt.Errorf("list groups: %w", err)))
				return
			}
			groups := result.Msg.GetGroups()
			sort.SliceStable(groups, func(i, j int) bool { return groups[i].GetId() < groups[j].GetId() })
			for _, group := range groups {
				if group.GetSystemManaged() || group.GetDirectShare() {
					continue
				}
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = group.GetName()
				if item.DisplayName == "" {
					item.DisplayName = group.GetId()
				}
				item.Diagnostics.Append(item.Identity.Set(ctx, GroupIdentityModel{ID: types.StringValue(group.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model GroupModel
					populateGroupModel(&model, group)
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
