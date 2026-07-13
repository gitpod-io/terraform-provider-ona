// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
)

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

func TestListResourcesInitiallyEmpty(t *testing.T) {
	t.Parallel()

	provider := &OnaProvider{}
	if got := provider.ListResources(t.Context()); len(got) != 0 {
		t.Fatalf("ListResources() returned %d registrations, want 0", len(got))
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
