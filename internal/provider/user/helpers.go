// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

type clientHolder struct {
	client *managementclient.ManagementPlane
}

func (h *clientHolder) configure(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerdata.Data)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *providerdata.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	h.client = data.Client
}

func (h *clientHolder) requireClient(diags *diag.Diagnostics, dataSourceType string) bool {
	if h.client != nil {
		return true
	}
	diags.AddError(
		"Ona API Client Is Not Configured",
		fmt.Sprintf("Set the provider token argument or ONA_TOKEN before reading %s data sources.", dataSourceType),
	)
	return false
}

func (h *clientHolder) authenticatedOrganizationID(ctx context.Context) (string, error) {
	result, err := h.client.IdentityService().GetAuthenticatedIdentity(ctx, connect.NewRequest(&v1.GetAuthenticatedIdentityRequest{}))
	if err != nil {
		return "", fmt.Errorf("get authenticated identity: %w", err)
	}
	if result == nil || result.Msg == nil {
		return "", fmt.Errorf("get authenticated identity: Ona returned an empty response")
	}
	organizationID := result.Msg.GetOrganizationId()
	if organizationID == "" {
		return "", fmt.Errorf("authenticated identity did not include an organization ID")
	}
	if _, err := uuid.Parse(organizationID); err != nil {
		return "", fmt.Errorf("authenticated identity included invalid organization ID %q: %w", organizationID, err)
	}
	return organizationID, nil
}
