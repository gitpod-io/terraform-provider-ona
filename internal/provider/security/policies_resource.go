// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package security

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ resource.Resource = &PolicyResource{}
var _ resource.ResourceWithConfigure = &PolicyResource{}
var _ resource.ResourceWithImportState = &PolicyResource{}
var _ resource.ResourceWithValidateConfig = &PolicyResource{}

func NewPolicyResource() resource.Resource {
	return &PolicyResource{}
}

type PolicyResource struct {
	client *managementclient.ManagementPlane
}

type PolicyModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	Spec           *SpecModel   `tfsdk:"spec"`
}

type SpecModel struct {
	Ports        *PortPolicyModel        `tfsdk:"ports"`
	Executables  *ExecutablePolicyModel  `tfsdk:"executables"`
	Files        *FilePolicyModel        `tfsdk:"files"`
	BlockDevices *BlockDevicePolicyModel `tfsdk:"block_devices"`
	Data         *DataPolicyModel        `tfsdk:"data"`
}

type PortPolicyModel struct {
	DefaultEffect types.String    `tfsdk:"default_effect"`
	Rules         []PortRuleModel `tfsdk:"rule"`
}

type PortRuleModel struct {
	RangeFrom types.Int64  `tfsdk:"range_from"`
	RangeTo   types.Int64  `tfsdk:"range_to"`
	Effect    types.String `tfsdk:"effect"`
}

type ExecutablePolicyModel struct {
	DefaultEffect types.String          `tfsdk:"default_effect"`
	Rules         []ExecutableRuleModel `tfsdk:"rule"`
}

type ExecutableRuleModel struct {
	Path   types.String `tfsdk:"path"`
	Effect types.String `tfsdk:"effect"`
}

type FilePolicyModel struct {
	DefaultEffect  types.String    `tfsdk:"default_effect"`
	DefaultActions types.Set       `tfsdk:"default_actions"`
	Rules          []FileRuleModel `tfsdk:"rule"`
}

type FileRuleModel struct {
	Path    types.String `tfsdk:"path"`
	Actions types.Set    `tfsdk:"actions"`
	Effect  types.String `tfsdk:"effect"`
}

type BlockDevicePolicyModel struct {
	DefaultEffect types.String `tfsdk:"default_effect"`
}

type DataPolicyModel struct {
	DefaultEffect types.String    `tfsdk:"default_effect"`
	Rules         []DataRuleModel `tfsdk:"rule"`
}

type DataRuleModel struct {
	Source      *DataSourceModel      `tfsdk:"source"`
	Destination *DataDestinationModel `tfsdk:"destination"`
	Effect      types.String          `tfsdk:"effect"`
}

type DataSourceModel struct {
	File        types.String `tfsdk:"file"`
	Integration types.String `tfsdk:"integration"`
	Selector    types.String `tfsdk:"selector"`
}

type DataDestinationModel struct {
	Host types.String `tfsdk:"host"`
}

const (
	effectAllow = "allow"
	effectBlock = "block"
	effectAudit = "audit"
)

func (r *PolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_policy"
}

func (r *PolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = policyResourceSchema()
}

func policyResourceSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Ona security policy for environment runtime controls. Attach the resulting policy through organization policy settings to make it the default for new environments.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Security policy ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Organization ID that owns the security policy. Changing this value replaces the policy.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Security policy name shown in Ona. Must be between 1 and 80 characters.",
			},
			"created_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the security policy was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the security policy was last updated.",
			},
		},
		Blocks: map[string]resourceschema.Block{
			"spec": specBlock(),
		},
	}
}

func specBlock() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		MarkdownDescription: "Runtime security controls enforced for environments using this policy. Configure one or more policy sections depending on what the policy should control.",
		Blocks: map[string]resourceschema.Block{
			"ports":         portPolicyBlock(),
			"executables":   executablePolicyBlock(),
			"files":         filePolicyBlock(),
			"block_devices": blockDevicePolicyBlock(),
			"data":          dataPolicyBlock(),
		},
	}
}

