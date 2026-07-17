// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

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

var _ list.ListResource = &SCIMConfigurationResource{}

func NewSCIMConfigurationListResource() list.ListResource { return &SCIMConfigurationResource{} }

func (r *SCIMConfigurationResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists Ona SCIM provisioning configurations for the authenticated organization."}
}

func (r *SCIMConfigurationResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_scim_configuration resources")))
			return
		}
		organizationID, err := listutil.AuthenticatedOrganizationID(ctx, r.client)
		if err != nil {
			push(listutil.Error("Unable to Determine Organization", err))
			return
		}
		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.OrganizationService().ListSCIMConfigurations(ctx, connect.NewRequest(&v1.ListSCIMConfigurationsRequest{
				Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona SCIM Configurations", fmt.Errorf("list SCIM configurations: %w", err)))
				return
			}
			configurations := result.Msg.GetScimConfigurations()
			sort.SliceStable(configurations, func(i, j int) bool { return configurations[i].GetId() < configurations[j].GetId() })
			for _, configuration := range configurations {
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				item := req.NewListResult(ctx)
				if !validateSCIMOrganizationScope(&item.Diagnostics, configuration, organizationID) {
					push(item)
					return
				}
				item.DisplayName = configuration.GetName()
				if item.DisplayName == "" {
					item.DisplayName = configuration.GetId()
				}
				item.Diagnostics.Append(item.Identity.Set(ctx, SCIMConfigurationIdentityModel{ID: types.StringValue(configuration.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model SCIMConfigurationModel
					populateSCIMConfigurationModel(&model, configuration, SCIMConfigurationModel{})
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
