// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package serviceaccount

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

var _ list.ListResource = &Resource{}

func NewListResource() list.ListResource { return &Resource{} }

type listModel struct {
	Search            types.String `tfsdk:"search"`
	ServiceAccountIDs types.List   `tfsdk:"service_account_ids"`
}

func (r *Resource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists customer-managed Ona service accounts for the authenticated organization.", Attributes: map[string]listschema.Attribute{
		"search":              listschema.StringAttribute{Optional: true, MarkdownDescription: "Search service accounts by name."},
		"service_account_ids": listschema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Service account IDs to include."},
	}}
}

func (r *Resource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_service_account resources")))
			return
		}
		var data listModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &data)) {
			return
		}
		ids, diags := listutil.StringList(ctx, data.ServiceAccountIDs)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}
		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.ServiceAccountService().ListServiceAccounts(ctx, connect.NewRequest(&v1.ListServiceAccountsRequest{
				Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token},
				Filter:     &v1.ListServiceAccountsRequest_Filter{Search: data.Search.ValueString(), ServiceAccountIds: ids},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Service Accounts", fmt.Errorf("list service accounts: %w", err)))
				return
			}
			accounts := result.Msg.GetServiceAccounts()
			sort.SliceStable(accounts, func(i, j int) bool { return accounts[i].GetId() < accounts[j].GetId() })
			for _, account := range accounts {
				if account.GetSuspended() || account.GetSystemManaged() {
					continue
				}
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = account.GetName()
				if item.DisplayName == "" {
					item.DisplayName = account.GetId()
				}
				item.Diagnostics.Append(item.Identity.Set(ctx, IdentityModel{ServiceAccountID: types.StringValue(account.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model Model
					populateModelFromServiceAccount(&model, account)
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
