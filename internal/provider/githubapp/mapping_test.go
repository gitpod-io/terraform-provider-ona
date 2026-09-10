// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package githubapp

import (
	"strings"
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const testID = "01980ed3-a090-7b5b-a74c-9bf5d8cfe501"

func testDefinition() *v1.IntegrationDefinition {
	return &v1.IntegrationDefinition{Id: "github-definition", Host: "github.com", Auth: &v1.IntegrationAuthentication{
		ProprietaryApp: &v1.IntegrationProprietaryAppConfig{AppId: "456", AppSlug: "shared-app", ClientId: "shared-client-id"}, Oauth: &v1.IntegrationOAuthConfig{RedirectUrl: "https://app.gitpod.io/auth/github/callback"},
	}}
}

func testIntegration() *v1.Integration {
	return &v1.Integration{Id: testID, IntegrationDefinitionId: "github-definition", Enabled: true, Auth: &v1.IntegrationAuthentication{
		ProprietaryApp: &v1.IntegrationProprietaryAppConfig{AppId: "123", AppSlug: "owned-app", ClientId: "client-id", PrivateKey: "private-must-not-appear", ClientSecret: "client-must-not-appear", WebhookSecret: "webhook-must-not-appear"},
	}}
}

func testSharedIntegration() *v1.Integration {
	remote := testIntegration()
	remote.Auth.ProprietaryApp = testDefinition().Auth.ProprietaryApp
	return remote
}

func TestIsOwnedGitHubApp(t *testing.T) {
	t.Parallel()
	type Expectation struct{ Owned bool }
	tests := []struct {
		Name       string
		Remote     *v1.Integration
		Definition *v1.IntegrationDefinition
		Mutate     func(*v1.Integration, *v1.IntegrationDefinition)
		Expected   Expectation
	}{
		{Name: "all_metadata_equal", Remote: testSharedIntegration(), Definition: testDefinition()},
		{Name: "different_credentials_only", Remote: testSharedIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) {
			i.Auth.ProprietaryApp.PrivateKey = "private-must-not-appear"
		}},
		{Name: "app_id_only", Remote: testSharedIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.Auth.ProprietaryApp.AppId = "123" }, Expected: Expectation{Owned: true}},
		{Name: "app_slug_only", Remote: testSharedIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.Auth.ProprietaryApp.AppSlug = "owned-app" }, Expected: Expectation{Owned: true}},
		{Name: "client_id_only", Remote: testSharedIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.Auth.ProprietaryApp.ClientId = "client-id" }, Expected: Expectation{Owned: true}},
		{Name: "all_metadata_different_pending", Remote: testIntegration(), Definition: testDefinition(), Expected: Expectation{Owned: true}},
		{Name: "disabled_custom_app", Remote: testIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.Enabled = false }, Expected: Expectation{Owned: true}},
		{Name: "installed_shared_app", Remote: testSharedIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) {
			i.ExternalInstallation = &v1.IntegrationExternalInstallation{Id: "42"}
		}},
		{Name: "case_is_not_normalized", Remote: testSharedIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.Auth.ProprietaryApp.AppSlug = "SHARED-APP" }, Expected: Expectation{Owned: true}},
		{Name: "whitespace_is_not_normalized", Remote: testSharedIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.Auth.ProprietaryApp.ClientId += " " }, Expected: Expectation{Owned: true}},
		{Name: "nil_integration", Definition: testDefinition()},
		{Name: "nil_definition", Remote: testIntegration()},
		{Name: "mismatched_definition", Remote: testIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.IntegrationDefinitionId = "other" }},
		{Name: "caller_host_without_definition", Remote: testIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) {
			i.IntegrationDefinitionId = ""
			i.Host = "github.com"
		}},
		{Name: "empty_definition_id", Remote: testIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, d *v1.IntegrationDefinition) { i.IntegrationDefinitionId = ""; d.Id = "" }},
		{Name: "noncanonical_host", Remote: testIntegration(), Definition: testDefinition(), Mutate: func(_ *v1.Integration, d *v1.IntegrationDefinition) { d.Host = "GitHub.com" }},
		{Name: "nil_integration_auth", Remote: testIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.Auth = nil }},
		{Name: "nil_integration_app", Remote: testIntegration(), Definition: testDefinition(), Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.Auth.ProprietaryApp = nil }},
		{Name: "nil_definition_auth", Remote: testIntegration(), Definition: testDefinition(), Mutate: func(_ *v1.Integration, d *v1.IntegrationDefinition) { d.Auth = nil }},
		{Name: "nil_definition_app", Remote: testIntegration(), Definition: testDefinition(), Mutate: func(_ *v1.Integration, d *v1.IntegrationDefinition) { d.Auth.ProprietaryApp = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			if tc.Mutate != nil {
				tc.Mutate(tc.Remote, tc.Definition)
			}
			got := Expectation{Owned: isOwnedGitHubApp(tc.Remote, tc.Definition)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("classification mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func errorSummaries(diags diag.Diagnostics) []string {
	var errors []string
	for _, diagnostic := range diags.Errors() {
		errors = append(errors, diagnostic.Summary())
	}
	return errors
}

func TestPopulateModel(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		ID, AppID, Slug, ClientID, Callback, Webhook, Dashboard, Installation string
		Version                                                               int64
		SecretPresent                                                         bool
		Errors                                                                []string
	}
	valid := Expectation{ID: testID, AppID: "123", Slug: "owned-app", ClientID: "client-id", Callback: "https://app.gitpod.io/auth/github/callback", Webhook: "https://app.gitpod.io/integrations/app/github/webhook", Dashboard: "https://ona.example.com/settings/org-integrations", Version: 5}
	tests := []struct {
		Name       string
		Deployment string
		Mutate     func(*v1.Integration, *v1.IntegrationDefinition)
		Expected   Expectation
	}{
		{Name: "pending_owned_app_with_censored_output", Expected: valid},
		{Name: "default_deployment", Deployment: "https://app.gitpod.io/api", Expected: func() Expectation {
			e := valid
			e.Dashboard = "https://app.gitpod.io/settings/org-integrations"
			return e
		}()},
		{Name: "invalid_deployment", Deployment: "not-a-url", Expected: Expectation{Errors: []string{"Invalid Ona Deployment URL"}}},
		{Name: "installed", Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) {
			i.ExternalInstallation = &v1.IntegrationExternalInstallation{Id: "42", AccountName: "my-org", AccountType: "Organization"}
		}, Expected: func() Expectation { e := valid; e.Installation = "42"; return e }()},
		{Name: "custom_domain_canonical_callback", Mutate: func(_ *v1.Integration, d *v1.IntegrationDefinition) {
			d.Auth.Oauth.RedirectUrl = "https://canonical.example.com/auth/github/callback"
		}, Expected: func() Expectation {
			e := valid
			e.Callback = "https://canonical.example.com/auth/github/callback"
			e.Webhook = "https://canonical.example.com/integrations/app/github/webhook"
			return e
		}()},
		{Name: "shared_app", Mutate: func(i *v1.Integration, d *v1.IntegrationDefinition) { i.Auth.ProprietaryApp = d.Auth.ProprietaryApp }, Expected: Expectation{Errors: []string{"Not an Organization-Owned GitHub App"}}},
		{Name: "misleading_custom_host_and_auth", Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) {
			i.IntegrationDefinitionId = ""
			i.Host = "github.com"
		}, Expected: Expectation{Errors: []string{"Not an Organization-Owned GitHub App"}}},
		{Name: "other_definition", Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) {
			i.IntegrationDefinitionId = "other"
			i.Host = "github.com"
		}, Expected: Expectation{Errors: []string{"Not an Organization-Owned GitHub App"}}},
		{Name: "non_github_catalog", Mutate: func(_ *v1.Integration, d *v1.IntegrationDefinition) { d.Host = "github.com.example.com" }, Expected: Expectation{Errors: []string{"Not an Organization-Owned GitHub App"}}},
		{Name: "catalog_without_app", Mutate: func(_ *v1.Integration, d *v1.IntegrationDefinition) { d.Auth.ProprietaryApp = nil }, Expected: Expectation{Errors: []string{"Not an Organization-Owned GitHub App"}}},
		{Name: "invalid_id", Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.Id = "invalid" }, Expected: Expectation{Errors: []string{"Invalid GitHub App Integration Metadata"}}},
		{Name: "missing_app_auth", Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.Auth = nil }, Expected: Expectation{Errors: []string{"Not an Organization-Owned GitHub App"}}},
		{Name: "missing_app_metadata", Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) { i.Auth.ProprietaryApp.AppSlug = "" }, Expected: Expectation{Errors: []string{"Incomplete GitHub App Metadata"}}},
		{Name: "session_callback_rejected", Mutate: func(_ *v1.Integration, d *v1.IntegrationDefinition) { d.Auth.Oauth.RedirectUrl += "?state=secret" }, Expected: Expectation{Errors: []string{"Invalid GitHub App Callback Metadata"}}},
		{Name: "missing_callback", Mutate: func(_ *v1.Integration, d *v1.IntegrationDefinition) { d.Auth.Oauth = nil }, Expected: Expectation{Errors: []string{"Invalid GitHub App Callback Metadata"}}},
		{Name: "malformed_installation", Mutate: func(i *v1.Integration, _ *v1.IntegrationDefinition) {
			i.ExternalInstallation = &v1.IntegrationExternalInstallation{}
		}, Expected: Expectation{Errors: []string{"Incomplete GitHub Installation Metadata"}}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			remote, definition := testIntegration(), testDefinition()
			if tc.Mutate != nil {
				tc.Mutate(remote, definition)
			}
			model := emptyModel()
			model.CredentialsVersion = types.Int64Value(5)
			deployment := tc.Deployment
			if deployment == "" {
				deployment = "https://ona.example.com/api"
			}
			diags := populateModel(&model, remote, definition, deployment)
			got := Expectation{Errors: errorSummaries(diags)}
			if !diags.HasError() {
				got.ID = model.ID.ValueString()
				got.AppID = model.AppID.ValueString()
				got.Slug = model.AppSlug.ValueString()
				got.ClientID = model.ClientID.ValueString()
				got.Version = model.CredentialsVersion.ValueInt64()
				got.Callback = testStringAttribute(t, model.Setup, "callback_url")
				got.Webhook = testStringAttribute(t, model.Setup, "webhook_url")
				got.Dashboard = testStringAttribute(t, model.Setup, "installation_url")
				if !model.Installation.IsNull() {
					got.Installation = testStringAttribute(t, model.Installation, "id")
				}
				got.SecretPresent = !model.Credentials.IsNull() || strings.Contains(model.Credentials.String()+model.Setup.String(), "must-not-appear")
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("populateModel mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func testStringAttribute(t *testing.T, object types.Object, name string) string {
	t.Helper()
	attribute := object.Attributes()[name]
	value, ok := attribute.(types.String)
	if !ok {
		t.Fatalf("attribute %q: expected types.String, got %T", name, attribute)
	}
	return value.ValueString()
}
