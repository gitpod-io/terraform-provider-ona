// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

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

var _ list.ListResource = &EnvironmentClassResource{}

func NewEnvironmentClassListResource() list.ListResource {
	return &EnvironmentClassResource{}
}

type environmentClassListModel struct {
	RunnerIDs types.List `tfsdk:"runner_ids"`
	Providers types.List `tfsdk:"providers"`
	Enabled   types.Bool `tfsdk:"enabled"`
}

type environmentClassListFilter struct {
	RunnerIDs []string
	Providers []v1.RunnerProvider
	Enabled   *bool
}

func (r *EnvironmentClassResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists Ona runner environment classes.",
		Attributes: map[string]listschema.Attribute{
			"enabled": listschema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether to include only enabled (`true`) or disabled (`false`) environment classes.",
			},
			"runner_ids": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Runner IDs to include.",
			},
			"providers": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Runner providers to include. Supported values are `aws_ec2` and `gcp`.",
			},
		},
	}
}

func (r *EnvironmentClassResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_environment_class resources")))
			return
		}

		filter, ok := newEnvironmentClassListFilter(ctx, req, push)
		if !ok {
			return
		}

		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.RunnerConfigurationService().ListEnvironmentClasses(ctx, connect.NewRequest(&v1.ListEnvironmentClassesRequest{
				Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token},
				Filter: &v1.ListEnvironmentClassesRequest_Filter{
					RunnerIds:       filter.RunnerIDs,
					Enabled:         filter.Enabled,
					RunnerProviders: filter.Providers,
				},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Environment Classes", fmt.Errorf("list environment classes: %w", err)))
				return
			}

			classes := result.Msg.GetEnvironmentClasses()
			sort.SliceStable(classes, func(i, j int) bool { return classes[i].GetId() < classes[j].GetId() })
			for _, class := range classes {
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = class.GetDisplayName()
				if item.DisplayName == "" {
					item.DisplayName = class.GetId()
				}
				item.Diagnostics.Append(item.Identity.Set(ctx, EnvironmentClassIdentityModel{ID: types.StringValue(class.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model EnvironmentClassModel
					item.Diagnostics.Append(populateEnvironmentClassModel(ctx, &model, class)...)
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

func newEnvironmentClassListFilter(ctx context.Context, req list.ListRequest, push func(list.ListResult) bool) (environmentClassListFilter, bool) {
	var config environmentClassListModel
	diags := req.Config.Get(ctx, &config)
	if !listutil.PushDiagnostics(push, diags) {
		return environmentClassListFilter{}, false
	}

	runnerIDs, diags := listutil.StringList(ctx, config.RunnerIDs)
	if !listutil.PushDiagnostics(push, diags) {
		return environmentClassListFilter{}, false
	}

	providerNames, diags := listutil.StringList(ctx, config.Providers)
	if !listutil.PushDiagnostics(push, diags) {
		return environmentClassListFilter{}, false
	}
	providers, err := runnerProvidersFromNames(providerNames)
	if err != nil {
		push(listutil.Error("Invalid Environment Class Provider", err))
		return environmentClassListFilter{}, false
	}
	if len(providers) == 0 {
		providers = importableRunnerProviders()
	}

	var enabled *bool
	if !config.Enabled.IsNull() && !config.Enabled.IsUnknown() {
		enabled = ptr(config.Enabled.ValueBool())
	}

	return environmentClassListFilter{
		RunnerIDs: runnerIDs,
		Providers: providers,
		Enabled:   enabled,
	}, true
}
