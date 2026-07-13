// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package secret

import (
	"context"
	"sort"
	"strings"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type resolvedScope struct {
	Scope            *v1.SecretScope
	OrganizationID   types.String
	ProjectID        types.String
	UserID           types.String
	ServiceAccountID types.String
}

func createSecretRequest(ctx context.Context, data Model, value types.String, scope resolvedScope) (*v1.CreateSecretRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateModel(ctx, data, true, &diags)
	if !isKnownString(value) {
		diags.AddAttributeError(pathRoot("value"), "Missing Secret Value", "Set value when creating an ona_secret resource.")
	}
	if diags.HasError() {
		return nil, diags
	}

	req := &v1.CreateSecretRequest{
		Name:  data.Name.ValueString(),
		Value: value.ValueString(),
		Scope: scope.Scope,
	}
	setSecretCreateMount(req, data)
	req.CredentialProxy = credentialProxyFromModel(ctx, data.CredentialProxy, &diags)
	if diags.HasError() {
		return nil, diags
	}
	return req, diags
}

func setSecretCreateMount(req *v1.CreateSecretRequest, data Model) {
	switch {
	case isKnownBool(data.EnvironmentVariable) && data.EnvironmentVariable.ValueBool():
		req.Mount = &v1.CreateSecretRequest_EnvironmentVariable{EnvironmentVariable: true}
	case isKnownString(data.FilePath):
		req.Mount = &v1.CreateSecretRequest_FilePath{FilePath: data.FilePath.ValueString()}
	case isKnownString(data.ContainerRegistryBasicAuthHost):
		req.Mount = &v1.CreateSecretRequest_ContainerRegistryBasicAuthHost{ContainerRegistryBasicAuthHost: data.ContainerRegistryBasicAuthHost.ValueString()}
	case isKnownBool(data.APIOnly) && data.APIOnly.ValueBool():
		req.Mount = &v1.CreateSecretRequest_ApiOnly{ApiOnly: true}
	}
}

func secretScopeFromModel(data Model) (*v1.SecretScope, diag.Diagnostics) {
	var diags diag.Diagnostics
	if data.Scope.IsUnknown() || data.Scope.IsNull() {
		diags.AddAttributeError(pathRoot("scope"), "Unknown Secret Scope", "scope must be known before apply.")
		return nil, diags
	}

	switch data.Scope.ValueString() {
	case scopeOrganization:
		if !isKnownString(data.OrganizationID) {
			diags.AddAttributeError(pathRoot("organization_id"), "Missing Organization ID", "The provider could not infer an organization ID from the authenticated identity.")
			return nil, diags
		}
		return &v1.SecretScope{Scope: &v1.SecretScope_OrganizationId{OrganizationId: data.OrganizationID.ValueString()}}, diags
	case scopeProject:
		return &v1.SecretScope{Scope: &v1.SecretScope_ProjectId{ProjectId: data.ProjectID.ValueString()}}, diags
	case scopeUser:
		return &v1.SecretScope{Scope: &v1.SecretScope_UserId{UserId: data.UserID.ValueString()}}, diags
	case scopeServiceAccount:
		return &v1.SecretScope{Scope: &v1.SecretScope_ServiceAccountId{ServiceAccountId: data.ServiceAccountID.ValueString()}}, diags
	default:
		diags.AddAttributeError(pathRoot("scope"), "Invalid Secret Scope", "Supported values are \"organization\", \"project\", \"user\", and \"service_account\".")
		return nil, diags
	}
}

func credentialProxyFromModel(ctx context.Context, values []CredentialProxyModel, diags *diag.Diagnostics) *v1.Secret_CredentialProxy {
	if len(values) == 0 {
		return nil
	}
	value := values[0]
	targetHosts := normalizedTargetHosts(ctx, value.TargetHosts, diags)
	if diags.HasError() {
		return nil
	}
	return &v1.Secret_CredentialProxy{
		TargetHosts: targetHosts,
		Header:      strings.TrimSpace(value.Header.ValueString()),
	}
}

func normalizedTargetHosts(ctx context.Context, value types.Set, diags *diag.Diagnostics) []string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var targetHosts []string
	diags.Append(value.ElementsAs(ctx, &targetHosts, false)...)
	if diags.HasError() {
		return nil
	}
	normalized := make([]string, 0, len(targetHosts))
	for _, targetHost := range targetHosts {
		targetHost = strings.TrimSpace(targetHost)
		if targetHost == "" {
			continue
		}
		normalized = append(normalized, targetHost)
	}
	sort.Strings(normalized)
	return normalized
}

func populateModelFromSecret(ctx context.Context, data *Model, secret *v1.Secret, diags *diag.Diagnostics) {
	data.ID = types.StringValue(secret.GetId())
	data.Name = types.StringValue(secret.GetName())
	data.CreatedAt = timestampValue(secret.GetCreatedAt())
	data.UpdatedAt = timestampValue(secret.GetUpdatedAt())
	data.Creator = subjectObjectFromProto(secret.GetCreator(), diags)
	data.Value = types.StringNull()
	populateScopeFromProto(data, secret.GetScope())
	populateMountFromProto(data, secret)
	populateCredentialProxyFromProto(ctx, data, secret.GetCredentialProxy(), diags)
}

