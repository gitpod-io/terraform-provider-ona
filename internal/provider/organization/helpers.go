// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type authenticatedOrganization struct {
	ID        string
	Principal v1.Principal
}

func authenticatedOrganizationForClient(ctx context.Context, client *managementclient.ManagementPlane) (authenticatedOrganization, error) {
	result, err := client.IdentityService().GetAuthenticatedIdentity(ctx, connect.NewRequest(&v1.GetAuthenticatedIdentityRequest{}))
	if err != nil {
		return authenticatedOrganization{}, fmt.Errorf("get authenticated identity: %w", err)
	}
	organizationID := result.Msg.GetOrganizationId()
	if organizationID == "" {
		return authenticatedOrganization{}, fmt.Errorf("authenticated identity did not include an organization ID")
	}
	return authenticatedOrganization{
		ID:        organizationID,
		Principal: result.Msg.GetSubject().GetPrincipal(),
	}, nil
}

func guardStateOrganizationID(diags *diag.Diagnostics, stateID types.String, authenticatedID string, resourceType string) bool {
	if stateID.IsNull() || stateID.IsUnknown() || stateID.ValueString() == "" || stateID.ValueString() == authenticatedID {
		return true
	}
	diags.AddError(
		"Authenticated Organization Changed",
		fmt.Sprintf(
			"%s state belongs to organization %q, but the configured Ona token is authenticated for organization %q. Use a token for the original organization or import a separate resource for the current organization.",
			resourceType,
			stateID.ValueString(),
			authenticatedID,
		),
	)
	return false
}

func timestampRFC3339(value *timestamppb.Timestamp) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(value.AsTime().UTC().Format(time.RFC3339))
}

func timestampString(value *timestamppb.Timestamp) types.String {
	return timestampRFC3339(value)
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

func secretVersionChanged(current types.String, prior types.String) bool {
	return stringValueChanged(current, prior)
}

func ptr[T any](value T) *T {
	return &value
}
