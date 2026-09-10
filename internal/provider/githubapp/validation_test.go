// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package githubapp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func testConfig() Model {
	m := emptyModel()
	m.AppID = types.StringValue("123")
	m.Enabled = types.BoolValue(true)
	m.CredentialsVersion = types.Int64Value(1)
	m.Credentials = types.ObjectValueMust(credentialsTypes, map[string]attr.Value{"private_key": types.StringValue("private-must-not-appear"), "client_secret": types.StringValue("client-must-not-appear"), "webhook_secret": types.StringValue("webhook-must-not-appear")})
	return m
}

func testEmptyCredentials() types.Object {
	return types.ObjectValueMust(credentialsTypes, map[string]attr.Value{
		"private_key": types.StringNull(), "client_secret": types.StringNull(), "webhook_secret": types.StringNull(),
	})
}

func modelState(t *testing.T, model Model) tfsdk.State {
	t.Helper()
	state := tfsdk.State{Schema: resourceSchema()}
	if diags := state.Set(t.Context(), &model); diags.HasError() {
		t.Fatalf("test state: %v", diags)
	}
	return state
}

func modelConfig(t *testing.T, model Model) tfsdk.Config {
	t.Helper()
	state := modelState(t, model)
	return tfsdk.Config(state)
}
func modelPlan(t *testing.T, model Model) tfsdk.Plan {
	t.Helper()
	state := modelState(t, model)
	return tfsdk.Plan(state)
}

