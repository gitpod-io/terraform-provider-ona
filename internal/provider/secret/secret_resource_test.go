// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package secret

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"google.golang.org/protobuf/testing/protocmp"
)

const (
	testOrgID            = "01980ed3-a090-7b5b-a74c-9bf5d8cfe500"
	testProjectID        = "01980ed3-a090-7b5b-a74c-9bf5d8cfe501"
	testUserID           = "01980ed3-a090-7b5b-a74c-9bf5d8cfe502"
	testServiceAccountID = "01980ed3-a090-7b5b-a74c-9bf5d8cfe503"
	testSecretID         = "01980ed3-a090-7b5b-a74c-9bf5d8cfe504"
)

func TestCreateSecretRequest(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result *v1.CreateSecretRequest
		Err    string
	}

	tests := []struct {
		Name     string
		Input    Model
		Scope    resolvedScope
		Expected Expectation
	}{
		{
			Name: "organization_environment_variable",
			Input: Model{
				Scope:               types.StringValue(scopeOrganization),
				Name:                types.StringValue("THIRD_PARTY_API_KEY"),
				EnvironmentVariable: types.BoolValue(true),
			},
			Scope: resolvedScope{
				Scope: &v1.SecretScope{Scope: &v1.SecretScope_OrganizationId{OrganizationId: testOrgID}},
			},
			Expected: Expectation{
				Result: &v1.CreateSecretRequest{
					Name:  "THIRD_PARTY_API_KEY",
					Value: redactedSecretValue,
					Mount: &v1.CreateSecretRequest_EnvironmentVariable{EnvironmentVariable: true},
					Scope: &v1.SecretScope{Scope: &v1.SecretScope_OrganizationId{OrganizationId: testOrgID}},
				},
			},
		},
		{
			Name: "project_file_path",
			Input: Model{
				Scope:     types.StringValue(scopeProject),
				ProjectID: types.StringValue(testProjectID),
				Name:      types.StringValue("FILE_SECRET"),
				FilePath:  types.StringValue("/etc/secret/value"),
			},
			Scope: resolvedScope{
				Scope:     &v1.SecretScope{Scope: &v1.SecretScope_ProjectId{ProjectId: testProjectID}},
				ProjectID: types.StringValue(testProjectID),
			},
			Expected: Expectation{
				Result: &v1.CreateSecretRequest{
					Name:  "FILE_SECRET",
					Value: redactedSecretValue,
					Mount: &v1.CreateSecretRequest_FilePath{FilePath: "/etc/secret/value"},
					Scope: &v1.SecretScope{Scope: &v1.SecretScope_ProjectId{ProjectId: testProjectID}},
				},
			},
		},
		{
			Name: "user_registry",
			Input: Model{
				Scope:                          types.StringValue(scopeUser),
				UserID:                         types.StringValue(testUserID),
				Name:                           types.StringValue("REGISTRY_AUTH"),
				ContainerRegistryBasicAuthHost: types.StringValue("registry.example.com"),
			},
			Scope: resolvedScope{
				Scope:  &v1.SecretScope{Scope: &v1.SecretScope_UserId{UserId: testUserID}},
				UserID: types.StringValue(testUserID),
			},
			Expected: Expectation{
				Result: &v1.CreateSecretRequest{
					Name:  "REGISTRY_AUTH",
					Value: redactedSecretValue,
					Mount: &v1.CreateSecretRequest_ContainerRegistryBasicAuthHost{ContainerRegistryBasicAuthHost: "registry.example.com"},
					Scope: &v1.SecretScope{Scope: &v1.SecretScope_UserId{UserId: testUserID}},
				},
			},
		},
		{
			Name: "service_account_api_only_with_credential_proxy",
			Input: Model{
				Scope:            types.StringValue(scopeServiceAccount),
				ServiceAccountID: types.StringValue(testServiceAccountID),
				Name:             types.StringValue("API_SECRET"),
				APIOnly:          types.BoolValue(true),
				CredentialProxy: []CredentialProxyModel{
					{
						TargetHosts: mustStringSet(t, " github.com ", "", "*.github.com"),
						Header:      types.StringValue(" Authorization "),
					},
				},
			},
			Scope: resolvedScope{
				Scope:            &v1.SecretScope{Scope: &v1.SecretScope_ServiceAccountId{ServiceAccountId: testServiceAccountID}},
				ServiceAccountID: types.StringValue(testServiceAccountID),
			},
			Expected: Expectation{
				Result: &v1.CreateSecretRequest{
					Name:            "API_SECRET",
					Value:           redactedSecretValue,
					Mount:           &v1.CreateSecretRequest_ApiOnly{ApiOnly: true},
					CredentialProxy: &v1.Secret_CredentialProxy{TargetHosts: []string{"*.github.com", "github.com"}, Header: "Authorization"},
					Scope:           &v1.SecretScope{Scope: &v1.SecretScope_ServiceAccountId{ServiceAccountId: testServiceAccountID}},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			result, diags := createSecretRequest(t.Context(), tc.Input, types.StringValue("input-secret"), tc.Scope)
			if result != nil {
				result.Value = redactedSecretValue
			}
			got := Expectation{
				Result: result,
				Err:    diagnosticsString(diags),
			}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("createSecretRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateModel(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Err string
	}

	tests := []struct {
		Name     string
		Input    Model
		Expected Expectation
	}{
		{
			Name: "project_requires_project_id",
			Input: Model{
				Scope:               types.StringValue(scopeProject),
				Name:                types.StringValue("VALID_NAME"),
				EnvironmentVariable: types.BoolValue(true),
			},
			Expected: Expectation{
				Err: "Missing Project ID: Set project_id when scope is \"project\".",
			},
		},
		{
			Name: "mount_requires_exactly_one",
			Input: Model{
				Scope:               types.StringValue(scopeOrganization),
				Name:                types.StringValue("VALID_NAME"),
				EnvironmentVariable: types.BoolValue(true),
				APIOnly:             types.BoolValue(true),
			},
			Expected: Expectation{
				Err: "Invalid Secret Mount: Set exactly one of environment_variable, file_path, container_registry_basic_auth_host, or api_only.",
			},
		},
		{
			Name: "false_boolean_mount_rejected",
			Input: Model{
				Scope:               types.StringValue(scopeOrganization),
				Name:                types.StringValue("VALID_NAME"),
				EnvironmentVariable: types.BoolValue(false),
			},
			Expected: Expectation{
				Err: "Invalid Secret Mount: environment_variable can only be set to true.\nMissing Secret Mount: Set exactly one of environment_variable, file_path, container_registry_basic_auth_host, or api_only.",
			},
		},
		{
			Name: "file_path_must_be_absolute",
			Input: Model{
				Scope:    types.StringValue(scopeOrganization),
				Name:     types.StringValue("VALID_NAME"),
				FilePath: types.StringValue("relative/path"),
			},
			Expected: Expectation{
				Err: "Invalid Secret File Path: file_path must be an absolute path such as /path/to/file.",
			},
		},
		{
			Name: "credential_proxy_trims_and_requires_values",
			Input: Model{
				Scope:   types.StringValue(scopeOrganization),
				Name:    types.StringValue("VALID_NAME"),
				APIOnly: types.BoolValue(true),
				CredentialProxy: []CredentialProxyModel{
					{
						TargetHosts: mustStringSet(t, " "),
						Header:      types.StringValue(" "),
					},
				},
			},
			Expected: Expectation{
				Err: "Missing Credential Proxy Header: credential_proxy.header must not be empty.\nMissing Credential Proxy Target Hosts: credential_proxy.target_hosts must include at least one non-empty host.",
			},
		},
		{
			Name: "valid_user_scope_with_api_only_mount",
			Input: Model{
				Scope:   types.StringValue(scopeUser),
				UserID:  types.StringValue(testUserID),
				Name:    types.StringValue("VALID_NAME"),
				APIOnly: types.BoolValue(true),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			validateModel(t.Context(), tc.Input, true, &diags)
			got := Expectation{
				Err: diagnosticsString(diags),
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateModel() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseImportID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result importState
		Err    string
	}

	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			Name:  "organization",
			Input: "organization/" + testSecretID,
			Expected: Expectation{
				Result: importState{Scope: scopeOrganization, ID: testSecretID},
			},
		},
		{
			Name:  "project",
			Input: "project/" + testProjectID + "/" + testSecretID,
			Expected: Expectation{
				Result: importState{Scope: scopeProject, ProjectID: testProjectID, ID: testSecretID},
			},
		},
		{
			Name:  "user",
			Input: "user/" + testUserID + "/" + testSecretID,
			Expected: Expectation{
				Result: importState{Scope: scopeUser, UserID: testUserID, ID: testSecretID},
			},
		},
		{
			Name:  "service_account",
			Input: "service_account/" + testServiceAccountID + "/" + testSecretID,
			Expected: Expectation{
				Result: importState{Scope: scopeServiceAccount, ServiceAccountID: testServiceAccountID, ID: testSecretID},
			},
		},
		{
			Name:  "bare_id_rejected",
			Input: testSecretID,
			Expected: Expectation{
				Err: invalidImportIDDiagnostic,
			},
		},
		{
			Name:  "malformed_id_rejected",
			Input: "project/not-a-uuid/" + testSecretID,
			Expected: Expectation{
				Err: invalidImportIDDiagnostic,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			result, diags := parseImportID(tc.Input)
			got := Expectation{
				Result: result,
				Err:    diagnosticsString(diags),
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("parseImportID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestImportStateLegacyAndIdentitySeedEquivalentState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name                   string
		LegacyID               string
		Identity               IdentityModel
		LegacyNeedsIdentityAPI bool
		Expected               importStateSnapshot
	}{
		{
			Name:                   "organization",
			LegacyID:               "organization/" + testSecretID,
			LegacyNeedsIdentityAPI: true,
			Identity: IdentityModel{
				ID:               types.StringValue(testSecretID),
				Scope:            types.StringValue(scopeOrganization),
				OrganizationID:   types.StringValue(testOrgID),
				ProjectID:        types.StringNull(),
				UserID:           types.StringNull(),
				ServiceAccountID: types.StringNull(),
			},
			Expected: importStateSnapshot{
				ID:               testSecretID,
				Scope:            scopeOrganization,
				ProjectID:        nullSnapshotValue,
				UserID:           nullSnapshotValue,
				ServiceAccountID: nullSnapshotValue,
			},
		},
		{
			Name:     "project",
			LegacyID: "project/" + testProjectID + "/" + testSecretID,
			Identity: IdentityModel{
				ID:               types.StringValue(testSecretID),
				Scope:            types.StringValue(scopeProject),
				OrganizationID:   types.StringNull(),
				ProjectID:        types.StringValue(testProjectID),
				UserID:           types.StringNull(),
				ServiceAccountID: types.StringNull(),
			},
			Expected: importStateSnapshot{
				ID:               testSecretID,
				Scope:            scopeProject,
				ProjectID:        testProjectID,
				UserID:           nullSnapshotValue,
				ServiceAccountID: nullSnapshotValue,
			},
		},
		{
			Name:     "user",
			LegacyID: "user/" + testUserID + "/" + testSecretID,
			Identity: IdentityModel{
				ID:               types.StringValue(testSecretID),
				Scope:            types.StringValue(scopeUser),
				OrganizationID:   types.StringNull(),
				ProjectID:        types.StringNull(),
				UserID:           types.StringValue(testUserID),
				ServiceAccountID: types.StringNull(),
			},
			Expected: importStateSnapshot{
				ID:               testSecretID,
				Scope:            scopeUser,
				ProjectID:        nullSnapshotValue,
				UserID:           testUserID,
				ServiceAccountID: nullSnapshotValue,
			},
		},
		{
			Name:     "service_account",
			LegacyID: "service_account/" + testServiceAccountID + "/" + testSecretID,
			Identity: IdentityModel{
				ID:               types.StringValue(testSecretID),
				Scope:            types.StringValue(scopeServiceAccount),
				OrganizationID:   types.StringNull(),
				ProjectID:        types.StringNull(),
				UserID:           types.StringNull(),
				ServiceAccountID: types.StringValue(testServiceAccountID),
			},
			Expected: importStateSnapshot{
				ID:               testSecretID,
				Scope:            scopeServiceAccount,
				ProjectID:        nullSnapshotValue,
				UserID:           nullSnapshotValue,
				ServiceAccountID: testServiceAccountID,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			legacy := runImportState(t, resource.ImportStateRequest{ID: tc.LegacyID}, tc.LegacyNeedsIdentityAPI)
			identity := newTestResourceIdentity(t, tc.Identity)
			structured := runImportState(t, resource.ImportStateRequest{Identity: identity}, false)

			if diff := cmp.Diff(tc.Expected, legacy); diff != "" {
				t.Errorf("legacy import state mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.Expected, structured); diff != "" {
				t.Errorf("structured import state mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(legacy, structured); diff != "" {
				t.Errorf("legacy and structured import state differ (-legacy +structured):\n%s", diff)
			}
		})
	}
}

func mustStringSet(t *testing.T, values ...string) types.Set {
	t.Helper()

	result, diags := types.SetValueFrom(t.Context(), types.StringType, values)
	if diags.HasError() {
		t.Fatalf("types.SetValueFrom() failed: %s", diagnosticsString(diags))
	}
	return result
}

func diagnosticsString(diags diag.Diagnostics) string {
	if !diags.HasError() {
		return ""
	}
	var parts []string
	for _, item := range diags {
		if item.Severity() == diag.SeverityError {
			parts = append(parts, item.Summary()+": "+item.Detail())
		}
	}
	return strings.Join(parts, "\n")
}

func runImportState(t *testing.T, req resource.ImportStateRequest, expectIdentityLookup bool) importStateSnapshot {
	t.Helper()

	ctx := t.Context()
	schema := resourceSchema()
	identitySchema := testIdentitySchema(t)
	resp := resource.ImportStateResponse{
		State:    newTestState(t, ctx, schema),
		Identity: newTestIdentityData(t, ctx, identitySchema),
	}

	client := managementclient.NewWithServices(managementclient.Services{
		IdentityService: &testIdentityClient{t: t, allowLookup: expectIdentityLookup},
	})
	r := &Resource{client: client}
	r.ImportState(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ImportState() returned diagnostics: %s", diagnosticsString(resp.Diagnostics))
	}

	return snapshotImportState(t, ctx, resp.State)
}

type testIdentityClient struct {
	t           *testing.T
	allowLookup bool
}

func (c *testIdentityClient) GetAuthenticatedIdentity(context.Context, *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	c.t.Helper()
	if !c.allowLookup {
		c.t.Fatal("unexpected authenticated identity lookup")
	}
	return connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{OrganizationId: testOrgID}), nil
}

func (c *testIdentityClient) GetIDToken(context.Context, *connect.Request[v1.GetIDTokenRequest]) (*connect.Response[v1.GetIDTokenResponse], error) {
	c.t.Helper()
	c.t.Fatal("unexpected ID token request")
	return nil, errors.New("unexpected ID token request")
}

func (c *testIdentityClient) ExchangeToken(context.Context, *connect.Request[v1.ExchangeTokenRequest]) (*connect.Response[v1.ExchangeTokenResponse], error) {
	c.t.Helper()
	c.t.Fatal("unexpected token exchange")
	return nil, errors.New("unexpected token exchange")
}

func newTestResourceIdentity(t *testing.T, data IdentityModel) *tfsdk.ResourceIdentity {
	t.Helper()

	ctx := t.Context()
	identity := newTestIdentityData(t, ctx, testIdentitySchema(t))
	diags := identity.Set(ctx, data)
	if diags.HasError() {
		t.Fatalf("identity.Set() failed: %s", diagnosticsString(diags))
	}
	return identity
}

func testIdentitySchema(t *testing.T) identityschema.Schema {
	t.Helper()

	r := &Resource{}
	var resp resource.IdentitySchemaResponse
	r.IdentitySchema(t.Context(), resource.IdentitySchemaRequest{}, &resp)
	return resp.IdentitySchema
}

func newTestState(t *testing.T, ctx context.Context, schema resourceschema.Schema) tfsdk.State {
	t.Helper()

	return tfsdk.State{
		Schema: schema,
		Raw:    nullObjectValue(t, schema.Type().TerraformType(ctx)),
	}
}

func newTestIdentityData(t *testing.T, ctx context.Context, schema identityschema.Schema) *tfsdk.ResourceIdentity {
	t.Helper()

	return &tfsdk.ResourceIdentity{
		Schema: schema,
		Raw:    nullObjectValue(t, schema.Type().TerraformType(ctx)),
	}
}

func nullObjectValue(t *testing.T, typ tftypes.Type) tftypes.Value {
	t.Helper()

	objectType, ok := typ.(tftypes.Object)
	if !ok {
		t.Fatalf("expected object Terraform type, got %T", typ)
	}
	values := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attrType := range objectType.AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
	}
	return tftypes.NewValue(typ, values)
}

func snapshotImportState(t *testing.T, ctx context.Context, state tfsdk.State) importStateSnapshot {
	t.Helper()

	var model Model
	diags := state.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("state.Get() failed: %s", diagnosticsString(diags))
	}
	return importStateSnapshot{
		ID:               snapshotString(model.ID),
		Scope:            snapshotString(model.Scope),
		ProjectID:        snapshotString(model.ProjectID),
		UserID:           snapshotString(model.UserID),
		ServiceAccountID: snapshotString(model.ServiceAccountID),
	}
}

func snapshotString(value types.String) string {
	switch {
	case value.IsNull():
		return nullSnapshotValue
	case value.IsUnknown():
		return unknownSnapshotValue
	default:
		return value.ValueString()
	}
}

type importStateSnapshot struct {
	ID               string
	Scope            string
	ProjectID        string
	UserID           string
	ServiceAccountID string
}

const (
	redactedSecretValue       = "<redacted>"
	invalidImportIDDiagnostic = "Invalid Import ID: Expected one of: organization/<secret_id>, project/<project_id>/<secret_id>, user/<user_id>/<secret_id>, or service_account/<service_account_id>/<secret_id>."
	nullSnapshotValue         = "<null>"
	unknownSnapshotValue      = "<unknown>"
)
