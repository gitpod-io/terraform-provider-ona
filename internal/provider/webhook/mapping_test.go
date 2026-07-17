// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package webhook

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestWebhookEnumMappings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		webhookType      string
		provider         string
		expectedType     v1.WebhookType
		expectedProvider v1.WebhookProvider
	}{
		{
			name:             "github_repository",
			webhookType:      webhookTypeRepository,
			provider:         webhookProviderGitHub,
			expectedType:     v1.WebhookType_WEBHOOK_TYPE_SCM_REPOSITORY,
			expectedProvider: v1.WebhookProvider_WEBHOOK_PROVIDER_GITHUB,
		},
		{
			name:             "gitlab_organization",
			webhookType:      webhookTypeOrganization,
			provider:         webhookProviderGitLab,
			expectedType:     v1.WebhookType_WEBHOOK_TYPE_SCM_ORGANIZATION,
			expectedProvider: v1.WebhookProvider_WEBHOOK_PROVIDER_GITLAB,
		},
		{
			name:             "bitbucket_repository",
			webhookType:      webhookTypeRepository,
			provider:         webhookProviderBitbucket,
			expectedType:     v1.WebhookType_WEBHOOK_TYPE_SCM_REPOSITORY,
			expectedProvider: v1.WebhookProvider_WEBHOOK_PROVIDER_BITBUCKET,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotType, typeOK := webhookTypeFromString(tc.webhookType)
			gotProvider, providerOK := webhookProviderFromString(tc.provider)
			if !typeOK || !providerOK {
				t.Fatalf("mapping rejected supported values: type=%t provider=%t", typeOK, providerOK)
			}
			if gotType != tc.expectedType {
				t.Errorf("webhook type mismatch: got %v want %v", gotType, tc.expectedType)
			}
			if gotProvider != tc.expectedProvider {
				t.Errorf("webhook provider mismatch: got %v want %v", gotProvider, tc.expectedProvider)
			}
		})
	}
}

func TestCreateWebhookRequest(t *testing.T) {
	t.Parallel()

	repositoryScopes, diags := types.SetValueFrom(t.Context(), types.ObjectType{AttrTypes: repositoryScopeAttributeTypes}, []RepositoryScopeModel{
		{Host: types.StringValue("github.com"), Owner: types.StringValue("ona"), Name: types.StringValue("zeta")},
		{Host: types.StringValue("github.com"), Owner: types.StringValue("ona"), Name: types.StringValue("alpha")},
	})
	if diags.HasError() {
		t.Fatalf("create repository scope set: %v", diags)
	}

	request, requestDiags := createWebhookRequest(t.Context(), Model{
		Name:              types.StringValue("Deployments"),
		Description:       types.StringValue("Deployment events"),
		Type:              types.StringValue(webhookTypeRepository),
		Provider:          types.StringValue(webhookProviderGitHub),
		RepositoryScopes:  repositoryScopes,
		OrganizationScope: types.ObjectNull(organizationScopeAttributeTypes),
	})
	if requestDiags.HasError() {
		t.Fatalf("createWebhookRequest() diagnostics: %v", requestDiags)
	}

	expected := &v1.CreateWebhookRequest{
		Name:        "Deployments",
		Description: "Deployment events",
		Type:        v1.WebhookType_WEBHOOK_TYPE_SCM_REPOSITORY,
		Provider:    v1.WebhookProvider_WEBHOOK_PROVIDER_GITHUB,
		Scopes: []*v1.WebhookRepositoryScope{
			{Host: "github.com", Owner: "ona", Name: "alpha"},
			{Host: "github.com", Owner: "ona", Name: "zeta"},
		},
	}
	if diff := cmp.Diff(expected, request, protocmp.Transform()); diff != "" {
		t.Errorf("createWebhookRequest() mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateWebhookModelScopeCombinations(t *testing.T) {
	t.Parallel()

	repositoryScopes, setDiags := types.SetValueFrom(t.Context(), types.ObjectType{AttrTypes: repositoryScopeAttributeTypes}, []RepositoryScopeModel{{
		Host:  types.StringValue("github.com"),
		Owner: types.StringValue("ona"),
		Name:  types.StringValue("terraform-provider-ona"),
	}})
	if setDiags.HasError() {
		t.Fatalf("create repository scope set: %v", setDiags)
	}
	organizationScope, objectDiags := types.ObjectValueFrom(t.Context(), organizationScopeAttributeTypes, OrganizationScopeModel{
		Host: types.StringValue("github.com"),
		Name: types.StringValue("ona"),
	})
	if objectDiags.HasError() {
		t.Fatalf("create organization scope object: %v", objectDiags)
	}

	tests := []struct {
		name         string
		webhookType  string
		repositories types.Set
		organization types.Object
		wantError    bool
	}{
		{
			name:         "repository_scope",
			webhookType:  webhookTypeRepository,
			repositories: repositoryScopes,
			organization: types.ObjectNull(organizationScopeAttributeTypes),
		},
		{
			name:         "organization_scope",
			webhookType:  webhookTypeOrganization,
			repositories: types.SetNull(types.ObjectType{AttrTypes: repositoryScopeAttributeTypes}),
			organization: organizationScope,
		},
		{
			name:         "repository_missing_scope",
			webhookType:  webhookTypeRepository,
			repositories: types.SetNull(types.ObjectType{AttrTypes: repositoryScopeAttributeTypes}),
			organization: types.ObjectNull(organizationScopeAttributeTypes),
			wantError:    true,
		},
		{
			name:         "organization_with_repository_scope",
			webhookType:  webhookTypeOrganization,
			repositories: repositoryScopes,
			organization: organizationScope,
			wantError:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var diags diag.Diagnostics
			validateModel(t.Context(), Model{
				Name:              types.StringValue("Webhook"),
				Type:              types.StringValue(tc.webhookType),
				Provider:          types.StringValue(webhookProviderGitHub),
				RepositoryScopes:  tc.repositories,
				OrganizationScope: tc.organization,
			}, true, &diags)
			if diags.HasError() != tc.wantError {
				t.Errorf("validateModel() HasError=%t want %t: %v", diags.HasError(), tc.wantError, diags)
			}
		})
	}
}
