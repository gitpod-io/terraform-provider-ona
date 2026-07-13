// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

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

func NewRunnerPolicyListResource() list.ListResource {
	return &PolicyResource{}
}

type runnerPolicyListModel struct {
	RunnerIDs types.List `tfsdk:"runner_ids"`
}

func (r *PolicyResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists supported Ona runner group-access policies.",
		Attributes: map[string]listschema.Attribute{
			"runner_ids": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Runner IDs whose policies should be listed. When omitted, all importable runners are enumerated.",
			},
		},
	}
}

func (r *PolicyResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_runner_policy resources")))
			return
		}

		var config runnerPolicyListModel
		diags := req.Config.Get(ctx, &config)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}
		runnerIDs, diags := listutil.StringList(ctx, config.RunnerIDs)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}
		if len(runnerIDs) == 0 {
			var err error
			runnerIDs, err = r.listImportableRunnerIDs(ctx)
			if err != nil {
				push(listutil.Error("Unable to List Ona Runners", err))
				return
			}
		}
		sort.Strings(runnerIDs)

		var emitted int64
		for _, runnerID := range runnerIDs {
			var token string
			for listutil.HasCapacity(req.Limit, emitted) {
				result, err := r.client.RunnerService().ListRunnerPolicies(ctx, connect.NewRequest(&v1.ListRunnerPoliciesRequest{
					RunnerId: runnerID,
					Pagination: &v1.PaginationRequest{
						PageSize: listutil.PageSize(req.Limit, emitted),
						Token:    token,
					},
				}))
				if err != nil {
					push(listutil.Error("Unable to List Ona Runner Policies", fmt.Errorf("list policies for runner %q: %w", runnerID, err)))
					return
				}

				policies := result.Msg.GetPolicies()
				sort.SliceStable(policies, func(i, j int) bool { return policies[i].GetGroupId() < policies[j].GetGroupId() })
				for _, policy := range policies {
					if !listutil.HasCapacity(req.Limit, emitted) {
						return
					}
					if _, ok := apiToRunnerPolicyRole[policy.GetRole()]; !ok {
						continue
					}
					item := req.NewListResult(ctx)
					item.DisplayName = fmt.Sprintf("%s / %s", runnerID, policy.GetGroupId())
					item.Diagnostics.Append(item.Identity.Set(ctx, RunnerPolicyIdentityModel{
						RunnerID: types.StringValue(runnerID),
						GroupID:  types.StringValue(policy.GetGroupId()),
					})...)
					if req.IncludeResource && !item.Diagnostics.HasError() {
						var model PolicyModel
						item.Diagnostics.Append(populateRunnerPolicyModel(&model, runnerID, policy)...)
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
					break
				}
			}
		}
	}
}

func (r *PolicyResource) listImportableRunnerIDs(ctx context.Context) ([]string, error) {
	var token string
	var runnerIDs []string
	for {
		result, err := r.client.RunnerService().ListRunners(ctx, connect.NewRequest(&v1.ListRunnersRequest{
			Pagination: &v1.PaginationRequest{PageSize: listutil.DefaultPageSize, Token: token},
			Filter: &v1.ListRunnersRequest_Filter{
				Kinds:     []v1.RunnerKind{v1.RunnerKind_RUNNER_KIND_REMOTE},
				Providers: []v1.RunnerProvider{v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2, v1.RunnerProvider_RUNNER_PROVIDER_GCP},
			},
		}))
		if err != nil {
			return nil, fmt.Errorf("list runners for policy discovery: %w", err)
		}
		for _, remoteRunner := range result.Msg.GetRunners() {
			if remoteRunner.GetKind() != v1.RunnerKind_RUNNER_KIND_REMOTE {
				continue
			}
			if remoteRunner.GetProvider() != v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2 && remoteRunner.GetProvider() != v1.RunnerProvider_RUNNER_PROVIDER_GCP {
				continue
			}
			runnerIDs = append(runnerIDs, remoteRunner.GetRunnerId())
		}
		token = result.Msg.GetPagination().GetNextToken()
		if token == "" {
			return runnerIDs, nil
		}
	}
}