func portPolicyBlock() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		MarkdownDescription: "Port access policy. Rules match inclusive TCP/UDP port ranges from 0 through 65535.",
		Attributes: map[string]resourceschema.Attribute{
			"default_effect": effectAttribute("Default port access effect."),
		},
		Blocks: map[string]resourceschema.Block{
			"rule": resourceschema.ListNestedBlock{
				MarkdownDescription: "Port range rule.",
				NestedObject: resourceschema.NestedBlockObject{
					Attributes: map[string]resourceschema.Attribute{
						"range_from": resourceschema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "First port in the inclusive range. Must be between 0 and 65535.",
						},
						"range_to": resourceschema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "Last port in the inclusive range. Must be between `range_from` and 65535.",
						},
						"effect": effectAttribute("Effect for this port range."),
					},
				},
			},
		},
	}
}

func executablePolicyBlock() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		MarkdownDescription: "Executable access policy. Rules match executable paths inside the environment.",
		Attributes: map[string]resourceschema.Attribute{
			"default_effect": effectAttribute("Default executable access effect."),
		},
		Blocks: map[string]resourceschema.Block{
			"rule": resourceschema.ListNestedBlock{
				MarkdownDescription: "Executable path rule.",
				NestedObject: resourceschema.NestedBlockObject{
					Attributes: map[string]resourceschema.Attribute{
						"path": resourceschema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Executable path inside the environment.",
						},
						"effect": effectAttribute("Effect for this executable path."),
					},
				},
			},
		},
	}
}

func filePolicyBlock() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		MarkdownDescription: "File access policy. Rules match file paths inside the environment and can control read and write actions separately.",
		Attributes: map[string]resourceschema.Attribute{
			"default_effect": effectAttribute("Default file access effect."),
			"default_actions": resourceschema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Actions applied to file rules that omit actions. Supported values are `read` and `write`; omit to use the API default.",
			},
		},
		Blocks: map[string]resourceschema.Block{
			"rule": resourceschema.ListNestedBlock{
				MarkdownDescription: "File path rule.",
				NestedObject: resourceschema.NestedBlockObject{
					Attributes: map[string]resourceschema.Attribute{
						"path": resourceschema.StringAttribute{
							Required:            true,
							MarkdownDescription: "File path inside the environment.",
						},
						"actions": resourceschema.SetAttribute{
							Optional:            true,
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "File actions controlled by this rule. Supported values are `read` and `write`; omit to use `default_actions`.",
						},
						"effect": effectAttribute("Effect for this file path."),
					},
				},
			},
		},
	}
}

func blockDevicePolicyBlock() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		MarkdownDescription: "Block device access policy for environment runtime controls.",
		Attributes: map[string]resourceschema.Attribute{
			"default_effect": effectAttribute("Default block device access effect."),
		},
	}
}

func dataPolicyBlock() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		MarkdownDescription: "Data flow policy. Rules describe allowed or blocked movement from a source to a destination.",
		Attributes: map[string]resourceschema.Attribute{
			"default_effect": effectAttribute("Default data flow effect."),
		},
		Blocks: map[string]resourceschema.Block{
			"rule": resourceschema.ListNestedBlock{
				MarkdownDescription: "Data flow rule.",
				NestedObject: resourceschema.NestedBlockObject{
					Attributes: map[string]resourceschema.Attribute{
						"effect": effectAttribute("Effect for this data flow."),
					},
					Blocks: map[string]resourceschema.Block{
						"source": resourceschema.SingleNestedBlock{
							MarkdownDescription: "Data source. Exactly one of `file` or `integration` must be set.",
							Attributes: map[string]resourceschema.Attribute{
								"file": resourceschema.StringAttribute{
									Optional:            true,
									MarkdownDescription: "Source file path.",
								},
								"integration": resourceschema.StringAttribute{
									Optional:            true,
									MarkdownDescription: "Source integration ID.",
								},
								"selector": resourceschema.StringAttribute{
									Optional:            true,
									MarkdownDescription: "Source-dependent selector for narrowing what data within the source is matched.",
								},
							},
						},
						"destination": resourceschema.SingleNestedBlock{
							MarkdownDescription: "Data destination.",
							Attributes: map[string]resourceschema.Attribute{
								"host": resourceschema.StringAttribute{
									Required:            true,
									MarkdownDescription: "Destination host, domain, service endpoint, or app-owned host.",
								},
							},
						},
					},
				},
			},
		},
	}
}

func effectAttribute(description string) resourceschema.StringAttribute {
	return resourceschema.StringAttribute{
		Required:            true,
		MarkdownDescription: description + " Supported values are `allow`, `block`, and `audit`.",
	}
}

func (r *PolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = data.Client
}

