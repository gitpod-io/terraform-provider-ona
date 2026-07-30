// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package providerdata

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

type Data struct {
	Client     *managementclient.ManagementPlane
	APIBaseURL string
	UserAgent  string
}

// ErrEmptyAuthenticatedIdentityResponse reports a successful identity call without a response message.
var ErrEmptyAuthenticatedIdentityResponse = errors.New("API returned an empty response")

// ResourceClient extracts the management client supplied to a resource and
// preserves current when provider data is absent or invalid.
func ResourceClient(providerData any, current *managementclient.ManagementPlane, diags *diag.Diagnostics) *managementclient.ManagementPlane {
	data := configureData(providerData, "Resource", diags)
	if data == nil {
		return current
	}
	return data.Client
}

// DataSourceClient extracts the management client supplied to a data source
// and preserves current when provider data is absent or invalid.
func DataSourceClient(providerData any, current *managementclient.ManagementPlane, diags *diag.Diagnostics) *managementclient.ManagementPlane {
	data := configureData(providerData, "Data Source", diags)
	if data == nil {
		return current
	}
	return data.Client
}

// EphemeralResourceData extracts the complete provider data supplied to an
// ephemeral resource and preserves current when provider data is absent or invalid.
func EphemeralResourceData(providerData any, current *Data, diags *diag.Diagnostics) *Data {
	data := configureData(providerData, "Ephemeral Resource", diags)
	if data == nil {
		return current
	}
	return data
}

func configureData(providerData any, component string, diags *diag.Diagnostics) *Data {
	if providerData == nil {
		return nil
	}
	data, ok := providerData.(*Data)
	if !ok {
		diags.AddError(
			"Unexpected "+component+" Configure Type",
			fmt.Sprintf("Expected *providerdata.Data, got: %T. Please report this issue to the provider developers.", providerData),
		)
		return nil
	}
	return data
}

// RequireResourceClient reports whether a resource has a configured client.
func RequireResourceClient(client *managementclient.ManagementPlane, diags *diag.Diagnostics, action, resourceType string) bool {
	return requireClient(client, diags, fmt.Sprintf("Set the provider token argument or ONA_TOKEN before %s %s resources.", action, resourceType))
}

// RequireDataSourceClient reports whether a data source has a configured client.
func RequireDataSourceClient(client *managementclient.ManagementPlane, diags *diag.Diagnostics, dataSourceType string) bool {
	return requireClient(client, diags, fmt.Sprintf("Set the provider token argument or ONA_TOKEN before reading %s data sources.", dataSourceType))
}

// RequireEphemeralResourceClient reports whether an ephemeral resource has a configured client.
func RequireEphemeralResourceClient(client *managementclient.ManagementPlane, diags *diag.Diagnostics, resourceType string) bool {
	return requireClient(client, diags, fmt.Sprintf("Set the provider token argument or ONA_TOKEN before opening %s ephemeral resources.", resourceType))
}

func requireClient(client *managementclient.ManagementPlane, diags *diag.Diagnostics, detail string) bool {
	if client != nil {
		return true
	}
	diags.AddError("Ona API Client Is Not Configured", detail)
	return false
}

// AuthenticatedOrganizationID resolves the organization associated with the configured identity.
func AuthenticatedOrganizationID(ctx context.Context, client *managementclient.ManagementPlane) (string, error) {
	result, err := client.IdentityService().GetAuthenticatedIdentity(ctx, connect.NewRequest(&v1.GetAuthenticatedIdentityRequest{}))
	if err != nil {
		return "", fmt.Errorf("get authenticated identity: %w", err)
	}
	if result == nil || result.Msg == nil {
		return "", fmt.Errorf("get authenticated identity: %w", ErrEmptyAuthenticatedIdentityResponse)
	}
	organizationID := result.Msg.GetOrganizationId()
	if organizationID == "" {
		return "", errors.New("authenticated identity did not include an organization ID")
	}
	return organizationID, nil
}
