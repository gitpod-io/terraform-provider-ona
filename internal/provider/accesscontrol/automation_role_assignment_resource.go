// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/tfvalue"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const automationRoleAssignmentPageSize int32 = 100

var _ resource.Resource = &AutomationRoleAssignmentResource{}
var _ resource.ResourceWithConfigure = &AutomationRoleAssignmentResource{}
var _ resource.ResourceWithIdentity = &AutomationRoleAssignmentResource{}
var _ resource.ResourceWithImportState = &AutomationRoleAssignmentResource{}
var _ resource.ResourceWithValidateConfig = &AutomationRoleAssignmentResource{}

func NewAutomationRoleAssignmentResource() resource.Resource {
	return &AutomationRoleAssignmentResource{}
}

type AutomationRoleAssignmentResource struct {
	client *managementclient.ManagementPlane
}

type AutomationRoleAssignmentModel struct {
	ID           types.String `tfsdk:"id"`
	AutomationID types.String `tfsdk:"automation_id"`
	GroupID      types.String `tfsdk:"group_id"`
	Role         types.String `tfsdk:"role"`
}

var automationRoleToAPI = map[string]v1.ResourceRole{
	"admin":    v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_ADMIN,
	"executor": v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_EXECUTOR,
	"viewer":   v1.ResourceRole_RESOURCE_ROLE_WORKFLOW_VIEWER,
}

var apiToAutomationRole = func() map[v1.ResourceRole]string {
	result := make(map[v1.ResourceRole]string, len(automationRoleToAPI))
	for role, apiRole := range automationRoleToAPI {
		result[apiRole] = role
	}
	return result
}()

func (r *AutomationRoleAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_automation_role_assignment"
}

func (r *AutomationRoleAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona role assignment granting a group access to one Automation.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role assignment ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"automation_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Automation ID receiving the group access. Changing this value replaces the assignment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"group_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group ID receiving access to the Automation. Changing this value replaces the assignment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Automation role. Supported values are `viewer`, `executor`, and `admin`. Changing this value replaces the assignment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *AutomationRoleAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
}

func (r *AutomationRoleAssignmentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data AutomationRoleAssignmentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateAutomationRole(data.Role, &resp.Diagnostics)
}

