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

var _ list.ListResource = &TermsOfServiceResource{}

func NewTermsOfServiceListResource() list.ListResource { return &TermsOfServiceResource{} }

type termsOfServiceListModel struct {
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (r *TermsOfServiceResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists the singleton Ona Terms of Service configuration when it is configured.",
		Attributes: map[string]listschema.Attribute{
			"organization_id": listschema.StringAttribute{Optional: true, MarkdownDescription: "Organization ID to query. Defaults to the authenticated organization."},
		},
	}
}

func (r *TermsOfServiceResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_terms_of_service resources")))
			return
		}
		if !listutil.HasCapacity(req.Limit, 0) {
			return
		}

		var data termsOfServiceListModel
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

		terms, err := r.getTermsOfService(ctx, organizationID)
		if connect.CodeOf(err) == connect.CodeNotFound {
			return
		}
		if err != nil {
			push(listutil.Error("Unable to Read Ona Terms of Service", err))
			return
		}
		if terms == nil || (!terms.GetEnabled() && terms.GetCurrentVersion() == nil) {
			return
		}

		item := req.NewListResult(ctx)
		item.DisplayName = organizationID
		item.Diagnostics.Append(item.Identity.Set(ctx, TermsOfServiceIdentityModel{OrganizationID: types.StringValue(organizationID)})...)
		if req.IncludeResource && !item.Diagnostics.HasError() {
			var model TermsOfServiceModel
			item.Diagnostics.Append(populateTermsOfServiceModel(&model, terms, organizationID, TermsOfServiceModel{})...)
			if !item.Diagnostics.HasError() {
				item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
			}
		}
		push(item)
	}
}
