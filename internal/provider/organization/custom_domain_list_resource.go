// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &CustomDomainResource{}

func NewCustomDomainListResource() list.ListResource { return &CustomDomainResource{} }

type customDomainListModel struct {
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (r *CustomDomainResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists the singleton Ona custom domain when it is configured.",
		Attributes: map[string]listschema.Attribute{
			"organization_id": listschema.StringAttribute{Optional: true, MarkdownDescription: "Organization ID to query. Defaults to the authenticated organization."},
		},
	}
}

func (r *CustomDomainResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_custom_domain resources")))
			return
		}
		if !listutil.HasCapacity(req.Limit, 0) {
			return
		}
		var data customDomainListModel
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
		customDomain, err := r.getCustomDomain(ctx, organizationID)
		if connect.CodeOf(err) == connect.CodeNotFound {
			return
		}
		if err != nil {
			push(listutil.Error("Unable to Read Ona Custom Domain", err))
			return
		}

		item := req.NewListResult(ctx)
		item.DisplayName = customDomain.GetDomainName()
		if item.DisplayName == "" {
			item.DisplayName = organizationID
		}
		item.Diagnostics.Append(item.Identity.Set(ctx, CustomDomainIdentityModel{OrganizationID: types.StringValue(organizationID)})...)
		if req.IncludeResource && !item.Diagnostics.HasError() {
			var model CustomDomainModel
			item.Diagnostics.Append(populateCustomDomainModel(&model, customDomain, organizationID)...)
			if !item.Diagnostics.HasError() {
				item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
			}
		}
		push(item)
	}
}
