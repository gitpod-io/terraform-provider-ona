// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

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

var _ list.ListResource = &SSOConfigurationResource{}

func NewSSOConfigurationListResource() list.ListResource { return &SSOConfigurationResource{} }

type ssoConfigurationListModel struct {
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (r *SSOConfigurationResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists custom Ona SSO configurations.", Attributes: map[string]listschema.Attribute{
		"organization_id": listschema.StringAttribute{Optional: true, MarkdownDescription: "Organization ID to query. Defaults to the authenticated organization."},
	}}
}

func (r *SSOConfigurationResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_sso_configuration resources")))
			return
		}
		var data ssoConfigurationListModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &data)) {
			return
		}
		organizationID := data.OrganizationID.ValueString()
		if organizationID == "" {
			var err error
			organizationID, err = listutil.AuthenticatedOrganizationID(ctx, r.client)
			if err != nil {
				push(listutil.Error("Unable to Determine Organization", err))
				return
			}
		}

		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.OrganizationService().ListSSOConfigurations(ctx, connect.NewRequest(&v1.ListSSOConfigurationsRequest{
				OrganizationId: organizationID,
				Pagination:     &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona SSO Configurations", fmt.Errorf("list SSO configurations: %w", err)))
				return
			}
			configurations := result.Msg.GetSsoConfigurations()
			sort.SliceStable(configurations, func(i, j int) bool { return configurations[i].GetId() < configurations[j].GetId() })
			for _, configuration := range configurations {
				if configuration.GetProviderType() != v1.SSOConfiguration_PROVIDER_TYPE_CUSTOM {
					continue
				}
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = configuration.GetDisplayName()
				if item.DisplayName == "" {
					item.DisplayName = configuration.GetId()
				}
				item.Diagnostics.Append(item.Identity.Set(ctx, SSOConfigurationIdentityModel{ID: types.StringValue(configuration.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model SSOConfigurationModel
					populateSSOConfigurationModel(ctx, &model, configuration, SSOConfigurationModel{}, &item.Diagnostics)
					if !item.Diagnostics.HasError() {
						item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
					}
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
