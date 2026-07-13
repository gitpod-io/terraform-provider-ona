// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package security

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

var _ list.ListResource = &PolicyResource{}

func NewPolicyListResource() list.ListResource {
	return &PolicyResource{}
}

type policyListModel struct {
	OrganizationID    types.String `tfsdk:"organization_id"`
	Search            types.String `tfsdk:"search"`
	SecurityPolicyIDs types.List   `tfsdk:"security_policy_ids"`
}

func (r *PolicyResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists Ona security policies.",
		Attributes: map[string]listschema.Attribute{
			"organization_id": listschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Organization ID to list security policies for. Defaults to the authenticated organization.",
			},
			"search": listschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Search string for filtering security policies by name.",
			},
			"security_policy_ids": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Security policy IDs to include. The API accepts at most 25 IDs.",
			},
		},
	}
}

func (r *PolicyResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_security_policy resources")))
			return
		}

		filter, ok := policyListFilter(ctx, r, req, push)
		if !ok {
			return
		}

		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.SecurityService().ListSecurityPolicies(ctx, connect.NewRequest(&v1.ListSecurityPoliciesRequest{
				Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token},
				Filter:     filter,
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Security Policies", fmt.Errorf("list security policies: %w", err)))
				return
			}

			policies := result.Msg.GetSecurityPolicies()
			sort.SliceStable(policies, func(i, j int) bool { return policies[i].GetId() < policies[j].GetId() })
			for _, policy := range policies {
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = policy.GetMetadata().GetName()
				if item.DisplayName == "" {
					item.DisplayName = policy.GetId()
				}
				item.Diagnostics.Append(item.Identity.Set(ctx, PolicyIdentityModel{ID: types.StringValue(policy.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model PolicyModel
					item.Diagnostics.Append(populatePolicyModel(ctx, &model, policy)...)
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

func policyListFilter(ctx context.Context, r *PolicyResource, req list.ListRequest, push func(list.ListResult) bool) (*v1.ListSecurityPoliciesRequest_Filter, bool) {
	var data policyListModel
	diags := req.Config.Get(ctx, &data)
	if !listutil.PushDiagnostics(push, diags) {
		return nil, false
	}

	ids, diags := listutil.StringList(ctx, data.SecurityPolicyIDs)
	if !listutil.PushDiagnostics(push, diags) {
		return nil, false
	}
	if len(ids) > 25 {
		push(listutil.Error("Too Many Security Policy IDs", fmt.Errorf("the Ona API accepts at most 25 security_policy_ids")))
		return nil, false
	}

	organizationID := data.OrganizationID.ValueString()
	if organizationID == "" {
		var err error
		organizationID, err = listutil.AuthenticatedOrganizationID(ctx, r.client)
		if err != nil {
			push(listutil.Error("Unable to Determine Organization", err))
			return nil, false
		}
	}

	return &v1.ListSecurityPoliciesRequest_Filter{
		OrganizationId:    organizationID,
		Search:            data.Search.ValueString(),
		SecurityPolicyIds: ids,
	}, true
}
