// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type clientHolder struct {
	client *managementclient.ManagementPlane
}

func (h *clientHolder) configure(req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerdata.Data)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *providerdata.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	h.client = data.Client
}

func (h *clientHolder) requireClient(diags *diag.Diagnostics, action string, resourceType string) bool {
	if h.client != nil {
		return true
	}
	diags.AddError(
		"Ona API Client Is Not Configured",
		fmt.Sprintf("Set the provider token argument or ONA_TOKEN before %s %s resources.", action, resourceType),
	)
	return false
}

type authenticatedOrganization struct {
	ID        string
	Principal v1.Principal
}

func (h *clientHolder) authenticatedOrganization(ctx context.Context) (authenticatedOrganization, error) {
	result, err := h.client.IdentityService().GetAuthenticatedIdentity(ctx, connect.NewRequest(&v1.GetAuthenticatedIdentityRequest{}))
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

func (h *clientHolder) authenticatedOrganizationID(ctx context.Context) (string, error) {
	result, err := h.client.IdentityService().GetAuthenticatedIdentity(ctx, connect.NewRequest(&v1.GetAuthenticatedIdentityRequest{}))
	if err != nil {
		return "", fmt.Errorf("get authenticated identity: %w", err)
	}
	organizationID := result.Msg.GetOrganizationId()
	if organizationID == "" {
		return "", fmt.Errorf("authenticated identity did not include an organization ID")
	}
	return organizationID, nil
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

func preserveString(current types.String, planned types.String) types.String {
	if planned.IsNull() || planned.IsUnknown() {
		return current
	}
	return planned
}

func preserveBool(current types.Bool, planned types.Bool) types.Bool {
	if planned.IsNull() || planned.IsUnknown() {
		return current
	}
	return planned
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

func secretVersionChanged(current types.String, prior types.String) bool {
	return stringValueChanged(current, prior)
}

func ptr[T any](value T) *T {
	return &value
}
