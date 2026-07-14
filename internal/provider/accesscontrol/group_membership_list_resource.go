// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

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

var _ list.ListResource = &GroupMembershipResource{}

func NewGroupMembershipListResource() list.ListResource { return &GroupMembershipResource{} }

type groupMembershipListModel struct {
	GroupID types.String `tfsdk:"group_id"`
	Search  types.String `tfsdk:"search"`
}

func (r *GroupMembershipResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists service-account memberships in an Ona group.", Attributes: map[string]listschema.Attribute{
		"group_id": listschema.StringAttribute{Required: true, MarkdownDescription: "Group ID whose memberships are queried."},
		"search":   listschema.StringAttribute{Optional: true, MarkdownDescription: "Search member names and IDs."},
	}}
}

func (r *GroupMembershipResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_group_membership resources")))
			return
		}
		var data groupMembershipListModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &data)) {
			return
		}
		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.GroupService().ListMemberships(ctx, connect.NewRequest(&v1.ListMembershipsRequest{GroupId: data.GroupID.ValueString(), Filter: &v1.ListMembershipsRequest_Filter{Search: data.Search.ValueString()}, Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token}}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Group Memberships", fmt.Errorf("list memberships: %w", err)))
				return
			}
			members := result.Msg.GetMembers()
			sort.SliceStable(members, func(i, j int) bool { return members[i].GetSubject().GetId() < members[j].GetSubject().GetId() })
			for _, member := range members {
				if member.GetSubject().GetPrincipal() != v1.Principal_PRINCIPAL_SERVICE_ACCOUNT {
					continue
				}
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = member.GetName()
				if item.DisplayName == "" {
					item.DisplayName = member.GetSubject().GetId()
				}
				item.Diagnostics.Append(item.Identity.Set(ctx, GroupMembershipIdentityModel{GroupID: types.StringValue(member.GetGroupId()), ServiceAccountID: types.StringValue(member.GetSubject().GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model GroupMembershipModel
					populateGroupMembershipModel(&model, member)
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
