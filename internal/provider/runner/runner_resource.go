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
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
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
	Kind                      types.String        `tfsdk:"kind"`
	CloudFormationTemplateURL types.String        `tfsdk:"cloudformation_template_url"`
	CreatedAt                 types.String        `tfsdk:"created_at"`
	Configuration             *ConfigurationModel `tfsdk:"configuration"`
	Creator                   *CreatorModel       `tfsdk:"creator"`
}

type RunnerInputModel struct {
	ID             types.String        `tfsdk:"id"`
	RunnerID       types.String        `tfsdk:"runner_id"`
	Name           types.String        `tfsdk:"name"`
	RunnerProvider types.String        `tfsdk:"runner_provider"`
	Configuration  *ConfigurationModel `tfsdk:"configuration"`
}

type ConfigurationModel struct {
	Region                        types.String       `tfsdk:"region"`
	ReleaseChannel                types.String       `tfsdk:"release_channel"`
	AutoUpdate                    types.Bool         `tfsdk:"auto_update"`
	Metrics                       *MetricsModel      `tfsdk:"metrics"`
	UpdateWindow                  *UpdateWindowModel `tfsdk:"update_window"`
	DevcontainerImageCacheEnabled types.Bool         `tfsdk:"devcontainer_image_cache_enabled"`
	LogLevel                      types.String       `tfsdk:"log_level"`
}

type MetricsModel struct {
	Managed *ManagedMetricsModel `tfsdk:"managed"`
	Custom  *CustomMetricsModel  `tfsdk:"custom"`
}

type ManagedMetricsModel struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

type CustomMetricsModel struct {
	Enabled         types.Bool   `tfsdk:"enabled"`
	URL             types.String `tfsdk:"url"`
	Username        types.String `tfsdk:"username"`
	Password        types.String `tfsdk:"password"`
	PasswordVersion types.String `tfsdk:"password_version"`
}

