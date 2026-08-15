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

var _ list.ListResource = &TeamResource{}

func NewTeamListResource() list.ListResource { return &TeamResource{} }

func (r *TeamResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists Ona teams visible to the authenticated provider token."}
}

func (r *TeamResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_team resources")))
			return
		}

		var token string
		seenTokens := make(map[string]struct{})
		var emitted int64
		displayNames := listutil.NewDisplayNames()
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.TeamService().ListTeams(ctx, connect.NewRequest(&v1.ListTeamsRequest{
				Pagination: &v1.PaginationRequest{
					PageSize: listutil.PageSize(req.Limit, emitted),
					Token:    token,
				},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Teams", fmt.Errorf("list teams: %w", err)))
				return
			}
			if result == nil || result.Msg == nil {
				push(listutil.Error("Unable to List Ona Teams", fmt.Errorf("the Ona API returned an empty response")))
				return
			}

			teams := result.Msg.GetTeams()
			sort.SliceStable(teams, func(i, j int) bool { return teams[i].GetId() < teams[j].GetId() })
			for _, team := range teams {
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				if team == nil || team.GetId() == "" {
					push(listutil.Error("Unable to List Ona Teams", fmt.Errorf("the Ona API returned a team without an ID")))
					return
				}

				item := req.NewListResult(ctx)
				item.DisplayName = displayNames.Unique(team.GetName(), team.GetId(), "team")
				item.Diagnostics.Append(item.Identity.Set(ctx, TeamIdentityModel{ID: types.StringValue(team.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model TeamModel
					populateTeamModel(&model, team)
					item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
				}
				if !push(item) || item.Diagnostics.HasError() {
					return
				}
				emitted++
			}

			nextToken := result.Msg.GetPagination().GetNextToken()
			if err := listutil.NextPageToken(seenTokens, nextToken); err != nil {
				push(listutil.Error("Unable to List Ona Teams", err))
				return
			}
			if nextToken == "" {
				return
			}
			token = nextToken
		}
	}
}
