// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
)

type registrationExpectation struct {
	Count int
}

type registrationTestCase struct {
	Name     string
	TypeName string
	Expected registrationExpectation
}

func TestDataSourcesAreRegistered(t *testing.T) {
	t.Parallel()

	tests := []registrationTestCase{
		{Name: "integration_definitions", TypeName: "ona_integration_definitions", Expected: registrationExpectation{Count: 1}},
		{Name: "project", TypeName: "ona_project", Expected: registrationExpectation{Count: 1}},
		{Name: "runners", TypeName: "ona_runners", Expected: registrationExpectation{Count: 1}},
		{Name: "runner", TypeName: "ona_runner", Expected: registrationExpectation{Count: 1}},
		{Name: "security_policies", TypeName: "ona_security_policies", Expected: registrationExpectation{Count: 1}},
		{Name: "skill", TypeName: "ona_skill", Expected: registrationExpectation{Count: 1}},
		{Name: "users", TypeName: "ona_users", Expected: registrationExpectation{Count: 1}},
		{Name: "user", TypeName: "ona_user", Expected: registrationExpectation{Count: 1}},
		{Name: "warm_pools", TypeName: "ona_warm_pools", Expected: registrationExpectation{Count: 1}},
		{Name: "warm_pool", TypeName: "ona_warm_pool", Expected: registrationExpectation{Count: 1}},
		{Name: "automations", TypeName: "ona_automations", Expected: registrationExpectation{Count: 1}},
	}

	provider := &OnaProvider{}
	registrations := make(map[string]int)
	for _, newDataSource := range provider.DataSources(t.Context()) {
		var resp datasource.MetadataResponse
		newDataSource().Metadata(t.Context(), datasource.MetadataRequest{ProviderTypeName: "ona"}, &resp)
		registrations[resp.TypeName]++
	}

	testRegistrations(t, tests, registrations)
}

func TestResourcesAreRegistered(t *testing.T) {
	t.Parallel()

	tests := []registrationTestCase{
		{Name: "group_membership", TypeName: "ona_group_membership", Expected: registrationExpectation{Count: 1}},
		{Name: "group", TypeName: "ona_group", Expected: registrationExpectation{Count: 1}},
		{Name: "organization_role_assignment", TypeName: "ona_organization_role_assignment", Expected: registrationExpectation{Count: 1}},
		{Name: "team", TypeName: "ona_team", Expected: registrationExpectation{Count: 1}},
		{Name: "organization_ai_budget", TypeName: "ona_organization_ai_budget", Expected: registrationExpectation{Count: 1}},
		{Name: "team_ai_budget", TypeName: "ona_team_ai_budget", Expected: registrationExpectation{Count: 1}},
		{Name: "user_ai_budget", TypeName: "ona_user_ai_budget", Expected: registrationExpectation{Count: 1}},
		{Name: "integration", TypeName: "ona_integration", Expected: registrationExpectation{Count: 1}},
		{Name: "announcement_banner", TypeName: "ona_announcement_banner", Expected: registrationExpectation{Count: 1}},
		{Name: "custom_domain", TypeName: "ona_custom_domain", Expected: registrationExpectation{Count: 1}},
		{Name: "oidc_config", TypeName: "ona_oidc_config", Expected: registrationExpectation{Count: 1}},
		{Name: "organization_policies", TypeName: "ona_organization_policies", Expected: registrationExpectation{Count: 1}},
		{Name: "scim_configuration", TypeName: "ona_scim_configuration", Expected: registrationExpectation{Count: 1}},
		{Name: "sso_configuration", TypeName: "ona_sso_configuration", Expected: registrationExpectation{Count: 1}},
		{Name: "terms_of_service", TypeName: "ona_terms_of_service", Expected: registrationExpectation{Count: 1}},
		{Name: "project_insights", TypeName: "ona_project_insights", Expected: registrationExpectation{Count: 1}},
		{Name: "project", TypeName: "ona_project", Expected: registrationExpectation{Count: 1}},
		{Name: "environment_class", TypeName: "ona_environment_class", Expected: registrationExpectation{Count: 1}},
		{Name: "runner_llm_integration", TypeName: "ona_runner_llm_integration", Expected: registrationExpectation{Count: 1}},
		{Name: "runner_policy", TypeName: "ona_runner_policy", Expected: registrationExpectation{Count: 1}},
		{Name: "runner", TypeName: "ona_runner", Expected: registrationExpectation{Count: 1}},
		{Name: "scm_integration", TypeName: "ona_scm_integration", Expected: registrationExpectation{Count: 1}},
		{Name: "secret", TypeName: "ona_secret", Expected: registrationExpectation{Count: 1}},
		{Name: "security_policy", TypeName: "ona_security_policy", Expected: registrationExpectation{Count: 1}},
		{Name: "service_account", TypeName: "ona_service_account", Expected: registrationExpectation{Count: 1}},
		{Name: "skill", TypeName: "ona_skill", Expected: registrationExpectation{Count: 1}},
		{Name: "warm_pool", TypeName: "ona_warm_pool", Expected: registrationExpectation{Count: 1}},
		{Name: "webhook", TypeName: "ona_webhook", Expected: registrationExpectation{Count: 1}},
		{Name: "automation", TypeName: "ona_automation", Expected: registrationExpectation{Count: 1}},
	}

	provider := &OnaProvider{}
	registrations := make(map[string]int)
	for _, newResource := range provider.Resources(t.Context()) {
		var resp frameworkresource.MetadataResponse
		newResource().Metadata(t.Context(), frameworkresource.MetadataRequest{ProviderTypeName: "ona"}, &resp)
		registrations[resp.TypeName]++
	}

	testRegistrations(t, tests, registrations)
}

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"ona":  providerserver.NewProtocol6WithError(New("test")()),
	"echo": echoprovider.NewProviderServer(),
}