type UpdateWindowModel struct {
	Start types.String `tfsdk:"start"`
	End   types.String `tfsdk:"end"`
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
	password := readCustomMetricsPassword(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

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

	createReq, diags := createRunnerRequest(data, password)
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

	prior := data
	data = RunnerModel{}
	populateModelFromRunner(&data, runner)
	preserveMetricsState(data.Configuration, prior.Configuration)
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
	password := readCustomMetricsPassword(ctx, req.Config, &resp.Diagnostics)
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

	updateReq, diags := updateRunnerRequest(id, data, prior, password)
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
	diags.Append(plan.GetAttribute(ctx, path.Root("configuration"), &data.Configuration)...)
	return data, diags
}

func (data RunnerInputModel) runnerModel() RunnerModel {
	return RunnerModel{
		ID:             data.ID,
		RunnerID:       data.RunnerID,
		Name:           data.Name,
		RunnerProvider: data.RunnerProvider,
		Configuration:  data.Configuration,
	}
}

func createRunnerRequest(data RunnerModel, password types.String) (*v1.CreateRunnerRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	provider, ok := providerFromString(data.RunnerProvider.ValueString())
	if !ok {
		diags.AddAttributeError(path.Root("runner_provider"), "Invalid Runner Provider", "Supported values are aws_ec2 and gcp.")
		return nil, diags
	}

	spec, specDiags := createRunnerSpec(data.Configuration, password)
	diags.Append(specDiags...)
	if diags.HasError() {
		return nil, diags
	}

	req := &v1.CreateRunnerRequest{
		Name:     data.Name.ValueString(),
		Provider: provider,
		Spec:     spec,
	}
	return req, diags
}

func createRunnerSpec(config *ConfigurationModel, password types.String) (*v1.RunnerSpec, diag.Diagnostics) {
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
	if config.Metrics != nil {
		spec.Configuration.Metrics = metricsConfigurationFromModel(config.Metrics, password)
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

func updateRunnerRequest(id string, data RunnerModel, prior RunnerModel, password types.String) (*v1.UpdateRunnerRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	req := &v1.UpdateRunnerRequest{
		RunnerId: id,
	}
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		name := data.Name.ValueString()
		req.Name = &name
	}
	if data.Configuration != nil {
		config, configDiags := updateRunnerConfiguration(data.Configuration, prior.Configuration, password)
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

func updateRunnerConfiguration(config *ConfigurationModel, prior *ConfigurationModel, password types.String) (*v1.UpdateRunnerRequest_RunnerConfiguration, diag.Diagnostics) {
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
	metrics, metricsDiags := updateMetricsConfiguration(config.Metrics, metricsFromConfiguration(prior), password)
	diags.Append(metricsDiags...)
	if diags.HasError() {
		return nil, diags
	}
	result.Metrics = metrics
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

func metricsFromConfiguration(config *ConfigurationModel) *MetricsModel {
	if config == nil {
		return nil
	}
	return config.Metrics
}

func metricsConfigurationFromModel(model *MetricsModel, password types.String) *v1.MetricsConfiguration {
	if model == nil {
		return nil
	}

	result := &v1.MetricsConfiguration{}
	if model.Managed != nil && !model.Managed.Enabled.IsNull() && !model.Managed.Enabled.IsUnknown() {
		result.ManagedMetricsEnabled = model.Managed.Enabled.ValueBool()
	}
	if model.Custom != nil {
		if !model.Custom.Enabled.IsNull() && !model.Custom.Enabled.IsUnknown() {
			result.Enabled = model.Custom.Enabled.ValueBool()
		}
		if !model.Custom.URL.IsNull() && !model.Custom.URL.IsUnknown() {
			result.Url = model.Custom.URL.ValueString()
		}
		if !model.Custom.Username.IsNull() && !model.Custom.Username.IsUnknown() {
			result.Username = model.Custom.Username.ValueString()
		}
		if !password.IsNull() && !password.IsUnknown() {
			result.Password = password.ValueString()
		}
	}
	return result
}

func updateMetricsConfiguration(model *MetricsModel, prior *MetricsModel, password types.String) (*v1.UpdateRunnerRequest_MetricsConfiguration, diag.Diagnostics) {
	var diags diag.Diagnostics
	if model == nil {
		if prior == nil {
			return nil, diags
		}
		return &v1.UpdateRunnerRequest_MetricsConfiguration{
			Enabled:               ptr(false),
			Url:                   ptr(""),
			Username:              ptr(""),
			Password:              ptr(""),
			ManagedMetricsEnabled: ptr(false),
		}, diags
	}

	result := &v1.UpdateRunnerRequest_MetricsConfiguration{}
	priorManaged := managedMetricsFromModel(prior)
	if model.Managed != nil && !model.Managed.Enabled.IsNull() && !model.Managed.Enabled.IsUnknown() {
		result.ManagedMetricsEnabled = ptr(model.Managed.Enabled.ValueBool())
	} else if priorManaged != nil {
		result.ManagedMetricsEnabled = ptr(false)
	}

	priorCustom := customMetricsFromModel(prior)
	if model.Custom == nil {
		if priorCustom != nil {
			result.Enabled = ptr(false)
			result.Url = ptr("")
			result.Username = ptr("")
			result.Password = ptr("")
		}
		return result, diags
	}
	if !model.Custom.Enabled.IsNull() && !model.Custom.Enabled.IsUnknown() {
		result.Enabled = ptr(model.Custom.Enabled.ValueBool())
	}
	if !model.Custom.URL.IsNull() && !model.Custom.URL.IsUnknown() {
		result.Url = ptr(model.Custom.URL.ValueString())
	} else if priorCustom != nil && !priorCustom.URL.IsNull() && !priorCustom.URL.IsUnknown() {
		result.Url = ptr("")
	}
	if !model.Custom.Username.IsNull() && !model.Custom.Username.IsUnknown() {
		result.Username = ptr(model.Custom.Username.ValueString())
	} else if priorCustom != nil && !priorCustom.Username.IsNull() && !priorCustom.Username.IsUnknown() {
		result.Username = ptr("")
	}
	if !password.IsNull() && !password.IsUnknown() {
		result.Password = ptr(password.ValueString())
	} else if priorCustom != nil && secretVersionChanged(model.Custom.PasswordVersion, priorCustom.PasswordVersion) {
		diags.AddAttributeError(
			path.Root("configuration").AtName("metrics").AtName("custom").AtName("password"),
			"Missing Custom Metrics Password",
			"Set configuration.metrics.custom.password when changing password_version.",
		)
		return nil, diags
	}
	return result, diags
}

func managedMetricsFromModel(model *MetricsModel) *ManagedMetricsModel {
	if model == nil {
		return nil
	}
	return model.Managed
}

func customMetricsFromModel(model *MetricsModel) *CustomMetricsModel {
	if model == nil {
		return nil
	}
	return model.Custom
}

func populateModelFromRunner(data *RunnerModel, runner *v1.Runner) {
	id := runner.GetRunnerId()
	data.ID = types.StringValue(id)
	data.RunnerID = types.StringValue(id)
	data.Name = types.StringValue(runner.GetName())
	data.RunnerProvider = stringValue(providerToString(runner.GetProvider()))
	data.Kind = stringValue(kindToString(runner.GetKind()))
	data.CreatedAt = timestampValue(runner.GetCreatedAt())
	data.Configuration = configurationModel(runner.GetSpec().GetConfiguration())
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
	data.Metrics = preserveMetrics(data.Metrics, planned.Metrics)
	data.DevcontainerImageCacheEnabled = preserveBool(data.DevcontainerImageCacheEnabled, planned.DevcontainerImageCacheEnabled)
	data.LogLevel = preserveString(data.LogLevel, planned.LogLevel)
	data.UpdateWindow = preserveUpdateWindow(data.UpdateWindow, planned.UpdateWindow)
	return data
}

func preserveMetrics(data *MetricsModel, planned *MetricsModel) *MetricsModel {
	if planned == nil {
		return data
	}
	if data == nil {
		data = &MetricsModel{}
	}
	if planned.Managed != nil {
		if data.Managed == nil {
			data.Managed = &ManagedMetricsModel{}
		}
		data.Managed.Enabled = preserveBool(data.Managed.Enabled, planned.Managed.Enabled)
	} else {
		data.Managed = nil
	}
	if planned.Custom != nil {
		if data.Custom == nil {
			data.Custom = &CustomMetricsModel{}
		}
		data.Custom.Enabled = preserveBool(data.Custom.Enabled, planned.Custom.Enabled)
		data.Custom.URL = preserveString(data.Custom.URL, planned.Custom.URL)
		data.Custom.Username = preserveString(data.Custom.Username, planned.Custom.Username)
		data.Custom.Password = types.StringNull()
		data.Custom.PasswordVersion = preserveString(data.Custom.PasswordVersion, planned.Custom.PasswordVersion)
	} else {
		data.Custom = nil
	}
	return data
}

func preserveMetricsState(data *ConfigurationModel, prior *ConfigurationModel) {
	if data == nil || prior == nil || prior.Metrics == nil {
		return
	}
	if data.Metrics == nil {
		if metricsInactive(prior.Metrics) {
			data.Metrics = prior.Metrics
			if data.Metrics.Custom != nil {
				data.Metrics.Custom.Password = types.StringNull()
			}
		}
		return
	}
	if data.Metrics.Managed == nil && prior.Metrics.Managed != nil && !knownBoolValue(prior.Metrics.Managed.Enabled) {
		data.Metrics.Managed = prior.Metrics.Managed
	}
	if data.Metrics.Custom == nil {
		if prior.Metrics.Custom != nil && customMetricsInactive(prior.Metrics.Custom) {
			data.Metrics.Custom = prior.Metrics.Custom
			data.Metrics.Custom.Password = types.StringNull()
		}
		return
	}
	data.Metrics.Custom.Password = types.StringNull()
	if prior.Metrics.Custom != nil {
		data.Metrics.Custom.PasswordVersion = prior.Metrics.Custom.PasswordVersion
	}
}

func metricsInactive(model *MetricsModel) bool {
	return model != nil &&
		(model.Managed == nil || !knownBoolValue(model.Managed.Enabled)) &&
		(model.Custom == nil || customMetricsInactive(model.Custom))
}

func customMetricsInactive(model *CustomMetricsModel) bool {
	return model != nil &&
		!knownBoolValue(model.Enabled) &&
		!knownStringValue(model.URL) &&
		!knownStringValue(model.Username) &&
		!knownStringValue(model.Password)
}

func knownBoolValue(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueBool()
}

func knownStringValue(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueString() != ""
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
		Metrics:                       metricsModel(config.GetMetrics()),
		UpdateWindow:                  updateWindowModel(config.GetUpdateWindow()),
		DevcontainerImageCacheEnabled: types.BoolValue(config.GetDevcontainerImageCacheEnabled()),
		LogLevel:                      stringValue(logLevelToString(config.GetLogLevel())),
	}
}

func metricsModel(metrics *v1.MetricsConfiguration) *MetricsModel {
	if metrics == nil {
		return nil
	}
	result := &MetricsModel{}
	if metrics.GetManagedMetricsEnabled() {
		result.Managed = &ManagedMetricsModel{Enabled: types.BoolValue(true)}
	}
	if metrics.GetEnabled() || metrics.GetUrl() != "" || metrics.GetUsername() != "" || metrics.GetPassword() != "" {
		result.Custom = &CustomMetricsModel{
			Enabled:         types.BoolValue(metrics.GetEnabled()),
			URL:             stringOptionalValue(metrics.GetUrl()),
			Username:        stringOptionalValue(metrics.GetUsername()),
			Password:        types.StringNull(),
			PasswordVersion: types.StringNull(),
		}
	}
	if result.Managed == nil && result.Custom == nil {
		return nil
	}
	return result
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
	validateCustomMetricsPassword(config.Metrics, diags)
}

func validateCustomMetricsPassword(metrics *MetricsModel, diags *diag.Diagnostics) {
	if metrics == nil || metrics.Custom == nil || metrics.Custom.PasswordVersion.IsNull() || metrics.Custom.PasswordVersion.IsUnknown() {
		return
	}
	if metrics.Custom.Password.IsUnknown() {
		return
	}
	if !isKnownString(metrics.Custom.Password) {
		diags.AddAttributeError(
			path.Root("configuration").AtName("metrics").AtName("custom").AtName("password"),
			"Missing Custom Metrics Password",
			"Set configuration.metrics.custom.password when setting password_version.",
		)
	}
}

func readCustomMetricsPassword(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) types.String {
	var configuration *ConfigurationModel
	diags.Append(cfg.GetAttribute(ctx, path.Root("configuration"), &configuration)...)
	if diags.HasError() || configuration == nil || configuration.Metrics == nil || configuration.Metrics.Custom == nil {
		return types.StringNull()
	}
	return configuration.Metrics.Custom.Password
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
