// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package secret

import (
	"context"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	secretNamePattern = regexp.MustCompile(`^[0-9A-Za-z_]{3,127}$`)
	uuidPattern       = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

var prohibitedEnvironmentVariableNames = map[string]struct{}{
	"HOME":       {},
	"LD_PRELOAD": {},
	"PATH":       {},
	"PWD":        {},
	"SHELL":      {},
	"USER":       {},
}

func validateModel(ctx context.Context, data Model, requireKnown bool, diags *diag.Diagnostics) {
	validateName(data, diags)
	validateScope(data, requireKnown, diags)
	validateMount(data, requireKnown, diags)
	validateCredentialProxy(ctx, data, requireKnown, diags)
}

func validateName(data Model, diags *diag.Diagnostics) {
	if !isKnownString(data.Name) {
		return
	}
	name := data.Name.ValueString()
	if !secretNamePattern.MatchString(name) {
		diags.AddAttributeError(path.Root("name"), "Invalid Secret Name", "Secret name must be 3 to 127 characters and contain only letters, digits, and underscores.")
	}
}

func validateScope(data Model, requireKnown bool, diags *diag.Diagnostics) {
	if data.Scope.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(path.Root("scope"), "Unknown Secret Scope", "scope must be known before apply.")
		}
		return
	}
	if data.Scope.IsNull() || data.Scope.ValueString() == "" {
		return
	}

	validateUUIDAttribute(data.ProjectID, path.Root("project_id"), diags)
	validateUUIDAttribute(data.UserID, path.Root("user_id"), diags)
	validateUUIDAttribute(data.ServiceAccountID, path.Root("service_account_id"), diags)

	hasProjectID := isKnownString(data.ProjectID)
	hasUserID := isKnownString(data.UserID)
	hasServiceAccountID := isKnownString(data.ServiceAccountID)

	switch data.Scope.ValueString() {
	case scopeOrganization:
		if hasProjectID || hasUserID || hasServiceAccountID {
			diags.AddAttributeError(path.Root("scope"), "Invalid Secret Scope", "Do not set project_id, user_id, or service_account_id when scope is \"organization\".")
		}
	case scopeProject:
		if !hasProjectID {
			if requireKnown || data.ProjectID.IsNull() {
				diags.AddAttributeError(path.Root("project_id"), "Missing Project ID", "Set project_id when scope is \"project\".")
			}
		}
		if hasUserID || hasServiceAccountID {
			diags.AddAttributeError(path.Root("scope"), "Invalid Secret Scope", "When scope is \"project\", set only project_id.")
		}
	case scopeUser:
		if hasProjectID || hasServiceAccountID {
			diags.AddAttributeError(path.Root("scope"), "Invalid Secret Scope", "When scope is \"user\", do not set project_id or service_account_id.")
		}
	case scopeServiceAccount:
		if !hasServiceAccountID {
			if requireKnown || data.ServiceAccountID.IsNull() {
				diags.AddAttributeError(path.Root("service_account_id"), "Missing Service Account ID", "Set service_account_id when scope is \"service_account\".")
			}
		}
		if hasProjectID || hasUserID {
			diags.AddAttributeError(path.Root("scope"), "Invalid Secret Scope", "When scope is \"service_account\", set only service_account_id.")
		}
	default:
		diags.AddAttributeError(path.Root("scope"), "Invalid Secret Scope", "Supported values are \"organization\", \"project\", \"user\", and \"service_account\".")
	}
}

