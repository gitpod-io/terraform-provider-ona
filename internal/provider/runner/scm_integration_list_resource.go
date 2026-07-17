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

var _ list.ListResource = &SCMIntegrationResource{}

func NewSCMIntegrationListResource() list.ListResource {
	return &SCMIntegrationResource{}
}

type scmIntegrationListModel struct {
	AuthModes    types.List `tfsdk:"auth_modes"`
	Hosts        types.List `tfsdk:"hosts"`
	SCMProviders types.List `tfsdk:"scm_providers"`
	RunnerIDs    types.List `tfsdk:"runner_ids"`
}

type scmIntegrationListFilter struct {
	AuthModes []string
	Hosts     []string
	Providers []string
	RunnerIDs []string
}

func (r *SCMIntegrationResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists Ona runner SCM integrations without retrieving OAuth or PAT secret values.",
		Attributes: map[string]listschema.Attribute{
			"auth_modes": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Authentication modes to include. Supported values are `oauth` and `pat`.",
			},
			"hosts": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "SCM host names to include.",
			},
			"scm_providers": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "SCM provider IDs to include. Known values are `github`, `gitlab`, `bitbucket`, `azuredevops_entra`, and `azuredevops_server`; the API can return additional runner-configured IDs.",
			},
			"runner_ids": listschema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Runner IDs to include.",
			},
		},
	}
}

func (r *SCMIntegrationResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_scm_integration resources")))
			return
		}

		filter, ok := newSCMIntegrationListFilter(ctx, req, push)
		if !ok {
			return
		}

		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.RunnerConfigurationService().ListSCMIntegrations(ctx, connect.NewRequest(&v1.ListSCMIntegrationsRequest{
				Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token},
				Filter:     &v1.ListSCMIntegrationsRequest_Filter{RunnerIds: filter.RunnerIDs},
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona SCM Integrations", fmt.Errorf("list SCM integrations: %w", err)))
				return
			}

			integrations := result.Msg.GetIntegrations()
			sort.SliceStable(integrations, func(i, j int) bool {
				return integrations[i].GetId() < integrations[j].GetId()
			})
			for _, integration := range integrations {
				if !filter.matches(integration) {
					continue
				}
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = scmIntegrationDisplayName(integration)
				item.Diagnostics.Append(item.Identity.Set(ctx, SCMIntegrationIdentityModel{ID: types.StringValue(integration.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model SCMIntegrationModel
					populateSCMIntegrationModel(&model, integration)
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

func newSCMIntegrationListFilter(ctx context.Context, req list.ListRequest, push func(list.ListResult) bool) (scmIntegrationListFilter, bool) {
	var config scmIntegrationListModel
	diags := req.Config.Get(ctx, &config)
	if !listutil.PushDiagnostics(push, diags) {
		return scmIntegrationListFilter{}, false
	}

	authModes, diags := listutil.StringList(ctx, config.AuthModes)
	if !listutil.PushDiagnostics(push, diags) {
		return scmIntegrationListFilter{}, false
	}
	if err := validateSCMIntegrationAuthModes(authModes); err != nil {
		push(listutil.Error("Invalid SCM Authentication Mode", err))
		return scmIntegrationListFilter{}, false
	}
	hosts, diags := listutil.StringList(ctx, config.Hosts)
	if !listutil.PushDiagnostics(push, diags) {
		return scmIntegrationListFilter{}, false
	}
	providers, diags := listutil.StringList(ctx, config.SCMProviders)
	if !listutil.PushDiagnostics(push, diags) {
		return scmIntegrationListFilter{}, false
	}
	runnerIDs, diags := listutil.StringList(ctx, config.RunnerIDs)
	if !listutil.PushDiagnostics(push, diags) {
		return scmIntegrationListFilter{}, false
	}

	return scmIntegrationListFilter{
		AuthModes: authModes,
		Hosts:     hosts,
		Providers: providers,
		RunnerIDs: runnerIDs,
	}, true
}

func validateSCMIntegrationAuthModes(authModes []string) error {
	for _, authMode := range authModes {
		switch authMode {
		case scmAuthModeOAuth, scmAuthModePAT:
		default:
			return fmt.Errorf("unsupported SCM authentication mode %q; supported values are oauth and pat", authMode)
		}
	}
	return nil
}

func (f scmIntegrationListFilter) matches(integration *v1.SCMIntegration) bool {
	return matchesSCMIntegrationFilter(f.AuthModes, scmIntegrationAuthMode(integration)) &&
		matchesSCMIntegrationFilter(f.Hosts, integration.GetHost()) &&
		matchesSCMIntegrationFilter(f.Providers, integration.GetScmId()) &&
		matchesSCMIntegrationFilter(f.RunnerIDs, integration.GetRunnerId())
}

func matchesSCMIntegrationFilter(filter []string, value string) bool {
	if len(filter) == 0 {
		return true
	}
	for _, candidate := range filter {
		if candidate == value {
			return true
		}
	}
	return false
}

func scmIntegrationAuthMode(integration *v1.SCMIntegration) string {
	if integration.GetPat() {
		return scmAuthModePAT
	}
	if integration.GetOauth() != nil {
		return scmAuthModeOAuth
	}
	return ""
}

func scmIntegrationDisplayName(integration *v1.SCMIntegration) string {
	if integration.GetHost() == "" {
		return integration.GetId()
	}
	if integration.GetScmId() == "" {
		return integration.GetHost()
	}
	return fmt.Sprintf("%s (%s)", integration.GetHost(), integration.GetScmId())
}
