// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/api/go/client"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithConfigure = &Resource{}
var _ resource.ResourceWithIdentity = &Resource{}
var _ resource.ResourceWithImportState = &Resource{}
var _ resource.ResourceWithValidateConfig = &Resource{}

func NewResource() resource.Resource {
	return &Resource{}
}

type Resource struct {
	client *managementclient.ManagementPlane
}

type RunnerModel struct {
	ID                        types.String        `tfsdk:"id"`
	RunnerID                  types.String        `tfsdk:"runner_id"`
	Name                      types.String        `tfsdk:"name"`
	RunnerProvider            types.String        `tfsdk:"runner_provider"`
	RunnerManagerID           types.String        `tfsdk:"runner_manager_id"`
	Kind                      types.String        `tfsdk:"kind"`
	CloudFormationTemplateURL types.String        `tfsdk:"cloudformation_template_url"`
	CreatedAt                 types.String        `tfsdk:"created_at"`
	UpdatedAt                 types.String        `tfsdk:"updated_at"`
	Configuration             *ConfigurationModel `tfsdk:"configuration"`
	Status                    *StatusModel        `tfsdk:"status"`
	Creator                   *CreatorModel       `tfsdk:"creator"`
}

type RunnerInputModel struct {
	ID              types.String        `tfsdk:"id"`
	RunnerID        types.String        `tfsdk:"runner_id"`
	Name            types.String        `tfsdk:"name"`
	RunnerProvider  types.String        `tfsdk:"runner_provider"`
	RunnerManagerID types.String        `tfsdk:"runner_manager_id"`
	Configuration   *ConfigurationModel `tfsdk:"configuration"`
}

type ConfigurationModel struct {
	Region                        types.String       `tfsdk:"region"`
	ReleaseChannel                types.String       `tfsdk:"release_channel"`
	AutoUpdate                    types.Bool         `tfsdk:"auto_update"`
	UpdateWindow                  *UpdateWindowModel `tfsdk:"update_window"`
	DevcontainerImageCacheEnabled types.Bool         `tfsdk:"devcontainer_image_cache_enabled"`
	LogLevel                      types.String       `tfsdk:"log_level"`
}

type UpdateWindowModel struct {
	Start types.String `tfsdk:"start"`
	End   types.String `tfsdk:"end"`
}

