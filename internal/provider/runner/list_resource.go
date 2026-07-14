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

var _ list.ListResource = &Resource{}

func NewRunnerListResource() list.ListResource {
	return &Resource{}
}

type runnerListModel struct {
	CreatorIDs      types.List `tfsdk:"creator_ids"`
	RunnerProviders types.List `tfsdk:"runner_providers"`
}

func (r *Resource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists importable Ona runners.",
		Attributes: map[string]listschema.Attribute{
			"creator_ids": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Creator subject IDs to include.",
			},
			"runner_providers": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Runner providers to include. Supported values are `aws_ec2` and `gcp`.",
			},
		},
	}
}

func (r *Resource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_runner resources")))
			return
		}

		filter, ok := runnerListFilter(ctx, req, push)
		if !ok {
			return
		}

		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.RunnerService().ListRunners(ctx, connect.NewRequest(&v1.ListRunnersRequest{
				Pagination: &v1.PaginationRequest{
					PageSize: listutil.PageSize(req.Limit, emitted),
					Token:    token,
				},
				Filter: filter,
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Runners", fmt.Errorf("list runners: %w", err)))
				return
			}

			runners := result.Msg.GetRunners()
			sort.SliceStable(runners, func(i, j int) bool {
				return runners[i].GetRunnerId() < runners[j].GetRunnerId()
			})
			for _, remoteRunner := range runners {
				if !importableRunner(remoteRunner) {
					continue
				}
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}

				item := req.NewListResult(ctx)
				item.DisplayName = remoteRunner.GetName()
				if item.DisplayName == "" {
					item.DisplayName = remoteRunner.GetRunnerId()
				}
				item.Diagnostics.Append(item.Identity.Set(ctx, RunnerIdentityModel{
					RunnerID: types.StringValue(remoteRunner.GetRunnerId()),
				})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model RunnerModel
					populateModelFromRunner(&model, remoteRunner)
					model.RunnerManagerID = types.StringNull()
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

func runnerListFilter(ctx context.Context, req list.ListRequest, push func(list.ListResult) bool) (*v1.ListRunnersRequest_Filter, bool) {
	var data runnerListModel
	diags := req.Config.Get(ctx, &data)
	if !listutil.PushDiagnostics(push, diags) {
		return nil, false
	}

	creatorIDs, diags := listutil.StringList(ctx, data.CreatorIDs)
	if !listutil.PushDiagnostics(push, diags) {
		return nil, false
	}

	providerNames, diags := listutil.StringList(ctx, data.RunnerProviders)
	if !listutil.PushDiagnostics(push, diags) {
		return nil, false
	}
	providers, err := runnerProvidersFromNames(providerNames)
	if err != nil {
		push(listutil.Error("Invalid Runner Provider", err))
		return nil, false
	}
	if len(providers) == 0 {
		providers = importableRunnerProviders()
	}

	return &v1.ListRunnersRequest_Filter{
		CreatorIds: creatorIDs,
		Kinds:      []v1.RunnerKind{v1.RunnerKind_RUNNER_KIND_REMOTE},
		Providers:  providers,
	}, true
}

func runnerProvidersFromNames(names []string) ([]v1.RunnerProvider, error) {
	result := make([]v1.RunnerProvider, 0, len(names))
	for _, name := range names {
		provider, ok := providerFromString(name)
		if !ok {
			return nil, fmt.Errorf("unsupported runner provider %q; supported values are aws_ec2 and gcp", name)
		}
		result = append(result, provider)
	}
	return result, nil
}

func importableRunnerProviders() []v1.RunnerProvider {
	return []v1.RunnerProvider{
		v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2,
		v1.RunnerProvider_RUNNER_PROVIDER_GCP,
	}
}

func importableRunner(remoteRunner *v1.Runner) bool {
	if remoteRunner.GetKind() != v1.RunnerKind_RUNNER_KIND_REMOTE {
		return false
	}
	_, ok := providerFromString(providerToString(remoteRunner.GetProvider()))
	return ok
}
