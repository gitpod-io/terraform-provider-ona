// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package githubapp

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *Resource) githubDefinition(ctx context.Context) (*v1.IntegrationDefinition, error) {
	var definition *v1.IntegrationDefinition
	var token string
	seen := make(map[string]struct{})
	for {
		result, err := r.client.IntegrationService().ListIntegrationDefinitions(ctx, connect.NewRequest(&v1.ListIntegrationDefinitionsRequest{
			Pagination: &v1.PaginationRequest{PageSize: listutil.DefaultPageSize, Token: token},
		}))
		if err != nil {
			return nil, safeAPIError("listing integration definitions", err)
		}
		if result == nil || result.Msg == nil {
			return nil, fmt.Errorf("the Ona API returned an empty integration definition response")
		}
		for _, candidate := range result.Msg.GetDefinitions() {
			if candidate.GetHost() != "github.com" || candidate.GetAuth().GetProprietaryApp() == nil {
				continue
			}
			if candidate.GetId() == "" {
				return nil, fmt.Errorf("the GitHub App definition has no ID")
			}
			if definition != nil {
				return nil, fmt.Errorf("the catalog contains multiple GitHub App definitions; resolve the ambiguity before managing this integration")
			}
			definition = candidate
		}
		token = result.Msg.GetPagination().GetNextToken()
		if err := listutil.NextPageToken(seen, token); err != nil {
			return nil, fmt.Errorf("the Ona API repeated an integration definition pagination token")
		}
		if token == "" {
			break
		}
	}
	if definition == nil {
		return nil, fmt.Errorf("no github.com proprietary App definition is available in this Ona deployment")
	}
	return definition, nil
}

func isOwnedGitHubApp(integration *v1.Integration, definition *v1.IntegrationDefinition) bool {
	if definition.GetId() == "" || definition.GetHost() != "github.com" || integration.GetIntegrationDefinitionId() != definition.GetId() {
		return false
	}
	app := integration.GetAuth().GetProprietaryApp()
	sharedApp := definition.GetAuth().GetProprietaryApp()
	if app == nil || sharedApp == nil {
		return false
	}
	// Match the dashboard's custom-App heuristic; this is not an authorization check.
	return app.GetAppId() != sharedApp.GetAppId() || app.GetAppSlug() != sharedApp.GetAppSlug() || app.GetClientId() != sharedApp.GetClientId()
}

func populateModel(data *Model, integration *v1.Integration, definition *v1.IntegrationDefinition, deployment string) diag.Diagnostics {
	var diags diag.Diagnostics
	if !isOwnedGitHubApp(integration, definition) {
		diags.AddError("Not an Organization-Owned GitHub App", "This resource requires a GitHub App associated with Ona's canonical GitHub definition and saved App ID, slug, or client ID differing from that definition. Matching metadata is classified as the shared App, even when credentials were configured locally. Check the integration and its saved App metadata; existing state is retained when classification fails.")
		return diags
	}
	if _, err := uuid.Parse(integration.GetId()); err != nil {
		diags.AddError("Invalid GitHub App Integration Metadata", "The Ona API returned an integration without a valid UUID.")
		return diags
	}
	app := integration.GetAuth().GetProprietaryApp()
	if !validAppID(app.GetAppId()) || app.GetAppSlug() == "" || app.GetClientId() == "" {
		diags.AddError("Incomplete GitHub App Metadata", "Ona did not return the saved App ID, slug, and client ID. Retry after metadata is available; credentials are not required for refresh.")
		return diags
	}
	callback := definition.GetAuth().GetOauth().GetRedirectUrl()
	canonical, err := stableURL(callback)
	if err != nil {
		diags.AddError("Invalid GitHub App Callback Metadata", "The canonical GitHub definition must include an absolute OAuth redirect URL without browser session parameters.")
		return diags
	}
	base, err := stableURL(deployment)
	if err != nil {
		diags.AddError("Invalid Ona Deployment URL", "Configure an absolute Ona API base URL to construct the integration dashboard URL.")
		return diags
	}
	setup, setupDiags := types.ObjectValue(setupTypes, map[string]attr.Value{
		"callback_url":     types.StringValue(callback),
		"webhook_url":      types.StringValue(canonical.ResolveReference(&url.URL{Path: "/integrations/app/github/webhook"}).String()),
		"installation_url": types.StringValue(base.ResolveReference(&url.URL{Path: "/settings/org-integrations"}).String()),
	})
	diags.Append(setupDiags...)
	if diags.HasError() {
		return diags
	}
	installation := types.ObjectNull(installationTypes)
	if remote := integration.GetExternalInstallation(); remote != nil {
		if remote.GetId() == "" {
			diags.AddError("Incomplete GitHub Installation Metadata", "Ona returned installation metadata without an installation ID.")
			return diags
		}
		var installationDiags diag.Diagnostics
		installation, installationDiags = types.ObjectValue(installationTypes, map[string]attr.Value{
			"id": types.StringValue(remote.GetId()), "account_name": types.StringValue(remote.GetAccountName()), "account_type": types.StringValue(remote.GetAccountType()),
		})
		diags.Append(installationDiags...)
		if diags.HasError() {
			return diags
		}
	}
	data.ID = types.StringValue(integration.GetId())
	data.AppID = types.StringValue(app.GetAppId())
	data.Enabled = types.BoolValue(integration.GetEnabled())
	data.AppSlug = types.StringValue(app.GetAppSlug())
	data.ClientID = types.StringValue(app.GetClientId())
	data.Setup = setup
	data.Installation = installation
	data.Credentials = types.ObjectNull(credentialsTypes)
	return diags
}

func stableURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("expected an absolute HTTP(S) URL without user information, query, or fragment")
	}
	return parsed, nil
}

// safeAPIError excludes response bodies because credential-validation errors can contain secrets.
func safeAPIError(operation string, err error) error {
	return errors.New(providerdiag.APIErrorDetail(operation, err, providerdiag.OmitRawError))
}
