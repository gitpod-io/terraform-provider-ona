// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"errors"
	"fmt"

	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/google/uuid"
)

func authenticatedOrganizationID(ctx context.Context, client *managementclient.ManagementPlane) (string, error) {
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, client)
	if err != nil {
		if errors.Is(err, providerdata.ErrEmptyAuthenticatedIdentityResponse) {
			return "", errors.New("get authenticated identity: Ona returned an empty response")
		}
		return "", err
	}
	if _, err := uuid.Parse(organizationID); err != nil {
		return "", fmt.Errorf("authenticated identity included invalid organization ID %q: %w", organizationID, err)
	}
	return organizationID, nil
}
