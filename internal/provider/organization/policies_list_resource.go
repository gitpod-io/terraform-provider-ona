// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"

	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &PoliciesResource{}

func NewPoliciesListResource() list.ListResource {
	return &PoliciesResource{}
}

type policiesListModel struct {
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (r *PoliciesResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists the singleton Ona organization policies resource.",
		Attributes: map[string]listschema.Attribute{
			"organization_id": listschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Organization ID to list policies for. Defaults to the authenticated organization.",
			},
		},
	}
}

func (r *PoliciesResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_organization_policies resources")))
			return
		}

		var data policiesListModel
		diags := req.Config.Get(ctx, &data)
		if !listutil.PushDiagnostics(push, diags) {
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

		policies, err := r.getPolicies(ctx, organizationID)
		if err != nil {
			push(listutil.Error("Unable to Read Ona Organization Policies", err))
			return
		}

		item := req.NewListResult(ctx)
		item.DisplayName = organizationID
		item.Diagnostics.Append(item.Identity.Set(ctx, PoliciesIdentityModel{
			OrganizationID: types.StringValue(organizationID),
		})...)
		if req.IncludeResource && !item.Diagnostics.HasError() {
			var model PoliciesModel
			item.Diagnostics.Append(populatePoliciesModel(ctx, &model, policies, PoliciesModel{}, true)...)
			if !item.Diagnostics.HasError() {
				item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
			}
		}
		push(item)
	}
}