func testAccPreCheck(t *testing.T) {
	// You can add code here to run prior to any test case execution, for example assertions
	// about the appropriate environment variables being set are common to see in a pre-check
	// function.
}

func TestManagedResourcesImplementImportState(t *testing.T) {
	t.Parallel()

	provider := &OnaProvider{}
	for _, newResource := range provider.Resources(t.Context()) {
		resource := newResource()
		if _, ok := resource.(frameworkresource.ResourceWithImportState); !ok {
			t.Errorf("%T must implement ResourceWithImportState", resource)
		}
	}
}

func TestListResourceRegistrationsAreValid(t *testing.T) {
	t.Parallel()

	provider := &OnaProvider{}
	for i, newListResource := range provider.ListResources(t.Context()) {
		if newListResource == nil {
			t.Errorf("ListResources()[%d] is nil", i)
			continue
		}
		if got := newListResource(); got == nil {
			t.Errorf("ListResources()[%d]() returned nil", i)
		}
	}
}

func TestListResourcesAreRegistered(t *testing.T) {
	t.Parallel()

	tests := []registrationTestCase{
		{Name: "group", TypeName: "ona_group", Expected: registrationExpectation{Count: 1}},
		{Name: "group_membership", TypeName: "ona_group_membership", Expected: registrationExpectation{Count: 1}},
		{Name: "organization_role_assignment", TypeName: "ona_organization_role_assignment", Expected: registrationExpectation{Count: 1}},
		{Name: "announcement_banner", TypeName: "ona_announcement_banner", Expected: registrationExpectation{Count: 1}},
		{Name: "custom_domain", TypeName: "ona_custom_domain", Expected: registrationExpectation{Count: 1}},
		{Name: "oidc_config", TypeName: "ona_oidc_config", Expected: registrationExpectation{Count: 1}},
		{Name: "organization_policies", TypeName: "ona_organization_policies", Expected: registrationExpectation{Count: 1}},
		{Name: "scim_configuration", TypeName: "ona_scim_configuration", Expected: registrationExpectation{Count: 1}},
		{Name: "sso_configuration", TypeName: "ona_sso_configuration", Expected: registrationExpectation{Count: 1}},
		{Name: "terms_of_service", TypeName: "ona_terms_of_service", Expected: registrationExpectation{Count: 1}},
		{Name: "project_insights", TypeName: "ona_project_insights", Expected: registrationExpectation{Count: 1}},
		{Name: "project", TypeName: "ona_project", Expected: registrationExpectation{Count: 1}},
		{Name: "environment_class", TypeName: "ona_environment_class", Expected: registrationExpectation{Count: 1}},
		{Name: "runner", TypeName: "ona_runner", Expected: registrationExpectation{Count: 1}},
		{Name: "runner_policy", TypeName: "ona_runner_policy", Expected: registrationExpectation{Count: 1}},
		{Name: "scm_integration", TypeName: "ona_scm_integration", Expected: registrationExpectation{Count: 1}},
		{Name: "security_policy", TypeName: "ona_security_policy", Expected: registrationExpectation{Count: 1}},
		{Name: "secret", TypeName: "ona_secret", Expected: registrationExpectation{Count: 1}},
		{Name: "service_account", TypeName: "ona_service_account", Expected: registrationExpectation{Count: 1}},
		{Name: "skill", TypeName: "ona_skill", Expected: registrationExpectation{Count: 1}},
		{Name: "warm_pool", TypeName: "ona_warm_pool", Expected: registrationExpectation{Count: 1}},
	}

	provider := &OnaProvider{}
	registrations := make(map[string]int)
	for _, newListResource := range provider.ListResources(t.Context()) {
		var resp frameworkresource.MetadataResponse
		newListResource().Metadata(t.Context(), frameworkresource.MetadataRequest{ProviderTypeName: "ona"}, &resp)
		registrations[resp.TypeName]++
	}

	testRegistrations(t, tests, registrations)
}

func testRegistrations(t *testing.T, tests []registrationTestCase, registrations map[string]int) {
	t.Helper()

	var registrationCount int
	for _, count := range registrations {
		registrationCount += count
	}
	if diff := cmp.Diff(len(tests), registrationCount); diff != "" {
		t.Errorf("registration count mismatch (-want +got):\n%s", diff)
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := registrationExpectation{Count: registrations[tc.TypeName]}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("registration mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConfigureSharesProviderDataWithListResources(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	provider := &OnaProvider{version: "test"}
	var schemaResp frameworkprovider.SchemaResponse
	provider.Schema(ctx, frameworkprovider.SchemaRequest{}, &schemaResp)

	configType := schemaResp.Schema.Type().TerraformType(ctx)
	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw: tftypes.NewValue(configType, map[string]tftypes.Value{
			"host":  tftypes.NewValue(tftypes.String, "https://example.com"),
			"token": tftypes.NewValue(tftypes.String, "test-token"),
		}),
	}

	var resp frameworkprovider.ConfigureResponse
	provider.Configure(ctx, frameworkprovider.ConfigureRequest{Config: config}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure() diagnostics: %v", resp.Diagnostics)
	}
	if resp.ListResourceData == nil {
		t.Fatal("Configure() did not set ListResourceData")
	}
	if resp.ListResourceData != resp.ResourceData {
		t.Fatal("Configure() did not share provider data between list and managed resources")
	}
}
