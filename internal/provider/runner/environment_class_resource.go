// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"
	"fmt"
	"sort"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &EnvironmentClassResource{}
var _ resource.ResourceWithConfigure = &EnvironmentClassResource{}
var _ resource.ResourceWithImportState = &EnvironmentClassResource{}

const defaultEnvironmentClassDescription = "Environment class managed by Terraform."

func NewEnvironmentClassResource() resource.Resource {
	return &EnvironmentClassResource{}
}

type EnvironmentClassResource struct {
	client *managementclient.ManagementPlane
}

type EnvironmentClassModel struct {
	ID            types.String `tfsdk:"id"`
	RunnerID      types.String `tfsdk:"runner_id"`
	DisplayName   types.String `tfsdk:"display_name"`
	Description   types.String `tfsdk:"description"`
	Configuration types.Map    `tfsdk:"configuration"`
	Enabled       types.Bool   `tfsdk:"enabled"`
}

func (r *EnvironmentClassResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_class"
}

func (r *EnvironmentClassResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona runner environment class used by projects to select runner capacity. Destroying this resource disables the remote environment class and removes it from Terraform state because the Ona API does not expose an environment class delete operation.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Environment class ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"runner_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID this environment class belongs to. Changing this value replaces the environment class.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Environment class display name shown to project and environment users.",
			},
			"description": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(defaultEnvironmentClassDescription),
				MarkdownDescription: "Environment class description. Defaults to `Environment class managed by Terraform.` when omitted.",
			},
			"configuration": resourceschema.MapAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Provider-specific environment class configuration as key/value strings, such as machine type and disk size. Valid keys depend on the runner provider. Changing this map replaces the environment class.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"enabled": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the environment class can be selected for new environments. Defaults to the provider value `true`.",
			},
		},
	}
}

