// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"
	"fmt"
	"sort"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &OrganizationRoleAssignmentResource{}

func NewOrganizationRoleAssignmentListResource() list.ListResource {
	return &OrganizationRoleAssignmentResource{}
}

type organizationRoleAssignmentListModel struct {
	GroupID        types.String `tfsdk:"group_id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Role           types.String `tfsdk:"role"`
}

func (r *OrganizationRoleAssignmentResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{MarkdownDescription: "Lists supported Ona organization role assignments.", Attributes: map[string]listschema.Attribute{
		"group_id":        listschema.StringAttribute{Optional: true, MarkdownDescription: "Group ID receiving the role."},
		"organization_id": listschema.StringAttribute{Optional: true, MarkdownDescription: "Organization ID to query. Defaults to the authenticated organization."},
		"role":            listschema.StringAttribute{Optional: true, MarkdownDescription: "Supported organization role to include."},
	}}
}

func (r *OrganizationRoleAssignmentResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_organization_role_assignment resources")))
			return
		}
		var data organizationRoleAssignmentListModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &data)) {
			return
		}
		organizationID := data.OrganizationID.ValueString()
		if organizationID == "" {
			var err error
			organizationID, err = listutil.AuthenticatedOrganizationID(ctx, r.client)
			if err != nil {
				push(listutil.Error("Unable to Determine Organization", err))
				return
			}
		}
		var roles []v1.ResourceRole
		if role := data.Role.ValueString(); role != "" {
			apiRole, ok := roleToAPI[role]
			if !ok {
				push(listutil.Error("Unsupported Organization Role", fmt.Errorf("unsupported organization role %q", role)))
				return
			}
			roles = []v1.ResourceRole{apiRole}
		}
		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			result, err := r.client.GroupService().ListRoleAssignments(ctx, connect.NewRequest(&v1.ListRoleAssignmentsRequest{Pagination: &v1.PaginationRequest{PageSize: listutil.PageSize(req.Limit, emitted), Token: token}, Filter: &v1.ListRoleAssignmentsRequest_Filter{GroupId: data.GroupID.ValueString(), ResourceId: organizationID, ResourceTypes: []v1.ResourceType{v1.ResourceType_RESOURCE_TYPE_ORGANIZATION}, ResourceRoles: roles}}))
			if err != nil {
				push(listutil.Error("Unable to List Ona Organization Role Assignments", fmt.Errorf("list role assignments: %w", err)))
				return
			}
			assignments := result.Msg.GetAssignments()
			sort.SliceStable(assignments, func(i, j int) bool { return assignments[i].GetId() < assignments[j].GetId() })
			for _, assignment := range assignments {
				role, ok := apiToRole[assignment.GetResourceRole()]
				if !ok || assignment.GetResourceType() != v1.ResourceType_RESOURCE_TYPE_ORGANIZATION {
					continue
				}
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = assignment.GetGroupId() + "/" + role
				item.Diagnostics.Append(item.Identity.Set(ctx, OrganizationRoleAssignmentIdentityModel{GroupID: types.StringValue(assignment.GetGroupId()), OrganizationID: types.StringValue(assignment.GetResourceId()), Role: types.StringValue(role)})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					var model OrganizationRoleAssignmentModel
					populateOrganizationRoleAssignmentModel(&model, assignment)
					item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
				}
				if !push(item) || item.Diagnostics.HasError() {
					return
				}
				emitted++
			}
			token = result.Msg.GetPagination().GetNextToken()
			if token == "" {
				return
			}
		}
	}
}
