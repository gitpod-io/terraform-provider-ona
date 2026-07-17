// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &OIDCConfigResource{}

func NewOIDCConfigListResource() list.ListResource { return &OIDCConfigResource{} }

type oidcConfigListModel struct {
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (r *OIDCConfigResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists the singleton Ona OIDC token-format configuration when it is configured.", Attributes: map[string]listschema.Attribute{
		"organization_id": listschema.StringAttribute{Optional: true, MarkdownDescription: "Organization ID to query. Defaults to the authenticated organization."},
	}}
}

func (r *OIDCConfigResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_oidc_config resources")))
			return
		}
		if !listutil.HasCapacity(req.Limit, 0) {
			return
		}
		var data oidcConfigListModel
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
		result, err := r.client.OrganizationService().GetOIDCConfig(ctx, connect.NewRequest(&v1.GetOIDCConfigRequest{OrganizationId: organizationID}))
		if connect.CodeOf(err) == connect.CodeNotFound {
			return
		}
		if err != nil {
			push(listutil.Error("Unable to Read Ona OIDC Config", fmt.Errorf("get OIDC config: %w", err)))
			return
		}
		if result.Msg.GetOidcConfig() == nil {
			return
		}

		item := req.NewListResult(ctx)
		item.DisplayName = organizationID
		item.Diagnostics.Append(item.Identity.Set(ctx, OIDCConfigIdentityModel{OrganizationID: types.StringValue(organizationID)})...)
		if req.IncludeResource && !item.Diagnostics.HasError() {
			var model OIDCConfigModel
			populateOIDCConfigModel(ctx, &model, organizationID, result.Msg.GetOidcConfig(), OIDCConfigModel{}, &item.Diagnostics)
			if !item.Diagnostics.HasError() {
				item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
			}
		}
		push(item)
	}
}
