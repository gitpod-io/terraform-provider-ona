// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package gitauthentication

import (
	"context"
	"fmt"
	"sort"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &Resource{}

func NewListResource() list.ListResource { return &Resource{} }

type listModel struct {
	ServiceAccountID types.String `tfsdk:"service_account_id"`
	SCMIntegrationID types.String `tfsdk:"scm_integration_id"`
}

func (r *Resource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists importable PAT-backed service-account Git authentications without retrieving PAT values.",
		Attributes: map[string]listschema.Attribute{
			"service_account_id": listschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Service account ID to include.",
			},
			"scm_integration_id": listschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "PAT-enabled SCM integration ID to include.",
			},
		},
	}
}

func (r *Resource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_git_authentication resources")))
			return
		}

		var config listModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &config)) {
			return
		}
		serviceAccountID := optionalString(config.ServiceAccountID)
		integrationID := optionalString(config.SCMIntegrationID)
		var runnerID string
		var configuredIntegration *v1.SCMIntegration
		cache := map[string]*v1.SCMIntegration{}
		if integrationID != "" {
			integration, err := r.getSCMIntegration(ctx, integrationID)
			if err != nil {
				push(listutil.Error("Unable to Read Ona SCM Integration", err))
				return
			}
			diags := validateSCMIntegration(integration, integrationID)
			if !listutil.PushDiagnostics(push, diags) {
				return
			}
			configuredIntegration = integration
			runnerID = integration.GetRunnerId()
		}

		var paginationToken string
		seenTokens := make(map[string]struct{})
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			filter := &v1.ListHostAuthenticationTokensRequest_Filter{}
			if runnerID != "" {
				filter.RunnerId = stringPointer(runnerID)
			}
			if serviceAccountID != "" {
				filter.SubjectId = stringPointer(serviceAccountID)
			}
			result, err := r.client.RunnerConfigurationService().ListHostAuthenticationTokens(ctx, connect.NewRequest(&v1.ListHostAuthenticationTokensRequest{
				Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: paginationToken},
				Filter:     filter,
			}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Git Authentications", fmt.Errorf("list host authentication tokens: %w", err)))
				return
			}
			nextToken := result.Msg.GetPagination().GetNextToken()
			if err := listutil.NextPageToken(seenTokens, nextToken); err != nil {
				push(listutil.Error("Unable to List Ona Git Authentications", err))
				return
			}

			tokens := result.Msg.GetTokens()
			sort.SliceStable(tokens, func(i, j int) bool { return tokens[i].GetId() < tokens[j].GetId() })
			for _, token := range tokens {
				if !basicRepresentableToken(token, serviceAccountID) {
					continue
				}
				integration := configuredIntegration
				if integration == nil {
					key := scmIntegrationTargetKey(token.GetRunnerId(), token.GetHost())
					var ok bool
					integration, ok = cache[key]
					if !ok {
						integration, err = r.resolveSCMIntegration(ctx, token.GetRunnerId(), token.GetHost())
						if err != nil {
							push(listutil.Error("Unable to Resolve Ona SCM Integration", err))
							return
						}
						cache[key] = integration
					}
				}
				if integration == nil {
					continue
				}
				if !representableToken(token, integration) {
					continue
				}
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = gitAuthenticationDisplayName(token)
				item.Diagnostics.Append(item.Identity.Set(ctx, IdentityModel{ID: types.StringValue(token.GetId())})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					model := modelFromRemote(ctx, token, integration.GetId(), types.StringNull(), &item.Diagnostics)
					if !item.Diagnostics.HasError() {
						item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
					}
				}
				if !push(item) || item.Diagnostics.HasError() {
					return
				}
				emitted++
			}

			paginationToken = nextToken
			if paginationToken == "" {
				return
			}
		}
	}
}

func basicRepresentableToken(token *v1.HostAuthenticationToken, serviceAccountID string) bool {
	if token == nil || token.GetId() == "" || token.GetRunnerId() == "" || token.GetHost() == "" || token.GetSource() != v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT {
		return false
	}
	subject := token.GetSubject()
	if subject == nil || subject.GetId() == "" || subject.GetPrincipal() != v1.Principal_PRINCIPAL_SERVICE_ACCOUNT {
		return false
	}
	return serviceAccountID == "" || subject.GetId() == serviceAccountID
}

func representableToken(token *v1.HostAuthenticationToken, integration *v1.SCMIntegration) bool {
	return integration != nil && integration.GetPat() && integration.GetId() != "" && integration.GetRunnerId() != "" && integration.GetHost() != "" && integration.GetRunnerId() == token.GetRunnerId() && integration.GetHost() == token.GetHost()
}

func scmIntegrationTargetKey(runnerID string, host string) string { return runnerID + "\x00" + host }

func gitAuthenticationDisplayName(token *v1.HostAuthenticationToken) string {
	if token.GetHost() == "" {
		return token.GetId()
	}
	if token.GetSubject().GetId() == "" {
		return token.GetHost()
	}
	return fmt.Sprintf("%s (%s)", token.GetHost(), token.GetSubject().GetId())
}

func optionalString(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}
