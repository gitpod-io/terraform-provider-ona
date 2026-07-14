// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package secret

import (
	"context"
	"fmt"
	"sort"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &Resource{}

func NewListResource() list.ListResource { return &Resource{} }

type listModel struct {
	Scope            types.String `tfsdk:"scope"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	ProjectID        types.String `tfsdk:"project_id"`
	UserID           types.String `tfsdk:"user_id"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
}

func (r *Resource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists Ona secret metadata for one scope. Secret values are never read or returned.", Attributes: map[string]listschema.Attribute{
		"scope":              listschema.StringAttribute{Required: true, MarkdownDescription: "Secret scope: organization, project, user, or service_account."},
		"organization_id":    listschema.StringAttribute{Optional: true, MarkdownDescription: "Organization ID. Defaults to the authenticated organization."},
		"project_id":         listschema.StringAttribute{Optional: true, MarkdownDescription: "Project ID required for project scope."},
		"user_id":            listschema.StringAttribute{Optional: true, MarkdownDescription: "User ID. Defaults to the authenticated user for user scope."},
		"service_account_id": listschema.StringAttribute{Optional: true, MarkdownDescription: "Service account ID required for service_account scope."},
	}}
}

func (r *Resource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_secret resources")))
			return
		}
		var config listModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &config)) {
			return
		}
		data := Model{Scope: config.Scope, OrganizationID: config.OrganizationID, ProjectID: config.ProjectID, UserID: config.UserID, ServiceAccountID: config.ServiceAccountID}
		var resolveDiags diag.Diagnostics
		resolved := r.resolveScope(ctx, &data, &resolveDiags)
		if !listutil.PushDiagnostics(push, resolveDiags) {
			return
		}
		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.SecretService().ListSecrets(ctx, connect.NewRequest(&v1.ListSecretsRequest{Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token}, Filter: &v1.ListSecretsRequest_Filter{Scope: resolved.Scope}}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Secrets", fmt.Errorf("list secret metadata: %w", err)))
				return
			}
			secrets := result.Msg.GetSecrets()
			sort.SliceStable(secrets, func(i, j int) bool { return secrets[i].GetId() < secrets[j].GetId() })
			for _, remote := range secrets {
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				var model Model
				var itemDiags diag.Diagnostics
				populateModelFromSecret(ctx, &model, remote, &itemDiags)
				model.Value = types.StringNull()
				item := req.NewListResult(ctx)
				item.DisplayName = remote.GetName()
				if item.DisplayName == "" {
					item.DisplayName = remote.GetId()
				}
				item.Diagnostics.Append(itemDiags...)
				item.Diagnostics.Append(item.Identity.Set(ctx, identityFromModel(model))...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
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