func populateScopeFromProto(data *Model, scope *v1.SecretScope) {
	data.ProjectID = types.StringNull()
	data.UserID = types.StringNull()
	data.ServiceAccountID = types.StringNull()

	switch scope.GetScope().(type) {
	case *v1.SecretScope_OrganizationId:
		data.Scope = types.StringValue(scopeOrganization)
		data.OrganizationID = stringOptionalValue(scope.GetOrganizationId())
	case *v1.SecretScope_ProjectId:
		data.Scope = types.StringValue(scopeProject)
		data.ProjectID = stringOptionalValue(scope.GetProjectId())
	case *v1.SecretScope_UserId:
		data.Scope = types.StringValue(scopeUser)
		data.UserID = stringOptionalValue(scope.GetUserId())
	case *v1.SecretScope_ServiceAccountId:
		data.Scope = types.StringValue(scopeServiceAccount)
		data.ServiceAccountID = stringOptionalValue(scope.GetServiceAccountId())
	}
}

func populateMountFromProto(data *Model, secret *v1.Secret) {
	data.EnvironmentVariable = types.BoolNull()
	data.FilePath = types.StringNull()
	data.ContainerRegistryBasicAuthHost = types.StringNull()
	data.APIOnly = types.BoolNull()

	switch secret.GetMount().(type) {
	case *v1.Secret_EnvironmentVariable:
		data.EnvironmentVariable = types.BoolValue(secret.GetEnvironmentVariable())
	case *v1.Secret_FilePath:
		data.FilePath = stringOptionalValue(secret.GetFilePath())
	case *v1.Secret_ContainerRegistryBasicAuthHost:
		data.ContainerRegistryBasicAuthHost = stringOptionalValue(secret.GetContainerRegistryBasicAuthHost())
	case *v1.Secret_ApiOnly:
		data.APIOnly = types.BoolValue(secret.GetApiOnly())
	}
}

func populateCredentialProxyFromProto(ctx context.Context, data *Model, proxy *v1.Secret_CredentialProxy, diags *diag.Diagnostics) {
	if proxy == nil {
		data.CredentialProxy = nil
		return
	}
	targetHosts, setDiags := types.SetValueFrom(ctx, types.StringType, proxy.GetTargetHosts())
	diags.Append(setDiags...)
	if diags.HasError() {
		return
	}
	data.CredentialProxy = []CredentialProxyModel{{
		TargetHosts: targetHosts,
		Header:      stringOptionalValue(proxy.GetHeader()),
	}}
}

func preservePlannedInputs(data *Model, planned Model) {
	data.Scope = preserveString(data.Scope, planned.Scope)
	data.OrganizationID = preserveString(data.OrganizationID, planned.OrganizationID)
	data.ProjectID = preserveString(data.ProjectID, planned.ProjectID)
	data.UserID = preserveString(data.UserID, planned.UserID)
	data.ServiceAccountID = preserveString(data.ServiceAccountID, planned.ServiceAccountID)
	data.Name = preserveString(data.Name, planned.Name)
	data.Value = types.StringNull()
	data.ValueVersion = preserveString(data.ValueVersion, planned.ValueVersion)
	data.EnvironmentVariable = preserveBool(data.EnvironmentVariable, planned.EnvironmentVariable)
	data.FilePath = preserveString(data.FilePath, planned.FilePath)
	data.ContainerRegistryBasicAuthHost = preserveString(data.ContainerRegistryBasicAuthHost, planned.ContainerRegistryBasicAuthHost)
	data.APIOnly = preserveBool(data.APIOnly, planned.APIOnly)
	if len(planned.CredentialProxy) > 0 {
		data.CredentialProxy = planned.CredentialProxy
	}
}

func preserveTerraformOnlyState(data *Model, prior Model) {
	data.Value = types.StringNull()
	data.ValueVersion = prior.ValueVersion
}

func subjectObjectFromProto(subject *v1.Subject, diags *diag.Diagnostics) types.Object {
	if subject == nil {
		return types.ObjectNull(subjectObjectAttributeTypes)
	}
	result, objectDiags := types.ObjectValue(
		subjectObjectAttributeTypes,
		map[string]attr.Value{
			"id":        stringOptionalValue(subject.GetId()),
			"principal": stringOptionalValue(principalToString(subject.GetPrincipal())),
		},
	)
	diags.Append(objectDiags...)
	return result
}

func principalToString(principal v1.Principal) string {
	switch principal {
	case v1.Principal_PRINCIPAL_USER:
		return principalUser
	case v1.Principal_PRINCIPAL_SERVICE_ACCOUNT:
		return principalServiceAccount
	case v1.Principal_PRINCIPAL_ACCOUNT:
		return "account"
	case v1.Principal_PRINCIPAL_RUNNER:
		return "runner"
	case v1.Principal_PRINCIPAL_ENVIRONMENT:
		return "environment"
	case v1.Principal_PRINCIPAL_RUNNER_MANAGER:
		return "runner_manager"
	default:
		return ""
	}
}

func timestampValue(ts *timestamppb.Timestamp) types.String {
	if ts == nil || !ts.IsValid() {
		return types.StringNull()
	}
	return types.StringValue(ts.AsTime().UTC().Format(time.RFC3339))
}

func stringOptionalValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func preserveString(current types.String, planned types.String) types.String {
	if !planned.IsNull() && !planned.IsUnknown() {
		return planned
	}
	return current
}

func preserveBool(current types.Bool, planned types.Bool) types.Bool {
	if !planned.IsNull() && !planned.IsUnknown() {
		return planned
	}
	return current
}

func pathRoot(name string) path.Path {
	return path.Root(name)
}
