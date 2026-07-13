// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package listutil

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/api/go/client"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const DefaultPageSize int32 = 100

// Error returns a list result containing one error diagnostic.
func Error(summary string, err error) list.ListResult {
	var diags diag.Diagnostics
	diags.AddError(summary, err.Error())
	return list.ListResult{Diagnostics: diags}
}

// StringList converts an optional Terraform list of strings into Go values.
func StringList(ctx context.Context, value types.List) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}

	var result []string
	diags.Append(value.ElementsAs(ctx, &result, false)...)
	return result, diags
}

// PushDiagnostics emits conversion errors and reports whether listing can
// continue.
func PushDiagnostics(push func(list.ListResult) bool, diags diag.Diagnostics) bool {
	if !diags.HasError() {
		return true
	}
	return push(list.ListResult{Diagnostics: diags})
}

// HasCapacity reports whether another result may be emitted for a Terraform
// limit. A non-positive limit means unlimited.
func HasCapacity(limit, emitted int64) bool {
	return limit <= 0 || emitted < limit
}

// PageSize returns an API page size that does not request more rows than the
// remaining Terraform result limit. It returns zero after the limit is met.
func PageSize(limit, emitted int64) int32 {
	if limit <= 0 {
		return DefaultPageSize
	}

	remaining := limit - emitted
	if remaining <= 0 {
		return 0
	}
	if remaining < int64(DefaultPageSize) {
		return int32(remaining)
	}
	return DefaultPageSize
}

// AuthenticatedOrganizationID resolves the organization associated with the
// configured provider identity.
func AuthenticatedOrganizationID(ctx context.Context, client *managementclient.ManagementPlane) (string, error) {
	result, err := client.IdentityService().GetAuthenticatedIdentity(ctx, connect.NewRequest(&v1.GetAuthenticatedIdentityRequest{}))
	if err != nil {
		return "", fmt.Errorf("get authenticated identity: %w", err)
	}
	if result.Msg.GetOrganizationId() == "" {
		return "", fmt.Errorf("authenticated identity did not include an organization ID")
	}
	return result.Msg.GetOrganizationId(), nil
}