type StatusModel struct {
	Phase            types.String `tfsdk:"phase"`
	Region           types.String `tfsdk:"region"`
	Message          types.String `tfsdk:"message"`
	Version          types.String `tfsdk:"version"`
	LogURL           types.String `tfsdk:"log_url"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
	SystemDetails    types.String `tfsdk:"system_details"`
	SupportBundleURL types.String `tfsdk:"support_bundle_url"`
}

type CreatorModel struct {
	ID        types.String `tfsdk:"id"`
	Principal types.String `tfsdk:"principal"`
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner"
}

func (r *Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data RunnerInputModel
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("runner_provider"), &data.RunnerProvider)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("configuration"), &data.Configuration)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateProvider(data.RunnerProvider, path.Root("runner_provider"), &resp.Diagnostics)
	validateConfiguration(data.RunnerProvider, data.Configuration, &resp.Diagnostics)
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	input, diags := runnerInputFromPlan(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data := input.runnerModel()

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before creating ona_runner resources.",
		)
		return
	}

	if data.Configuration == nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("configuration"),
			"Missing Runner Configuration",
			"Set a configuration block before creating an Ona runner.",
		)
		return
	}

	createReq, diags := createRunnerRequest(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.RunnerService().CreateRunner(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Runner", "creating the Ona runner registration", err)
		return
	}
	if result.Msg.GetRunner() == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Runner", "The Ona API returned an empty runner.")
		return
	}

	planned := data
	populateModelFromRunner(&data, result.Msg.GetRunner())
	preservePlannedInputs(&data, planned)
	populateCloudFormationTemplateURL(&data)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, RunnerIdentityModel{
		RunnerID: data.RunnerID,
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RunnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_runner resources.",
		)
		return
	}

	id := runnerID(data)
	if id == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Runner", "Runner ID is empty.")
		return
	}

	runner, err := r.getRunner(ctx, id)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Runner", "reading the Ona runner registration", err)
		return
	}
	if runner == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data = RunnerModel{}
	populateModelFromRunner(&data, runner)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, RunnerIdentityModel{
		RunnerID: data.RunnerID,
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	input, diags := runnerInputFromPlan(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data := input.runnerModel()

	var prior RunnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before updating ona_runner resources.",
		)
		return
	}

	id := runnerID(data)
	if id == "" {
		resp.Diagnostics.AddError("Unable to Update Ona Runner", "Runner ID is empty.")
		return
	}

	updateReq, diags := updateRunnerRequest(id, data, prior)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.RunnerService().UpdateRunner(ctx, connect.NewRequest(updateReq)); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Runner", "updating the Ona runner registration", err)
		return
	}

	runner, err := r.getRunner(ctx, id)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona Runner", "reading the updated Ona runner registration", err)
		return
	}
	if runner == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	populateModelFromRunner(&data, runner)
	preservePlannedInputs(&data, planned)
	populateCloudFormationTemplateURL(&data)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, RunnerIdentityModel{
		RunnerID: data.RunnerID,
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RunnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before deleting ona_runner resources.",
		)
		return
	}

	id := runnerID(data)
	if id == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.RunnerService().DeleteRunner(ctx, connect.NewRequest(&v1.DeleteRunnerRequest{
		RunnerId: id,
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Runner", "deleting the Ona runner registration", err)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("runner_id"), req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("runner_id"), path.Root("runner_id"), req, resp)
}

func (r *Resource) getRunner(ctx context.Context, id string) (*v1.Runner, error) {
	result, err := r.client.RunnerService().GetRunner(ctx, connect.NewRequest(&v1.GetRunnerRequest{
		RunnerId: id,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get runner: %w", err)
	}
	return result.Msg.GetRunner(), nil
}

func runnerInputFromPlan(ctx context.Context, plan tfsdk.Plan) (RunnerInputModel, diag.Diagnostics) {
	var data RunnerInputModel
	var diags diag.Diagnostics

	diags.Append(plan.GetAttribute(ctx, path.Root("id"), &data.ID)...)
	diags.Append(plan.GetAttribute(ctx, path.Root("runner_id"), &data.RunnerID)...)
	diags.Append(plan.GetAttribute(ctx, path.Root("name"), &data.Name)...)
	diags.Append(plan.GetAttribute(ctx, path.Root("runner_provider"), &data.RunnerProvider)...)
	diags.Append(plan.GetAttribute(ctx, path.Root("runner_manager_id"), &data.RunnerManagerID)...)
	diags.Append(plan.GetAttribute(ctx, path.Root("configuration"), &data.Configuration)...)
	return data, diags
}

func (data RunnerInputModel) runnerModel() RunnerModel {
	return RunnerModel{
		ID:              data.ID,
		RunnerID:        data.RunnerID,
		Name:            data.Name,
		RunnerProvider:  data.RunnerProvider,
		RunnerManagerID: data.RunnerManagerID,
		Configuration:   data.Configuration,
	}
}

func createRunnerRequest(data RunnerModel) (*v1.CreateRunnerRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	provider, ok := providerFromString(data.RunnerProvider.ValueString())
	if !ok {
		diags.AddAttributeError(path.Root("runner_provider"), "Invalid Runner Provider", "Supported values are aws_ec2 and gcp.")
		return nil, diags
	}

	spec, specDiags := createRunnerSpec(data.Configuration)
	diags.Append(specDiags...)
	if diags.HasError() {
		return nil, diags
	}

	req := &v1.CreateRunnerRequest{
		Name:     data.Name.ValueString(),
		Provider: provider,
		Spec:     spec,
	}
	if !data.RunnerManagerID.IsNull() && !data.RunnerManagerID.IsUnknown() {
		req.RunnerManagerId = data.RunnerManagerID.ValueString()
	}
	return req, diags
}

func createRunnerSpec(config *ConfigurationModel) (*v1.RunnerSpec, diag.Diagnostics) {
	var diags diag.Diagnostics
	if config == nil {
		return nil, diags
	}

	releaseChannel, ok := releaseChannelFromString(config.ReleaseChannel.ValueString())
	if !config.ReleaseChannel.IsNull() && !config.ReleaseChannel.IsUnknown() && !ok {
		diags.AddAttributeError(path.Root("configuration").AtName("release_channel"), "Invalid Runner Release Channel", "Supported values are stable and latest.")
	}
	logLevel, ok := logLevelFromString(config.LogLevel.ValueString())
	if !config.LogLevel.IsNull() && !config.LogLevel.IsUnknown() && !ok {
		diags.AddAttributeError(path.Root("configuration").AtName("log_level"), "Invalid Runner Log Level", "Supported values are debug, info, warn, and error.")
	}
	updateWindow, updateWindowDiags := updateWindowFromModel(config.UpdateWindow)
	diags.Append(updateWindowDiags...)
	if diags.HasError() {
		return nil, diags
	}

	spec := &v1.RunnerSpec{
		Configuration: &v1.RunnerConfiguration{},
	}
	if !config.Region.IsNull() && !config.Region.IsUnknown() {
		spec.Configuration.Region = config.Region.ValueString()
	}
	if !config.ReleaseChannel.IsNull() && !config.ReleaseChannel.IsUnknown() {
		spec.Configuration.ReleaseChannel = releaseChannel
	}
	if !config.AutoUpdate.IsNull() && !config.AutoUpdate.IsUnknown() {
		spec.Configuration.AutoUpdate = config.AutoUpdate.ValueBool()
	}
	if !config.DevcontainerImageCacheEnabled.IsNull() && !config.DevcontainerImageCacheEnabled.IsUnknown() {
		spec.Configuration.DevcontainerImageCacheEnabled = config.DevcontainerImageCacheEnabled.ValueBool()
	}
	if !config.LogLevel.IsNull() && !config.LogLevel.IsUnknown() {
		spec.Configuration.LogLevel = logLevel
	}
	if updateWindow != nil {
		spec.Configuration.UpdateWindow = updateWindow
	}
	return spec, diags
}

func updateRunnerRequest(id string, data RunnerModel, prior RunnerModel) (*v1.UpdateRunnerRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	req := &v1.UpdateRunnerRequest{
		RunnerId: id,
	}
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		name := data.Name.ValueString()
		req.Name = &name
	}
	if data.Configuration != nil {
		config, configDiags := updateRunnerConfiguration(data.Configuration, prior.Configuration)
		diags.Append(configDiags...)
		if diags.HasError() {
			return nil, diags
		}
		req.Spec = &v1.UpdateRunnerRequest_Spec{
			Configuration: config,
		}
	}
	return req, diags
}

func updateRunnerConfiguration(config *ConfigurationModel, prior *ConfigurationModel) (*v1.UpdateRunnerRequest_RunnerConfiguration, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := &v1.UpdateRunnerRequest_RunnerConfiguration{}

	if !config.ReleaseChannel.IsNull() && !config.ReleaseChannel.IsUnknown() {
		releaseChannel, ok := releaseChannelFromString(config.ReleaseChannel.ValueString())
		if !ok {
			diags.AddAttributeError(path.Root("configuration").AtName("release_channel"), "Invalid Runner Release Channel", "Supported values are stable and latest.")
			return nil, diags
		}
		result.ReleaseChannel = &releaseChannel
	}
	if !config.AutoUpdate.IsNull() && !config.AutoUpdate.IsUnknown() {
		value := config.AutoUpdate.ValueBool()
		result.AutoUpdate = &value
	}
	if !config.DevcontainerImageCacheEnabled.IsNull() && !config.DevcontainerImageCacheEnabled.IsUnknown() {
		value := config.DevcontainerImageCacheEnabled.ValueBool()
		result.DevcontainerImageCacheEnabled = &value
	}
	if !config.LogLevel.IsNull() && !config.LogLevel.IsUnknown() {
		logLevel, ok := logLevelFromString(config.LogLevel.ValueString())
		if !ok {
			diags.AddAttributeError(path.Root("configuration").AtName("log_level"), "Invalid Runner Log Level", "Supported values are debug, info, warn, and error.")
			return nil, diags
		}
		result.LogLevel = &logLevel
	}
	if config.UpdateWindow != nil {
		updateWindow, updateWindowDiags := updateWindowFromModel(config.UpdateWindow)
		diags.Append(updateWindowDiags...)
		if diags.HasError() {
			return nil, diags
		}
		result.UpdateWindow = updateWindow
	} else if prior != nil && prior.UpdateWindow != nil {
		result.UpdateWindow = &v1.UpdateWindow{}
	}

	return result, diags
}

func populateModelFromRunner(data *RunnerModel, runner *v1.Runner) {
	id := runner.GetRunnerId()
	data.ID = types.StringValue(id)
	data.RunnerID = types.StringValue(id)
	data.Name = types.StringValue(runner.GetName())
	data.RunnerProvider = stringValue(providerToString(runner.GetProvider()))
	data.RunnerManagerID = stringOptionalValue(runner.GetRunnerManagerId())
	data.Kind = stringValue(kindToString(runner.GetKind()))
	data.CreatedAt = timestampValue(runner.GetCreatedAt())
	data.UpdatedAt = timestampValue(runner.GetUpdatedAt())
	data.Configuration = configurationModel(runner.GetSpec().GetConfiguration())
	data.Status = statusModel(runner.GetStatus())
	data.Creator = creatorModel(runner.GetCreator())
	populateCloudFormationTemplateURL(data)
}

func populateCloudFormationTemplateURL(data *RunnerModel) {
	if data == nil || data.Configuration == nil {
		return
	}
	if data.RunnerProvider.IsNull() || data.RunnerProvider.IsUnknown() || data.RunnerProvider.ValueString() != "aws_ec2" {
		data.CloudFormationTemplateURL = types.StringNull()
		return
	}

	releaseChannel := "stable"
	if !data.Configuration.ReleaseChannel.IsNull() && !data.Configuration.ReleaseChannel.IsUnknown() && data.Configuration.ReleaseChannel.ValueString() != "" {
		releaseChannel = data.Configuration.ReleaseChannel.ValueString()
	}
	data.CloudFormationTemplateURL = types.StringValue(fmt.Sprintf("https://gitpod-flex-releases.s3.amazonaws.com/ec2/%s/gitpod-ec2-runner.json", releaseChannel))
}

func preservePlannedInputs(data *RunnerModel, planned RunnerModel) {
	data.Name = preserveString(data.Name, planned.Name)
	data.RunnerProvider = preserveString(data.RunnerProvider, planned.RunnerProvider)
	data.RunnerManagerID = preserveString(data.RunnerManagerID, planned.RunnerManagerID)
	data.Configuration = preserveConfiguration(data.Configuration, planned.Configuration)
}

func preserveConfiguration(data *ConfigurationModel, planned *ConfigurationModel) *ConfigurationModel {
	if planned == nil {
		return data
	}
	if data == nil {
		data = &ConfigurationModel{}
	}
	data.Region = preserveString(data.Region, planned.Region)
	data.ReleaseChannel = preserveString(data.ReleaseChannel, planned.ReleaseChannel)
	data.AutoUpdate = preserveBool(data.AutoUpdate, planned.AutoUpdate)
	data.DevcontainerImageCacheEnabled = preserveBool(data.DevcontainerImageCacheEnabled, planned.DevcontainerImageCacheEnabled)
	data.LogLevel = preserveString(data.LogLevel, planned.LogLevel)
	data.UpdateWindow = preserveUpdateWindow(data.UpdateWindow, planned.UpdateWindow)
	return data
}

func preserveUpdateWindow(data *UpdateWindowModel, planned *UpdateWindowModel) *UpdateWindowModel {
	if planned == nil {
		return data
	}
	if data == nil {
		data = &UpdateWindowModel{}
	}
	data.Start = preserveString(data.Start, planned.Start)
	data.End = preserveString(data.End, planned.End)
	return data
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

func configurationModel(config *v1.RunnerConfiguration) *ConfigurationModel {
	if config == nil {
		return nil
	}
	return &ConfigurationModel{
		Region:                        stringOptionalValue(config.GetRegion()),
		ReleaseChannel:                stringValue(releaseChannelToString(config.GetReleaseChannel())),
		AutoUpdate:                    types.BoolValue(config.GetAutoUpdate()),
		UpdateWindow:                  updateWindowModel(config.GetUpdateWindow()),
		DevcontainerImageCacheEnabled: types.BoolValue(config.GetDevcontainerImageCacheEnabled()),
		LogLevel:                      stringValue(logLevelToString(config.GetLogLevel())),
	}
}

func updateWindowModel(updateWindow *v1.UpdateWindow) *UpdateWindowModel {
	if updateWindow == nil || updateWindow.StartHour == nil {
		return nil
	}

	result := &UpdateWindowModel{
		Start: types.StringValue(formatHour(updateWindow.GetStartHour())),
		End:   types.StringNull(),
	}
	if updateWindow.EndHour != nil {
		result.End = types.StringValue(formatHour(updateWindow.GetEndHour()))
	}
	return result
}

func statusModel(status *v1.RunnerStatus) *StatusModel {
	if status == nil {
		return nil
	}
	return &StatusModel{
		Phase:            stringValue(phaseToString(status.GetPhase())),
		Region:           stringOptionalValue(status.GetRegion()),
		Message:          stringOptionalValue(status.GetMessage()),
		Version:          stringOptionalValue(status.GetVersion()),
		LogURL:           stringOptionalValue(status.GetLogUrl()),
		UpdatedAt:        timestampValue(status.GetUpdatedAt()),
		SystemDetails:    stringOptionalValue(status.GetSystemDetails()),
		SupportBundleURL: stringOptionalValue(status.GetSupportBundleUrl()),
	}
}

func creatorModel(creator *v1.Subject) *CreatorModel {
	if creator == nil {
		return nil
	}
	return &CreatorModel{
		ID:        stringOptionalValue(creator.GetId()),
		Principal: stringValue(principalToString(creator.GetPrincipal())),
	}
}

func validateProvider(value types.String, p path.Path, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	if _, ok := providerFromString(value.ValueString()); !ok {
		diags.AddAttributeError(p, "Invalid Runner Provider", "Supported values are aws_ec2 and gcp.")
	}
}

func validateConfiguration(provider types.String, config *ConfigurationModel, diags *diag.Diagnostics) {
	if config == nil {
		return
	}
	validateRegion(provider, config.Region, diags)
	if !config.ReleaseChannel.IsNull() && !config.ReleaseChannel.IsUnknown() {
		if _, ok := releaseChannelFromString(config.ReleaseChannel.ValueString()); !ok {
			diags.AddAttributeError(path.Root("configuration").AtName("release_channel"), "Invalid Runner Release Channel", "Supported values are stable and latest.")
		}
	}
	if !config.LogLevel.IsNull() && !config.LogLevel.IsUnknown() {
		if _, ok := logLevelFromString(config.LogLevel.ValueString()); !ok {
			diags.AddAttributeError(path.Root("configuration").AtName("log_level"), "Invalid Runner Log Level", "Supported values are debug, info, warn, and error.")
		}
	}
	if config.UpdateWindow != nil {
		validateUpdateWindow(config.UpdateWindow, diags)
	}
}

func validateRegion(provider types.String, region types.String, diags *diag.Diagnostics) {
	if provider.IsNull() || provider.IsUnknown() {
		return
	}
	if _, ok := providerFromString(provider.ValueString()); !ok {
		return
	}
	if !runnerProviderRequiresRegion(provider.ValueString()) {
		return
	}
	if region.IsUnknown() {
		return
	}
	if region.IsNull() || region.ValueString() == "" {
		diags.AddAttributeError(
			path.Root("configuration").AtName("region"),
			"Missing Runner Region",
			fmt.Sprintf("configuration.region is required when runner_provider is %q.", provider.ValueString()),
		)
	}
}

func runnerProviderRequiresRegion(provider string) bool {
	switch provider {
	case "aws_ec2":
		return true
	default:
		return false
	}
}

func validateUpdateWindow(model *UpdateWindowModel, diags *diag.Diagnostics) {
	if model == nil {
		return
	}
	if model.Start.IsUnknown() {
		return
	}
	if model.Start.IsNull() {
		diags.AddAttributeError(path.Root("configuration").AtName("update_window").AtName("start"), "Missing Update Window Start", "Set a start time in HH:00 UTC format when update_window is configured.")
		return
	}
	start, ok := parseHour(model.Start)
	if !ok {
		diags.AddAttributeError(path.Root("configuration").AtName("update_window").AtName("start"), "Invalid Update Window Start", "Use HH:00 UTC format with an hour from 00 through 23.")
		return
	}
	if model.End.IsNull() || model.End.IsUnknown() {
		return
	}
	end, ok := parseHour(model.End)
	if !ok {
		diags.AddAttributeError(path.Root("configuration").AtName("update_window").AtName("end"), "Invalid Update Window End", "Use HH:00 UTC format with an hour from 00 through 23.")
		return
	}
	if windowLength(start, end) < 2 {
		diags.AddAttributeError(path.Root("configuration").AtName("update_window"), "Invalid Update Window", "The update window must be at least two hours long.")
	}
}

func updateWindowFromModel(model *UpdateWindowModel) (*v1.UpdateWindow, diag.Diagnostics) {
	var diags diag.Diagnostics
	if model == nil {
		return nil, diags
	}

	start, ok := parseHour(model.Start)
	if !ok {
		diags.AddAttributeError(path.Root("configuration").AtName("update_window").AtName("start"), "Invalid Update Window Start", "Use HH:00 UTC format with an hour from 00 through 23.")
		return nil, diags
	}

	result := &v1.UpdateWindow{
		StartHour: ptr(uint32(start)),
	}
	if !model.End.IsNull() && !model.End.IsUnknown() {
		end, ok := parseHour(model.End)
		if !ok {
			diags.AddAttributeError(path.Root("configuration").AtName("update_window").AtName("end"), "Invalid Update Window End", "Use HH:00 UTC format with an hour from 00 through 23.")
			return nil, diags
		}
		result.EndHour = ptr(uint32(end))
		if windowLength(start, end) < 2 {
			diags.AddAttributeError(path.Root("configuration").AtName("update_window"), "Invalid Update Window", "The update window must be at least two hours long.")
		}
	}
	return result, diags
}

func runnerID(data RunnerModel) string {
	if !data.RunnerID.IsNull() && !data.RunnerID.IsUnknown() && data.RunnerID.ValueString() != "" {
		return data.RunnerID.ValueString()
	}
	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		return data.ID.ValueString()
	}
	return ""
}

var hourPattern = regexp.MustCompile(`^(?:[01][0-9]|2[0-3]):00$`)

func parseHour(value types.String) (int, bool) {
	if value.IsNull() || value.IsUnknown() {
		return 0, false
	}
	raw := value.ValueString()
	if !hourPattern.MatchString(raw) {
		return 0, false
	}
	hour, err := strconv.Atoi(strings.TrimSuffix(raw, ":00"))
	if err != nil {
		return 0, false
	}
	return hour, true
}

func formatHour(hour uint32) string {
	return fmt.Sprintf("%02d:00", hour)
}

func windowLength(start int, end int) int {
	if end >= start {
		return end - start
	}
	return end + 24 - start
}

func providerFromString(value string) (v1.RunnerProvider, bool) {
	switch value {
	case "aws_ec2":
		return v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2, true
	case "gcp":
		return v1.RunnerProvider_RUNNER_PROVIDER_GCP, true
	default:
		return v1.RunnerProvider_RUNNER_PROVIDER_UNSPECIFIED, false
	}
}

func providerToString(provider v1.RunnerProvider) string {
	switch provider {
	case v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2:
		return "aws_ec2"
	case v1.RunnerProvider_RUNNER_PROVIDER_GCP:
		return "gcp"
	case v1.RunnerProvider_RUNNER_PROVIDER_MANAGED:
		return "managed"
	case v1.RunnerProvider_RUNNER_PROVIDER_DEV_AGENT:
		return "dev_agent"
	default:
		return ""
	}
}

func releaseChannelFromString(value string) (v1.RunnerReleaseChannel, bool) {
	switch value {
	case "stable":
		return v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_STABLE, true
	case "latest":
		return v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_LATEST, true
	default:
		return v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_UNSPECIFIED, false
	}
}

func releaseChannelToString(channel v1.RunnerReleaseChannel) string {
	switch channel {
	case v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_STABLE:
		return "stable"
	case v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_LATEST:
		return "latest"
	default:
		return ""
	}
}

func logLevelFromString(value string) (v1.LogLevel, bool) {
	switch value {
	case "debug":
		return v1.LogLevel_LOG_LEVEL_DEBUG, true
	case "info":
		return v1.LogLevel_LOG_LEVEL_INFO, true
	case "warn":
		return v1.LogLevel_LOG_LEVEL_WARN, true
	case "error":
		return v1.LogLevel_LOG_LEVEL_ERROR, true
	default:
		return v1.LogLevel_LOG_LEVEL_UNSPECIFIED, false
	}
}

func logLevelToString(level v1.LogLevel) string {
	switch level {
	case v1.LogLevel_LOG_LEVEL_DEBUG:
		return "debug"
	case v1.LogLevel_LOG_LEVEL_INFO:
		return "info"
	case v1.LogLevel_LOG_LEVEL_WARN:
		return "warn"
	case v1.LogLevel_LOG_LEVEL_ERROR:
		return "error"
	default:
		return ""
	}
}

func kindToString(kind v1.RunnerKind) string {
	switch kind {
	case v1.RunnerKind_RUNNER_KIND_REMOTE:
		return "remote"
	case v1.RunnerKind_RUNNER_KIND_LOCAL_CONFIGURATION:
		return "local_configuration"
	default:
		return ""
	}
}

func phaseToString(phase v1.RunnerPhase) string {
	switch phase {
	case v1.RunnerPhase_RUNNER_PHASE_CREATED:
		return "created"
	case v1.RunnerPhase_RUNNER_PHASE_INACTIVE:
		return "inactive"
	case v1.RunnerPhase_RUNNER_PHASE_ACTIVE:
		return "active"
	case v1.RunnerPhase_RUNNER_PHASE_DELETING:
		return "deleting"
	case v1.RunnerPhase_RUNNER_PHASE_DELETED:
		return "deleted"
	case v1.RunnerPhase_RUNNER_PHASE_DEGRADED:
		return "degraded"
	default:
		return ""
	}
}

func principalToString(principal v1.Principal) string {
	switch principal {
	case v1.Principal_PRINCIPAL_USER:
		return "user"
	case v1.Principal_PRINCIPAL_SERVICE_ACCOUNT:
		return "service_account"
	case v1.Principal_PRINCIPAL_ACCOUNT:
		return "account"
	case v1.Principal_PRINCIPAL_RUNNER:
		return "runner"
	case v1.Principal_PRINCIPAL_ENVIRONMENT:
		return "environment"
	default:
		return ""
	}
}

func timestampValue(ts *timestamppb.Timestamp) types.String {
	if ts == nil || !ts.IsValid() {
		return types.StringNull()
	}
	return types.StringValue(ts.AsTime().Format("2006-01-02T15:04:05Z07:00"))
}

func stringValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func stringOptionalValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func ptr[T any](value T) *T {
	return &value
}