func TestAppIDValidator(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name  string
		Value types.String
		Valid bool
	}{
		{"positive", types.StringValue("123"), true}, {"max", types.StringValue("9223372036854775807"), true}, {"overflow", types.StringValue("9223372036854775808"), false}, {"zero", types.StringValue("0"), false}, {"leading_zero", types.StringValue("01"), false}, {"sign", types.StringValue("+1"), false}, {"negative", types.StringValue("-1"), false}, {"whitespace", types.StringValue(" 1"), false}, {"empty", types.StringValue(""), false}, {"unknown", types.StringUnknown(), true}, {"null_deferred_to_required_schema", types.StringNull(), true},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			type Expectation struct{ Valid bool }
			var resp validator.StringResponse
			appIDValidator{}.ValidateString(t.Context(), validator.StringRequest{ConfigValue: tc.Value, Path: path.Root("app_id")}, &resp)
			if diff := cmp.Diff(Expectation{tc.Valid}, Expectation{!resp.Diagnostics.HasError()}); diff != "" {
				t.Errorf("validator mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCredentialValidation(t *testing.T) {
	t.Parallel()
	type Expectation struct{ Errors []string }
	tests := []struct {
		Name              string
		Mutate            func(*Model)
		Required, Unknown bool
		Expected          Expectation
	}{
		{Name: "complete", Required: true, Expected: Expectation{}},
		{Name: "import_without_credentials", Mutate: func(m *Model) {
			m.Credentials = types.ObjectNull(credentialsTypes)
			m.CredentialsVersion = types.Int64Null()
		}, Expected: Expectation{}},
		{Name: "create_without_credentials", Mutate: func(m *Model) { m.Credentials = types.ObjectNull(credentialsTypes) }, Required: true, Expected: Expectation{[]string{"Missing GitHub App Credentials"}}},
		{Name: "import_with_all_null_credentials", Mutate: func(m *Model) {
			m.Credentials = testEmptyCredentials()
			m.CredentialsVersion = types.Int64Null()
		}, Expected: Expectation{}},
		{Name: "unchanged_version_with_all_null_credentials", Mutate: func(m *Model) { m.Credentials = testEmptyCredentials() }, Expected: Expectation{}},
		{Name: "mutation_requires_nonempty_bundle", Mutate: func(m *Model) { m.Credentials = testEmptyCredentials() }, Required: true, Expected: Expectation{[]string{"Missing GitHub App Credentials"}}},
		{Name: "unknown_children_without_version_deferred", Mutate: func(m *Model) {
			m.Credentials = types.ObjectValueMust(credentialsTypes, map[string]attr.Value{
				"private_key": types.StringUnknown(), "client_secret": types.StringUnknown(), "webhook_secret": types.StringUnknown(),
			})
			m.CredentialsVersion = types.Int64Null()
		}, Unknown: true, Expected: Expectation{}},
		{Name: "unknown_or_null_object_can_resolve_absent", Mutate: func(m *Model) {
			a := testEmptyCredentials().Attributes()
			a["private_key"] = types.StringUnknown()
			m.Credentials = types.ObjectValueMust(credentialsTypes, a)
			m.CredentialsVersion = types.Int64Null()
		}, Unknown: true, Expected: Expectation{}},
		{Name: "unknown_bundle_deferred", Mutate: func(m *Model) { m.Credentials = types.ObjectUnknown(credentialsTypes) }, Required: true, Unknown: true, Expected: Expectation{}},
		{Name: "unknown_bundle_apply_rejected", Mutate: func(m *Model) { m.Credentials = types.ObjectUnknown(credentialsTypes) }, Required: true, Expected: Expectation{[]string{"Missing GitHub App Credentials"}}},
		{Name: "bundle_requires_version", Mutate: func(m *Model) { m.CredentialsVersion = types.Int64Null() }, Expected: Expectation{[]string{"Missing GitHub App Credentials Version"}}},
		{Name: "unknown_child_deferred", Mutate: func(m *Model) {
			a := m.Credentials.Attributes()
			a["private_key"] = types.StringUnknown()
			m.Credentials = types.ObjectValueMust(credentialsTypes, a)
		}, Unknown: true, Expected: Expectation{}},
		{Name: "missing_child", Mutate: func(m *Model) {
			a := m.Credentials.Attributes()
			a["private_key"] = types.StringNull()
			m.Credentials = types.ObjectValueMust(credentialsTypes, a)
		}, Expected: Expectation{[]string{"Incomplete GitHub App Credentials"}}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			m := testConfig()
			if tc.Mutate != nil {
				tc.Mutate(&m)
			}
			var diags diag.Diagnostics
			validateCredentials(t.Context(), m, tc.Required, tc.Unknown, &diags)
			if diff := cmp.Diff(tc.Expected, Expectation{errorSummaries(diags)}); diff != "" {
				t.Errorf("credential validation mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestModifyPlanCredentials(t *testing.T) {
	t.Parallel()
	type Expectation struct{ Errors []string }
	tests := []struct {
		Name             string
		Create           bool
		Version          types.Int64
		Credentials      bool
		EmptyCredentials bool
		Expected         Expectation
	}{
		{Name: "create_requires_bundle", Create: true, Version: types.Int64Value(1), Expected: Expectation{[]string{"Missing GitHub App Credentials"}}},
		{Name: "create_requires_version", Create: true, Version: types.Int64Null(), Credentials: true, Expected: Expectation{[]string{"Missing GitHub App Credentials Version"}}},
		{Name: "unchanged_marker_without_bundle", Version: types.Int64Value(1)},
		{Name: "unchanged_marker_with_empty_bundle", Version: types.Int64Value(1), EmptyCredentials: true},
		{Name: "create_rejects_empty_bundle", Create: true, Version: types.Int64Value(1), EmptyCredentials: true, Expected: Expectation{[]string{"Missing GitHub App Credentials"}}},
		{Name: "rotation_rejects_empty_bundle", Version: types.Int64Value(2), EmptyCredentials: true, Expected: Expectation{[]string{"Missing GitHub App Credentials"}}},
		{Name: "changed_marker_requires_bundle", Version: types.Int64Value(2), Expected: Expectation{[]string{"Missing GitHub App Credentials"}}},
		{Name: "rotation_complete", Version: types.Int64Value(2), Credentials: true},
		{Name: "unknown_version_deferred", Version: types.Int64Unknown()},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			prior := testConfig()
			prior.Credentials = types.ObjectNull(credentialsTypes)
			prior.ID = types.StringValue(testID)
			state := modelState(t, prior)
			if tc.Create {
				state.Raw = tftypes.NewValue(state.Schema.Type().TerraformType(t.Context()), nil)
			}
			config := testConfig()
			config.CredentialsVersion = tc.Version
			if !tc.Credentials {
				config.Credentials = types.ObjectNull(credentialsTypes)
			}
			if tc.EmptyCredentials {
				config.Credentials = testEmptyCredentials()
			}
			plan := config
			plan.Credentials = types.ObjectNull(credentialsTypes)
			req := resource.ModifyPlanRequest{Plan: modelPlan(t, plan), Config: modelConfig(t, config), State: state}
			var resp resource.ModifyPlanResponse
			(&Resource{}).ModifyPlan(t.Context(), req, &resp)
			if diff := cmp.Diff(tc.Expected, Expectation{errorSummaries(resp.Diagnostics)}); diff != "" {
				t.Errorf("modify plan mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSchemaWriteOnlyCredentials(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		ParentSensitive, ParentWriteOnly bool
		Children                         map[string]bool
		ReplacementModifiers             int
		SlugModifiers                    int
	}
	s := resourceSchema()
	parent, ok := s.Attributes["credentials"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("credentials: expected schema.SingleNestedAttribute, got %T", s.Attributes["credentials"])
	}
	appID, ok := s.Attributes["app_id"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("app_id: expected schema.StringAttribute, got %T", s.Attributes["app_id"])
	}
	appSlug, ok := s.Attributes["app_slug"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("app_slug: expected schema.StringAttribute, got %T", s.Attributes["app_slug"])
	}
	got := Expectation{ParentSensitive: parent.Sensitive, ParentWriteOnly: parent.WriteOnly, Children: map[string]bool{}, ReplacementModifiers: len(appID.PlanModifiers), SlugModifiers: len(appSlug.PlanModifiers)}
	for name, a := range parent.Attributes {
		child, ok := a.(schema.StringAttribute)
		if !ok {
			t.Fatalf("credentials.%s: expected schema.StringAttribute, got %T", name, a)
		}
		got.Children[name] = child.Optional && !child.Required && child.Sensitive && child.WriteOnly
	}
	want := Expectation{ParentSensitive: true, ParentWriteOnly: true, Children: map[string]bool{"private_key": true, "client_secret": true, "webhook_secret": true}, ReplacementModifiers: 1}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("schema mismatch (-want +got):\n%s", diff)
	}
}