func (r *EnvironmentClassResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentClassResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EnvironmentClassModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before creating ona_environment_class resources.",
		)
		return
	}

	createReq, diags := createEnvironmentClassRequest(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.RunnerConfigurationService().CreateEnvironmentClass(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Environment Class", "creating the Ona runner environment class", err)
		return
	}

	data.ID = types.StringValue(result.Msg.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if isKnownBool(data.Enabled) && !data.Enabled.ValueBool() {
		_, err := r.client.RunnerConfigurationService().UpdateEnvironmentClass(ctx, connect.NewRequest(&v1.UpdateEnvironmentClassRequest{
			EnvironmentClassId: result.Msg.GetId(),
			Enabled:            ptr(false),
		}))
		if err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Disable Created Ona Environment Class", "disabling the created Ona runner environment class", err)
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	class, err := r.getEnvironmentClass(ctx, result.Msg.GetId())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Created Ona Environment Class", "reading the created Ona runner environment class", err)
		return
	}
	if class == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	resp.Diagnostics.Append(populateEnvironmentClassModel(ctx, &data, class)...)
	preserveEnvironmentClassPlannedInputs(&data, planned)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentClassResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EnvironmentClassModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_environment_class resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Environment Class", "Environment class ID is empty.")
		return
	}

	class, err := r.getEnvironmentClass(ctx, id)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Environment Class", "reading the Ona runner environment class", err)
		return
	}
	if class == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data = EnvironmentClassModel{}
	resp.Diagnostics.Append(populateEnvironmentClassModel(ctx, &data, class)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentClassResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EnvironmentClassModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before updating ona_environment_class resources.",
		)
		return
	}

	updateReq := updateEnvironmentClassRequest(data)
	if _, err := r.client.RunnerConfigurationService().UpdateEnvironmentClass(ctx, connect.NewRequest(updateReq)); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Environment Class", "updating the Ona runner environment class", err)
		return
	}

	class, err := r.getEnvironmentClass(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona Environment Class", "reading the updated Ona runner environment class", err)
		return
	}
	if class == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	resp.Diagnostics.Append(populateEnvironmentClassModel(ctx, &data, class)...)
	preserveEnvironmentClassPlannedInputs(&data, planned)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentClassResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EnvironmentClassModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before deleting ona_environment_class resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	enabled := false
	_, err := r.client.RunnerConfigurationService().UpdateEnvironmentClass(ctx, connect.NewRequest(&v1.UpdateEnvironmentClassRequest{
		EnvironmentClassId: id,
		Enabled:            &enabled,
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Disable Ona Environment Class", "disabling the Ona runner environment class during destroy", err)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *EnvironmentClassResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *EnvironmentClassResource) getEnvironmentClass(ctx context.Context, id string) (*v1.EnvironmentClass, error) {
	result, err := r.client.RunnerConfigurationService().GetEnvironmentClass(ctx, connect.NewRequest(&v1.GetEnvironmentClassRequest{
		EnvironmentClassId: id,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get environment class: %w", err)
	}
	return result.Msg.GetEnvironmentClass(), nil
}

func createEnvironmentClassRequest(ctx context.Context, data EnvironmentClassModel) (*v1.CreateEnvironmentClassRequest, diag.Diagnostics) {
	configuration, diags := fieldValuesFromMap(ctx, data.Configuration, path.Root("configuration"))
	if diags.HasError() {
		return nil, diags
	}
	return &v1.CreateEnvironmentClassRequest{
		RunnerId:      data.RunnerID.ValueString(),
		DisplayName:   data.DisplayName.ValueString(),
		Description:   data.Description.ValueString(),
		Configuration: configuration,
	}, diags
}

func updateEnvironmentClassRequest(data EnvironmentClassModel) *v1.UpdateEnvironmentClassRequest {
	req := &v1.UpdateEnvironmentClassRequest{
		EnvironmentClassId: data.ID.ValueString(),
	}
	if isKnownString(data.DisplayName) {
		req.DisplayName = ptr(data.DisplayName.ValueString())
	}
	if !data.Description.IsUnknown() && !data.Description.IsNull() {
		req.Description = ptr(data.Description.ValueString())
	}
	if isKnownBool(data.Enabled) {
		req.Enabled = ptr(data.Enabled.ValueBool())
	}
	return req
}

func populateEnvironmentClassModel(ctx context.Context, data *EnvironmentClassModel, class *v1.EnvironmentClass) diag.Diagnostics {
	var diags diag.Diagnostics
	configuration, mapDiags := mapFromFieldValues(ctx, class.GetConfiguration())
	diags.Append(mapDiags...)
	if diags.HasError() {
		return diags
	}

	data.ID = types.StringValue(class.GetId())
	data.RunnerID = types.StringValue(class.GetRunnerId())
	data.DisplayName = types.StringValue(class.GetDisplayName())
	data.Description = types.StringValue(class.GetDescription())
	data.Configuration = configuration
	data.Enabled = types.BoolValue(class.GetEnabled())
	return diags
}

func preserveEnvironmentClassPlannedInputs(data *EnvironmentClassModel, planned EnvironmentClassModel) {
	data.RunnerID = preserveString(data.RunnerID, planned.RunnerID)
	data.DisplayName = preserveString(data.DisplayName, planned.DisplayName)
	data.Description = preserveString(data.Description, planned.Description)
	if !planned.Configuration.IsNull() && !planned.Configuration.IsUnknown() {
		data.Configuration = planned.Configuration
	}
	data.Enabled = preserveBool(data.Enabled, planned.Enabled)
}

func fieldValuesFromMap(ctx context.Context, value types.Map, p path.Path) ([]*v1.FieldValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		diags.AddAttributeError(p, "Missing Environment Class Configuration", "Set configuration before creating an Ona environment class.")
		return nil, diags
	}

	var raw map[string]string
	diags.Append(value.ElementsAs(ctx, &raw, false)...)
	if diags.HasError() {
		return nil, diags
	}

	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]*v1.FieldValue, 0, len(keys))
	for _, key := range keys {
		result = append(result, &v1.FieldValue{
			Key:   key,
			Value: raw[key],
		})
	}
	return result, diags
}

func mapFromFieldValues(ctx context.Context, values []*v1.FieldValue) (types.Map, diag.Diagnostics) {
	raw := make(map[string]string, len(values))
	for _, value := range values {
		raw[value.GetKey()] = value.GetValue()
	}
	return types.MapValueFrom(ctx, types.StringType, raw)
}