func validateMount(data Model, requireKnown bool, diags *diag.Diagnostics) {
	mounts := 0
	unknowns := 0

	switch {
	case data.EnvironmentVariable.IsUnknown():
		unknowns++
	case isKnownBool(data.EnvironmentVariable):
		if !data.EnvironmentVariable.ValueBool() {
			diags.AddAttributeError(path.Root("environment_variable"), "Invalid Secret Mount", "environment_variable can only be set to true.")
		} else {
			mounts++
			validateEnvironmentVariableName(data, diags)
		}
	}

	switch {
	case data.FilePath.IsUnknown():
		unknowns++
	case isKnownString(data.FilePath):
		mounts++
		if !validAbsoluteFilePath(data.FilePath.ValueString()) {
			diags.AddAttributeError(path.Root("file_path"), "Invalid Secret File Path", "file_path must be an absolute path such as /path/to/file.")
		}
	}

	switch {
	case data.ContainerRegistryBasicAuthHost.IsUnknown():
		unknowns++
	case isKnownString(data.ContainerRegistryBasicAuthHost):
		mounts++
	}

	switch {
	case data.APIOnly.IsUnknown():
		unknowns++
	case isKnownBool(data.APIOnly):
		if !data.APIOnly.ValueBool() {
			diags.AddAttributeError(path.Root("api_only"), "Invalid Secret Mount", "api_only can only be set to true.")
		} else {
			mounts++
		}
	}

	if mounts > 1 {
		diags.AddAttributeError(path.Root("scope"), "Invalid Secret Mount", "Set exactly one of environment_variable, file_path, container_registry_basic_auth_host, or api_only.")
		return
	}
	if mounts == 0 && (requireKnown || unknowns == 0) {
		diags.AddAttributeError(path.Root("scope"), "Missing Secret Mount", "Set exactly one of environment_variable, file_path, container_registry_basic_auth_host, or api_only.")
	}
}

func validateEnvironmentVariableName(data Model, diags *diag.Diagnostics) {
	if !isKnownString(data.Name) {
		return
	}
	if _, ok := prohibitedEnvironmentVariableNames[strings.ToUpper(data.Name.ValueString())]; ok {
		diags.AddAttributeError(path.Root("name"), "Invalid Environment Variable Secret Name", "This name is reserved by Ona for environment-variable secrets.")
	}
}

func validateCredentialProxy(ctx context.Context, data Model, requireKnown bool, diags *diag.Diagnostics) {
	if len(data.CredentialProxy) == 0 {
		return
	}
	if len(data.CredentialProxy) > 1 {
		diags.AddAttributeError(path.Root("credential_proxy"), "Too Many Credential Proxy Blocks", "Set no more than one credential_proxy block.")
		return
	}

	proxy := data.CredentialProxy[0]
	if proxy.Header.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(path.Root("credential_proxy").AtListIndex(0).AtName("header"), "Unknown Credential Proxy Header", "credential_proxy.header must be known before apply.")
		}
		return
	}
	if strings.TrimSpace(proxy.Header.ValueString()) == "" {
		diags.AddAttributeError(path.Root("credential_proxy").AtListIndex(0).AtName("header"), "Missing Credential Proxy Header", "credential_proxy.header must not be empty.")
	}

	targetHosts := normalizedTargetHosts(ctx, proxy.TargetHosts, diags)
	if len(targetHosts) == 0 && (requireKnown || !proxy.TargetHosts.IsUnknown()) {
		diags.AddAttributeError(path.Root("credential_proxy").AtListIndex(0).AtName("target_hosts"), "Missing Credential Proxy Target Hosts", "credential_proxy.target_hosts must include at least one non-empty host.")
	}
}

func validateUUIDAttribute(value types.String, attrPath path.Path, diags *diag.Diagnostics) {
	if !isKnownString(value) {
		return
	}
	if !uuidPattern.MatchString(value.ValueString()) {
		diags.AddAttributeError(attrPath, "Invalid ID", "Value must be a UUID.")
	}
}

func validAbsoluteFilePath(value string) bool {
	return strings.HasPrefix(value, "/") && len(value) > 1 && !strings.HasPrefix(value, "//")
}

func isKnownString(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueString() != ""
}

func isKnownBool(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func stringValueChanged(current types.String, prior types.String) bool {
	if current.IsUnknown() || prior.IsUnknown() {
		return false
	}
	if current.IsNull() && prior.IsNull() {
		return false
	}
	if current.IsNull() != prior.IsNull() {
		return true
	}
	return current.ValueString() != prior.ValueString()
}