func (r *PolicyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data PolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validatePolicyModel(data)...)
}

func (r *PolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before creating ona_security_policy resources.",
		)
		return
	}

	createReq, diags := createPolicyRequest(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.SecurityService().CreateSecurityPolicy(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Security Policy", "creating the Ona security policy", err)
		return
	}
	policy := result.Msg.GetSecurityPolicy()
	if policy == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Security Policy", "The Ona API returned an empty security policy.")
		return
	}

	data.ID = types.StringValue(policy.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planned := data
	populatePolicyModel(&data, policy)
	preservePolicyPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_security_policy resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Security Policy", "Security policy ID is empty.")
		return
	}

	policy, err := r.getPolicy(ctx, id)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Security Policy", "reading the Ona security policy", err)
		return
	}
	if policy == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	prior := data
	data = PolicyModel{}
	populatePolicyModel(&data, policy)
	preservePolicyPlannedInputs(&data, prior)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before updating ona_security_policy resources.",
		)
		return
	}

	updateReq, diags := updatePolicyRequest(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.SecurityService().UpdateSecurityPolicy(ctx, connect.NewRequest(updateReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Security Policy", "updating the Ona security policy", err)
		return
	}
	policy := result.Msg.GetSecurityPolicy()
	if policy == nil {
		resp.Diagnostics.AddError("Unable to Update Ona Security Policy", "The Ona API returned an empty security policy.")
		return
	}

	planned := data
	populatePolicyModel(&data, policy)
	preservePolicyPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before deleting ona_security_policy resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.SecurityService().DeleteSecurityPolicy(ctx, connect.NewRequest(&v1.DeleteSecurityPolicyRequest{
		SecurityPolicyId: id,
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Security Policy", "deleting the Ona security policy", err)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *PolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *PolicyResource) getPolicy(ctx context.Context, id string) (*v1.SecurityPolicy, error) {
	result, err := r.client.SecurityService().GetSecurityPolicy(ctx, connect.NewRequest(&v1.GetSecurityPolicyRequest{
		SecurityPolicyId: id,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get security policy: %w", err)
	}
	return result.Msg.GetSecurityPolicy(), nil
}

func createPolicyRequest(data PolicyModel) (*v1.CreateSecurityPolicyRequest, diag.Diagnostics) {
	spec, diags := securityPolicySpecFromModel(data.Spec, path.Root("spec"))
	if diags.HasError() {
		return nil, diags
	}
	return &v1.CreateSecurityPolicyRequest{
		OrganizationId: data.OrganizationID.ValueString(),
		Metadata: &v1.SecurityPolicy_Metadata{
			Name: data.Name.ValueString(),
		},
		Spec: spec,
	}, diags
}

func updatePolicyRequest(data PolicyModel) (*v1.UpdateSecurityPolicyRequest, diag.Diagnostics) {
	spec, diags := securityPolicySpecFromModel(data.Spec, path.Root("spec"))
	if diags.HasError() {
		return nil, diags
	}
	return &v1.UpdateSecurityPolicyRequest{
		SecurityPolicyId: data.ID.ValueString(),
		Metadata: &v1.SecurityPolicy_Metadata{
			Name: data.Name.ValueString(),
		},
		Spec: spec,
	}, diags
}

func populatePolicyModel(data *PolicyModel, policy *v1.SecurityPolicy) {
	data.ID = types.StringValue(policy.GetId())
	data.OrganizationID = types.StringValue(policy.GetOrganizationId())
	data.Name = types.StringValue(policy.GetMetadata().GetName())
	data.CreatedAt = timestampValue(policy.GetCreatedAt())
	data.UpdatedAt = timestampValue(policy.GetUpdatedAt())
	data.Spec = specModelFromSecurityPolicy(policy.GetSpec())
}

func preservePolicyPlannedInputs(data *PolicyModel, planned PolicyModel) {
	data.OrganizationID = preserveString(data.OrganizationID, planned.OrganizationID)
	data.Name = preserveString(data.Name, planned.Name)
}

func timestampValue(ts *timestamppb.Timestamp) types.String {
	if ts == nil || !ts.IsValid() {
		return types.StringNull()
	}
	return types.StringValue(ts.AsTime().UTC().Format(time.RFC3339Nano))
}

func preserveString(current types.String, planned types.String) types.String {
	if !planned.IsNull() && !planned.IsUnknown() {
		return planned
	}
	return current
}
