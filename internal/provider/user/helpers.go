// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
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

func listOrganizationMembers(ctx context.Context, client *managementclient.ManagementPlane, organizationID string, filter *v1.ListMembersRequest_Filter) ([]*v1.OrganizationMember, error) {
	var members []*v1.OrganizationMember
	var token string
	seenTokens := make(map[string]struct{})

	for {
		result, err := client.OrganizationService().ListMembers(ctx, connect.NewRequest(&v1.ListMembersRequest{
			Pagination:     &v1.PaginationRequest{PageSize: 100, Token: token},
			OrganizationId: organizationID,
			Filter:         filter,
			Sort: &v1.ListMembersRequest_Sort{
				Field: v1.ListMembersRequest_SORT_FIELD_NAME,
				Order: v1.SortOrder_SORT_ORDER_ASC,
			},
		}))
		if err != nil {
			return nil, fmt.Errorf("list organization members: %w", err)
		}
		if result == nil || result.Msg == nil {
			return nil, fmt.Errorf("list organization members: Ona returned an empty response")
		}
		members = append(members, result.Msg.GetMembers()...)
		nextToken := result.Msg.GetPagination().GetNextToken()
		if nextToken == "" {
			return members, nil
		}
		if err := listutil.NextPageToken(seenTokens, nextToken); err != nil {
			return nil, fmt.Errorf("list organization members: %w", err)
		}
		token = nextToken
	}
}