func (r *AutomationRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AutomationRoleAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", "ona_automation_role_assignment") {
		return
	}

	apiRole, ok := automationRoleToAPI[data.Role.ValueString()]
	if !ok {
		addInvalidAutomationRoleDiagnostic(path.Root("role"), data.Role.ValueString(), &resp.Diagnostics)
		return
	}

	existing, err := r.findAssignment(ctx, data.AutomationID.ValueString(), data.GroupID.ValueString(), data.Role.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Automation Role Assignment", "checking for an existing Automation role assignment", err)
		return
	}
	if existing != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Ona Automation Role Assignment",
			fmt.Sprintf("An Automation role assignment for Automation %q, group %q, and role %q already exists with ID %q. Import the existing assignment instead of creating a duplicate.", data.AutomationID.ValueString(), data.GroupID.ValueString(), data.Role.ValueString(), existing.GetId()),
		)
		return
	}

	result, err := r.client.GroupService().CreateRoleAssignment(ctx, connect.NewRequest(&v1.CreateRoleAssignmentRequest{
		GroupId:      data.GroupID.ValueString(),
		ResourceType: v1.ResourceType_RESOURCE_TYPE_WORKFLOW,
		ResourceId:   data.AutomationID.ValueString(),
		ResourceRole: apiRole,
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Automation Role Assignment", "creating the Automation role assignment", err)
		return
	}
	if result == nil || result.Msg == nil || result.Msg.GetAssignment() == nil {
		r.recoverMalformedCreate(ctx, &data, "The Ona API returned an empty role assignment.", resp)
		return
	}

	assignment := result.Msg.GetAssignment()
	if assignment.GetId() == "" {
		r.recoverMalformedCreate(ctx, &data, "The Ona API returned a role assignment without an ID.", resp)
		return
	}

	data.ID = types.StringValue(assignment.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	observed, err := automationRoleAssignmentModelFromAPI(assignment)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Automation Role Assignment", fmt.Sprintf("The Ona API returned a malformed role assignment: %v.", err))
		return
	}
	if !matchesAutomationRoleAssignment(assignment, data.AutomationID.ValueString(), data.GroupID.ValueString(), apiRole) {
		resp.Diagnostics.AddError("Unable to Create Ona Automation Role Assignment", "The Ona API returned a role assignment that does not match the requested Automation, group, and role.")
		return
	}

	resp.Diagnostics.Append(resp.Identity.Set(ctx, AutomationRoleAssignmentIdentityModel{
		AutomationID: observed.AutomationID,
		GroupID:      observed.GroupID,
		Role:         observed.Role,
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &observed)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *AutomationRoleAssignmentResource) recoverMalformedCreate(ctx context.Context, data *AutomationRoleAssignmentModel, malformedResponseDetail string, resp *resource.CreateResponse) {
	assignment, err := r.findAssignment(ctx, data.AutomationID.ValueString(), data.GroupID.ValueString(), data.Role.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Ona Automation Role Assignment",
			fmt.Sprintf("%s The provider could not recover the created assignment: %v.", malformedResponseDetail, err),
		)
		return
	}
	if assignment == nil {
		resp.Diagnostics.AddError(
			"Unable to Create Ona Automation Role Assignment",
			malformedResponseDetail+" The provider could not find the created assignment to recover its ID.",
		)
		return
	}

	data.ID = types.StringValue(assignment.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, AutomationRoleAssignmentIdentityModel{
		AutomationID: data.AutomationID,
		GroupID:      data.GroupID,
		Role:         data.Role,
	})...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddError(
		"Unable to Create Ona Automation Role Assignment",
		fmt.Sprintf("%s Terraform recovered assignment ID %q in state so the assignment can be safely retried or destroyed.", malformedResponseDetail, assignment.GetId()),
	)
}

func (r *AutomationRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AutomationRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", "ona_automation_role_assignment") {
		return
	}

	assignment, err := r.findAssignment(ctx, data.AutomationID.ValueString(), data.GroupID.ValueString(), data.Role.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Automation Role Assignment", "reading the Automation role assignment", err)
		return
	}
	if assignment == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	observed, err := automationRoleAssignmentModelFromAPI(assignment)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Automation Role Assignment", fmt.Sprintf("The Ona API returned a malformed role assignment: %v.", err))
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, AutomationRoleAssignmentIdentityModel{
		AutomationID: observed.AutomationID,
		GroupID:      observed.GroupID,
		Role:         observed.Role,
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &observed)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *AutomationRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unable to Update Ona Automation Role Assignment", "Automation role assignments are immutable. Change automation_id, group_id, or role by replacing the resource.")
}

func (r *AutomationRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AutomationRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", "ona_automation_role_assignment") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Unable to Delete Ona Automation Role Assignment", "Terraform state does not contain the role assignment ID required for deletion. Refresh or re-import the resource before retrying.")
		return
	}

	_, err := r.client.GroupService().DeleteRoleAssignment(ctx, connect.NewRequest(&v1.DeleteRoleAssignmentRequest{
		AssignmentId: data.ID.ValueString(),
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Automation Role Assignment", "deleting the Automation role assignment", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *AutomationRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		var identity AutomationRoleAssignmentIdentityModel
		resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		validateAutomationRoleAssignmentIdentity(identity, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		tfvalue.SetImportString(ctx, resp, "automation_id", identity.AutomationID.ValueString())
		if resp.Diagnostics.HasError() {
			return
		}
		tfvalue.SetImportString(ctx, resp, "group_id", identity.GroupID.ValueString())
		if resp.Diagnostics.HasError() {
			return
		}
		tfvalue.SetImportString(ctx, resp, "role", identity.Role.ValueString())
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		return
	}

	parts, diags := splitAutomationRoleAssignmentImportID(req.ID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tfvalue.SetImportString(ctx, resp, "automation_id", parts[0])
	if resp.Diagnostics.HasError() {
		return
	}
	tfvalue.SetImportString(ctx, resp, "group_id", parts[1])
	if resp.Diagnostics.HasError() {
		return
	}
	tfvalue.SetImportString(ctx, resp, "role", parts[2])
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *AutomationRoleAssignmentResource) findAssignment(ctx context.Context, automationID string, groupID string, role string) (*v1.RoleAssignment, error) {
	apiRole, ok := automationRoleToAPI[role]
	if !ok {
		return nil, fmt.Errorf("unsupported Automation role %q", role)
	}

	var token string
	var match *v1.RoleAssignment
	seenTokens := make(map[string]struct{})
	for {
		result, err := r.client.GroupService().ListRoleAssignments(ctx, connect.NewRequest(&v1.ListRoleAssignmentsRequest{
			Pagination: &v1.PaginationRequest{PageSize: automationRoleAssignmentPageSize, Token: token},
			Filter: &v1.ListRoleAssignmentsRequest_Filter{
				GroupId:       groupID,
				ResourceTypes: []v1.ResourceType{v1.ResourceType_RESOURCE_TYPE_WORKFLOW},
				ResourceId:    automationID,
				ResourceRoles: []v1.ResourceRole{apiRole},
			},
		}))
		if err != nil {
			return nil, fmt.Errorf("list role assignments: %w", err)
		}
		if result == nil || result.Msg == nil {
			return nil, fmt.Errorf("list role assignments: Ona API returned an empty response")
		}
		for _, assignment := range result.Msg.GetAssignments() {
			if !matchesAutomationRoleAssignment(assignment, automationID, groupID, apiRole) {
				continue
			}
			if assignment.GetId() == "" {
				return nil, fmt.Errorf("list role assignments: matching assignment has an empty ID")
			}
			if match != nil {
				return nil, fmt.Errorf("list role assignments: multiple assignments match Automation %q, group %q, and role %q: IDs %q and %q", automationID, groupID, role, match.GetId(), assignment.GetId())
			}
			match = assignment
		}

		nextToken := result.Msg.GetPagination().GetNextToken()
		if nextToken == "" {
			return match, nil
		}
		if _, ok := seenTokens[nextToken]; ok {
			return nil, fmt.Errorf("list role assignments: Ona API returned repeated pagination token %q", nextToken)
		}
		seenTokens[nextToken] = struct{}{}
		token = nextToken
	}
}

func automationRoleAssignmentModelFromAPI(assignment *v1.RoleAssignment) (AutomationRoleAssignmentModel, error) {
	if assignment == nil {
		return AutomationRoleAssignmentModel{}, fmt.Errorf("role assignment is missing")
	}
	if assignment.GetId() == "" {
		return AutomationRoleAssignmentModel{}, fmt.Errorf("role assignment ID is empty")
	}
	if assignment.GetGroupId() == "" {
		return AutomationRoleAssignmentModel{}, fmt.Errorf("role assignment group ID is empty")
	}
	if assignment.GetResourceId() == "" {
		return AutomationRoleAssignmentModel{}, fmt.Errorf("role assignment Automation ID is empty")
	}
	if assignment.GetResourceType() != v1.ResourceType_RESOURCE_TYPE_WORKFLOW {
		return AutomationRoleAssignmentModel{}, fmt.Errorf("role assignment resource type is %s, expected RESOURCE_TYPE_WORKFLOW", assignment.GetResourceType())
	}
	role, ok := apiToAutomationRole[assignment.GetResourceRole()]
	if !ok {
		return AutomationRoleAssignmentModel{}, fmt.Errorf("role assignment has unsupported Automation role %s", assignment.GetResourceRole())
	}

	return AutomationRoleAssignmentModel{
		ID:           types.StringValue(assignment.GetId()),
		AutomationID: types.StringValue(assignment.GetResourceId()),
		GroupID:      types.StringValue(assignment.GetGroupId()),
		Role:         types.StringValue(role),
	}, nil
}

func matchesAutomationRoleAssignment(assignment *v1.RoleAssignment, automationID string, groupID string, role v1.ResourceRole) bool {
	return assignment != nil &&
		assignment.GetGroupId() == groupID &&
		assignment.GetResourceType() == v1.ResourceType_RESOURCE_TYPE_WORKFLOW &&
		assignment.GetResourceId() == automationID &&
		assignment.GetResourceRole() == role
}

func splitAutomationRoleAssignmentImportID(id string) ([]string, diag.Diagnostics) {
	parts, diags := tfvalue.SplitImportID(id, 3, "automation_id/group_id/role")
	if diags.HasError() {
		return nil, diags
	}
	validateAutomationRole(types.StringValue(parts[2]), &diags)
	if diags.HasError() {
		return nil, diags
	}
	return parts, diags
}

func validateAutomationRoleAssignmentIdentity(identity AutomationRoleAssignmentIdentityModel, diags *diag.Diagnostics) {
	validateAutomationRoleAssignmentIdentityString(path.Root("automation_id"), "automation_id", identity.AutomationID, diags)
	validateAutomationRoleAssignmentIdentityString(path.Root("group_id"), "group_id", identity.GroupID, diags)
	validateAutomationRoleAssignmentIdentityString(path.Root("role"), "role", identity.Role, diags)
	if diags.HasError() {
		return
	}
	validateAutomationRole(identity.Role, diags)
}

func validateAutomationRoleAssignmentIdentityString(attrPath path.Path, name string, value types.String, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() || strings.TrimSpace(value.ValueString()) == "" {
		diags.AddAttributeError(attrPath, "Invalid Automation Role Assignment Identity", fmt.Sprintf("%s must be a non-empty string.", name))
	}
}

func validateAutomationRole(role types.String, diags *diag.Diagnostics) {
	if role.IsNull() || role.IsUnknown() {
		return
	}
	if _, ok := automationRoleToAPI[role.ValueString()]; !ok {
		addInvalidAutomationRoleDiagnostic(path.Root("role"), role.ValueString(), diags)
	}
}

func addInvalidAutomationRoleDiagnostic(attrPath path.Path, role string, diags *diag.Diagnostics) {
	diags.AddAttributeError(
		attrPath,
		"Unsupported Automation Role",
		fmt.Sprintf("Unsupported Automation role %q. Supported values are: %s.", role, strings.Join(supportedAutomationRoles(), ", ")),
	)
}

func supportedAutomationRoles() []string {
	roles := make([]string, 0, len(automationRoleToAPI))
	for role := range automationRoleToAPI {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}
