// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

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

var _ list.ListResource = &TeamMembershipResource{}

func NewTeamMembershipListResource() list.ListResource { return &TeamMembershipResource{} }

type teamMembershipListModel struct {
	TeamID types.String `tfsdk:"team_id"`
}

func (r *TeamMembershipResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists user memberships in an Ona team.",
		Attributes: map[string]listschema.Attribute{
			"team_id": listschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Team ID whose memberships are queried.",
			},
		},
	}
}

func (r *TeamMembershipResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_team_membership resources")))
			return
		}

		var data teamMembershipListModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &data)) {
			return
		}

		var members []*v1.TeamMember
		var token string
		seenTokens := make(map[string]struct{})
		for listutil.HasCapacity(req.Limit, int64(len(members))) {
			result, err := r.client.TeamService().ListTeamMembers(ctx, connect.NewRequest(&v1.ListTeamMembersRequest{
				TeamId: data.TeamID.ValueString(),
				Pagination: &v1.PaginationRequest{
					PageSize: listutil.PageSize(req.Limit, int64(len(members))),
					Token:    token,
				},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Team Memberships", fmt.Errorf("list team members: %w", err)))
				return
			}
			if result == nil || result.Msg == nil {
				push(listutil.Error("Unable to List Ona Team Memberships", fmt.Errorf("list team members: the Ona API returned an empty response")))
				return
			}

			for _, member := range result.Msg.GetMembers() {
				if !listutil.HasCapacity(req.Limit, int64(len(members))) {
					break
				}
				members = append(members, member)
			}

			nextToken := result.Msg.GetPagination().GetNextToken()
			if nextToken == "" || !listutil.HasCapacity(req.Limit, int64(len(members))) {
				break
			}
			if err := listutil.NextPageToken(seenTokens, nextToken); err != nil {
				push(listutil.Error("Unable to List Ona Team Memberships", fmt.Errorf("list team members: %w", err)))
				return
			}
			token = nextToken
		}

		sortTeamMemberships(members)
		displayNames := listutil.NewDisplayNames()
		items := make([]list.ListResult, 0, len(members))
		for _, member := range members {
			if err := validateTeamMembership(member, data.TeamID.ValueString(), ""); err != nil {
				push(listutil.Error("Unable to Map Ona Team Membership", err))
				return
			}

			var model TeamMembershipModel
			populateTeamMembershipModel(&model, member)
			identity, err := teamMembershipIdentity(model)
			if err != nil {
				push(listutil.Error("Unable to Map Ona Team Membership", err))
				return
			}

			item := req.NewListResult(ctx)
			item.DisplayName = displayNames.Unique(member.GetName(), teamMembershipDisplayNameFallback(member), "team_membership")
			item.Diagnostics.Append(item.Identity.Set(ctx, identity)...)
			if req.IncludeResource && !item.Diagnostics.HasError() {
				item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
			}
			if item.Diagnostics.HasError() {
				push(item)
				return
			}
			items = append(items, item)
		}

		for _, item := range items {
			if !push(item) {
				return
			}
		}
	}
}

func teamMembershipDisplayNameFallback(member *v1.TeamMember) string {
	return "user_" + member.GetUserId()
}

func sortTeamMemberships(members []*v1.TeamMember) {
	sort.SliceStable(members, func(i, j int) bool {
		if members[i].GetUserId() != members[j].GetUserId() {
			return members[i].GetUserId() < members[j].GetUserId()
		}
		return members[i].GetId() < members[j].GetId()
	})
}
