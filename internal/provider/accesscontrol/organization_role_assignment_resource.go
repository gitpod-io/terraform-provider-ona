// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &OrganizationRoleAssignmentResource{}
var _ resource.ResourceWithConfigure = &OrganizationRoleAssignmentResource{}
var _ resource.ResourceWithImportState = &OrganizationRoleAssignmentResource{}
var _ resource.ResourceWithValidateConfig = &OrganizationRoleAssignmentResource{}

func NewOrganizationRoleAssignmentResource() resource.Resource {
	return &OrganizationRoleAssignmentResource{}
}

type OrganizationRoleAssignmentResource struct {
	clientHolder
}

type OrganizationRoleAssignmentModel struct {
	ID      types.String `tfsdk:"id"`
	GroupID types.String `tfsdk:"group_id"`
	Role    types.String `tfsdk:"role"`
}

var roleToAPI = map[string]v1.ResourceRole{
	"organization_admin":  v1.ResourceRole_RESOURCE_ROLE_ORG_ADMIN,
	"runners_admin":       v1.ResourceRole_RESOURCE_ROLE_ORG_RUNNERS_ADMIN,
	"projects_admin":      v1.ResourceRole_RESOURCE_ROLE_ORG_PROJECTS_ADMIN,
	"automations_admin":   v1.ResourceRole_RESOURCE_ROLE_ORG_AUTOMATIONS_ADMIN,
	"groups_admin":        v1.ResourceRole_RESOURCE_ROLE_ORG_GROUPS_ADMIN,
	"environments_reader": v1.ResourceRole_RESOURCE_ROLE_ORG_ENVIRONMENTS_READER,
	"insights_viewer":     v1.ResourceRole_RESOURCE_ROLE_ORG_INSIGHTS_VIEWER,
	"audit_log_reader":    v1.ResourceRole_RESOURCE_ROLE_ORG_AUDIT_LOG_READER,
	"billing_viewer":      v1.ResourceRole_RESOURCE_ROLE_ORG_BILLING_VIEWER,
}

var apiToRole = func() map[v1.ResourceRole]string {
	result := make(map[v1.ResourceRole]string, len(roleToAPI))
	for role, apiRole := range roleToAPI {
		result[apiRole] = role
	}
	return result
}()

func (r *OrganizationRoleAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_role_assignment"
}

func (r *OrganizationRoleAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona organization-level role assignment for a group. The assignment targets the organization associated with the authenticated provider token.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role assignment ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group ID receiving the organization role. Changing this value replaces the assignment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Organization role. Supported values are `organization_admin`, `runners_admin`, `projects_admin`, `automations_admin`, `groups_admin`, `environments_reader`, `insights_viewer`, `audit_log_reader`, and `billing_viewer`. Changing this value replaces the assignment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *OrganizationRoleAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configure(req, resp)
}

func (r *OrganizationRoleAssignmentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data OrganizationRoleAssignmentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateOrganizationRole(data.Role, &resp.Diagnostics)
}

func (r *OrganizationRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationRoleAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "creating", "ona_organization_role_assignment") {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}

	apiRole, ok := roleToAPI[data.Role.ValueString()]
	if !ok {
		addInvalidRoleDiagnostic(path.Root("role"), data.Role.ValueString(), &resp.Diagnostics)
		return
	}

	result, err := r.client.GroupService().CreateRoleAssignment(ctx, connect.NewRequest(&v1.CreateRoleAssignmentRequest{
		GroupId:      data.GroupID.ValueString(),
		ResourceType: v1.ResourceType_RESOURCE_TYPE_ORGANIZATION,
		ResourceId:   organizationID,
		ResourceRole: apiRole,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Organization Role Assignment", err.Error())
		return
	}
	if result.Msg.GetAssignment() == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Organization Role Assignment", "The Ona API returned an empty role assignment.")
		return
	}

	populateOrganizationRoleAssignmentModel(&data, result.Msg.GetAssignment())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "reading", "ona_organization_role_assignment") {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}

	assignment, err := r.findAssignment(ctx, data.GroupID.ValueString(), organizationID, data.Role.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Organization Role Assignment", err.Error())
		return
	}
	if assignment == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data = OrganizationRoleAssignmentModel{}
	populateOrganizationRoleAssignmentModel(&data, assignment)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unable to Update Ona Organization Role Assignment", "Organization role assignments are immutable. Change group_id or role by replacing the resource.")
}

func (r *OrganizationRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrganizationRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "deleting", "ona_organization_role_assignment") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.GroupService().DeleteRoleAssignment(ctx, connect.NewRequest(&v1.DeleteRoleAssignmentRequest{
		AssignmentId: data.ID.ValueString(),
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		resp.Diagnostics.AddError("Unable to Delete Ona Organization Role Assignment", err.Error())
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *OrganizationRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid Import ID", "Use group_id/role to import an Ona organization role assignment.")
		return
	}
	if _, ok := roleToAPI[parts[1]]; !ok {
		addInvalidRoleDiagnostic(path.Root("role"), parts[1], &resp.Diagnostics)
		return
	}
	setImportString(ctx, resp, "group_id", parts[0])
	setImportString(ctx, resp, "role", parts[1])
}

func (r *OrganizationRoleAssignmentResource) findAssignment(ctx context.Context, groupID string, organizationID string, role string) (*v1.RoleAssignment, error) {
	apiRole, ok := roleToAPI[role]
	if !ok {
		return nil, fmt.Errorf("unsupported organization role %q", role)
	}
	result, err := r.client.GroupService().ListRoleAssignments(ctx, connect.NewRequest(&v1.ListRoleAssignmentsRequest{
		Filter: &v1.ListRoleAssignmentsRequest_Filter{
			GroupId:       groupID,
			ResourceTypes: []v1.ResourceType{v1.ResourceType_RESOURCE_TYPE_ORGANIZATION},
			ResourceId:    organizationID,
			ResourceRoles: []v1.ResourceRole{apiRole},
		},
	}))
	if err != nil {
		return nil, fmt.Errorf("list role assignments: %w", err)
	}
	for _, assignment := range result.Msg.GetAssignments() {
		if assignment.GetGroupId() == groupID &&
			assignment.GetResourceType() == v1.ResourceType_RESOURCE_TYPE_ORGANIZATION &&
			assignment.GetResourceId() == organizationID &&
			assignment.GetResourceRole() == apiRole {
			return assignment, nil
		}
	}
	return nil, nil
}

func populateOrganizationRoleAssignmentModel(data *OrganizationRoleAssignmentModel, assignment *v1.RoleAssignment) {
	role := apiToRole[assignment.GetResourceRole()]
	data.ID = types.StringValue(assignment.GetId())
	data.GroupID = types.StringValue(assignment.GetGroupId())
	data.Role = types.StringValue(role)
}

func validateOrganizationRole(role types.String, diags *diag.Diagnostics) {
	if role.IsNull() || role.IsUnknown() {
		return
	}
	if _, ok := roleToAPI[role.ValueString()]; !ok {
		addInvalidRoleDiagnostic(path.Root("role"), role.ValueString(), diags)
	}
}

func addInvalidRoleDiagnostic(attrPath path.Path, role string, diags *diag.Diagnostics) {
	diags.AddAttributeError(
		attrPath,
		"Unsupported Organization Role",
		fmt.Sprintf("Unsupported organization role %q. Supported values are: %s.", role, strings.Join(supportedOrganizationRoles(), ", ")),
	)
}

func supportedOrganizationRoles() []string {
	roles := make([]string, 0, len(roleToAPI))
	for role := range roleToAPI {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}
