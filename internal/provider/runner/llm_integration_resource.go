// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"
	"fmt"
	"sort"
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
)

var _ resource.Resource = &LLMIntegrationResource{}
var _ resource.ResourceWithConfigure = &LLMIntegrationResource{}
var _ resource.ResourceWithImportState = &LLMIntegrationResource{}
var _ resource.ResourceWithValidateConfig = &LLMIntegrationResource{}

func NewLLMIntegrationResource() resource.Resource {
	return &LLMIntegrationResource{}
}

type LLMIntegrationResource struct {
	client *managementclient.ManagementPlane
}

type LLMIntegrationModel struct {
	ID            types.String `tfsdk:"id"`
	RunnerID      types.String `tfsdk:"runner_id"`
	Models        types.Set    `tfsdk:"models"`
	Endpoint      types.String `tfsdk:"endpoint"`
	APIKey        types.String `tfsdk:"api_key"`
	APIKeyVersion types.String `tfsdk:"api_key_version"`
	MaxTokens     types.Int64  `tfsdk:"max_tokens"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	Phase         types.String `tfsdk:"phase"`
	PhaseReason   types.String `tfsdk:"phase_reason"`
	LLMProvider   types.String `tfsdk:"llm_provider"`
}

func (r *LLMIntegrationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_llm_integration"
}

func (r *LLMIntegrationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = llmIntegrationResourceSchema()
}

func (r *LLMIntegrationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *LLMIntegrationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data LLMIntegrationModel
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("models"), &data.Models)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("endpoint"), &data.Endpoint)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("max_tokens"), &data.MaxTokens)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateLLMIntegrationModels(ctx, data.Models, &resp.Diagnostics)
	validateLLMIntegrationEndpoint(data.Endpoint, &resp.Diagnostics)
	validateLLMIntegrationMaxTokens(data.MaxTokens, &resp.Diagnostics)
}

func (r *LLMIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LLMIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := readLLMIntegrationAPIKey(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before creating ona_runner_llm_integration resources.",
		)
		return
	}

	createReq, diags := createLLMIntegrationRequest(ctx, data, apiKey)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.RunnerConfigurationService().CreateLLMIntegration(ctx, connect.NewRequest(createReq))
	if err != nil {
		addLLMIntegrationWriteError(&resp.Diagnostics, "Unable to Create Ona Runner LLM Integration", data, err)
		return
	}

	data.ID = types.StringValue(result.Msg.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err := r.getLLMIntegration(ctx, result.Msg.GetId())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Created Ona Runner LLM Integration", "reading the created Ona runner LLM integration", err)
		return
	}
	if integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	resp.Diagnostics.Append(populateLLMIntegrationModel(ctx, &data, integration)...)
	preserveLLMIntegrationPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LLMIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LLMIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_runner_llm_integration resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Runner LLM Integration", "LLM integration ID is empty.")
		return
	}

	integration, err := r.getLLMIntegration(ctx, id)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Runner LLM Integration", "reading the Ona runner LLM integration", err)
		return
	}
	if integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	prior := data
	data = LLMIntegrationModel{}
	resp.Diagnostics.Append(populateLLMIntegrationModel(ctx, &data, integration)...)
	data.APIKey = types.StringNull()
	data.APIKeyVersion = prior.APIKeyVersion
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LLMIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data LLMIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior LLMIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := readLLMIntegrationAPIKey(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before updating ona_runner_llm_integration resources.",
		)
		return
	}

	updateReq, diags := updateLLMIntegrationRequest(ctx, data, prior, apiKey)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.RunnerConfigurationService().UpdateLLMIntegration(ctx, connect.NewRequest(updateReq)); err != nil {
		addLLMIntegrationWriteError(&resp.Diagnostics, "Unable to Update Ona Runner LLM Integration", data, err)
		return
	}

	integration, err := r.getLLMIntegration(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona Runner LLM Integration", "reading the updated Ona runner LLM integration", err)
		return
	}
	if integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	resp.Diagnostics.Append(populateLLMIntegrationModel(ctx, &data, integration)...)
	preserveLLMIntegrationPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LLMIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LLMIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before deleting ona_runner_llm_integration resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.RunnerConfigurationService().DeleteLLMIntegration(ctx, connect.NewRequest(&v1.DeleteLLMIntegrationRequest{Id: id}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Runner LLM Integration", "deleting the Ona runner LLM integration", err)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *LLMIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *LLMIntegrationResource) getLLMIntegration(ctx context.Context, id string) (*v1.LLMIntegration, error) {
	result, err := r.client.RunnerConfigurationService().GetLLMIntegration(ctx, connect.NewRequest(&v1.GetLLMIntegrationRequest{Id: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get LLM integration: %w", err)
	}
	return result.Msg.GetIntegration(), nil
}

func createLLMIntegrationRequest(ctx context.Context, data LLMIntegrationModel, apiKey types.String) (*v1.CreateLLMIntegrationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	models, modelDiags := supportedModelsFromSet(ctx, data.Models, path.Root("models"))
	diags.Append(modelDiags...)
	if !isKnownString(apiKey) {
		diags.AddAttributeError(path.Root("api_key"), "Missing LLM API Key", "Set api_key when creating an Ona runner LLM integration.")
	}
	if diags.HasError() {
		return nil, diags
	}

	req := &v1.CreateLLMIntegrationRequest{
		RunnerId: data.RunnerID.ValueString(),
		Models:   models,
		ApiKey:   apiKey.ValueString(),
	}
	if isKnownString(data.Endpoint) {
		req.Endpoint = data.Endpoint.ValueString()
	}
	if isKnownInt64(data.MaxTokens) {
		req.MaxTokens = uint64(data.MaxTokens.ValueInt64())
	}
	return req, diags
}

func updateLLMIntegrationRequest(ctx context.Context, data LLMIntegrationModel, prior LLMIntegrationModel, apiKey types.String) (*v1.UpdateLLMIntegrationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	req := &v1.UpdateLLMIntegrationRequest{
		Id: data.ID.ValueString(),
	}

	if stringValueChanged(data.Endpoint, prior.Endpoint) {
		req.Endpoint = ptr("")
		if isKnownString(data.Endpoint) {
			req.Endpoint = ptr(data.Endpoint.ValueString())
		}
	}

	if setValueChanged(data.Models, prior.Models) {
		models, modelDiags := supportedModelsFromSet(ctx, data.Models, path.Root("models"))
		diags.Append(modelDiags...)
		if diags.HasError() {
			return nil, diags
		}
		req.Models = models
	}

	if int64ValueChanged(data.MaxTokens, prior.MaxTokens) && isKnownInt64(data.MaxTokens) {
		req.MaxTokens = ptr(uint64(data.MaxTokens.ValueInt64()))
	}

	if boolValueChanged(data.Enabled, prior.Enabled) && isKnownBool(data.Enabled) {
		phase := v1.LLMIntegrationPhase_LLM_INTEGRATION_PHASE_AVAILABLE
		if !data.Enabled.ValueBool() {
			phase = v1.LLMIntegrationPhase_LLM_INTEGRATION_PHASE_DISABLED
		}
		req.Phase = &phase
	}

	if secretVersionChanged(data.APIKeyVersion, prior.APIKeyVersion) {
		if !isKnownString(apiKey) {
			diags.AddAttributeError(path.Root("api_key"), "Missing LLM API Key", "Set api_key when changing api_key_version.")
			return nil, diags
		}
		req.ApiKey = ptr(apiKey.ValueString())
	}

	return req, diags
}

func populateLLMIntegrationModel(ctx context.Context, data *LLMIntegrationModel, integration *v1.LLMIntegration) diag.Diagnostics {
	var diags diag.Diagnostics
	models, modelDiags := setFromSupportedModels(ctx, integration.GetModels())
	diags.Append(modelDiags...)
	if diags.HasError() {
		return diags
	}

	data.ID = types.StringValue(integration.GetId())
	data.RunnerID = types.StringValue(integration.GetRunnerId())
	data.Models = models
	data.Endpoint = stringOptionalValue(integration.GetEndpoint())
	data.APIKey = types.StringNull()
	data.APIKeyVersion = types.StringNull()
	data.MaxTokens = types.Int64Value(int64(integration.GetMaxTokens()))
	data.Phase = stringValue(llmIntegrationPhaseToString(integration.GetPhase()))
	data.PhaseReason = stringOptionalValue(integration.GetPhaseReason())
	data.LLMProvider = stringValue(llmProviderToString(integration.GetProvider()))
	data.Enabled = types.BoolValue(integration.GetPhase() != v1.LLMIntegrationPhase_LLM_INTEGRATION_PHASE_DISABLED)
	return diags
}

func preserveLLMIntegrationPlannedInputs(data *LLMIntegrationModel, planned LLMIntegrationModel) {
	data.RunnerID = preserveString(data.RunnerID, planned.RunnerID)
	if !planned.Models.IsNull() && !planned.Models.IsUnknown() {
		data.Models = planned.Models
	}
	data.Endpoint = preserveString(data.Endpoint, planned.Endpoint)
	data.APIKey = types.StringNull()
	data.APIKeyVersion = preserveString(data.APIKeyVersion, planned.APIKeyVersion)
	data.MaxTokens = preserveInt64(data.MaxTokens, planned.MaxTokens)
	data.Enabled = preserveBool(data.Enabled, planned.Enabled)
}

func readLLMIntegrationAPIKey(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) types.String {
	var apiKey types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("api_key"), &apiKey)...)
	return apiKey
}

func validateLLMIntegrationModels(ctx context.Context, value types.Set, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	var models []string
	diags.Append(value.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return
	}
	if len(models) == 0 {
		diags.AddAttributeError(path.Root("models"), "Missing LLM Models", "Set at least one model for an Ona runner LLM integration.")
		return
	}
	for _, model := range models {
		if _, ok := supportedModelFromString(model); !ok {
			diags.AddAttributeError(
				path.Root("models"),
				"Invalid LLM Model",
				fmt.Sprintf("Unsupported model %q. Supported values are: %s.", model, strings.Join(supportedModelNames(), ", ")),
			)
		}
	}
}

func validateLLMIntegrationEndpoint(value types.String, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	if strings.TrimSpace(value.ValueString()) != value.ValueString() {
		diags.AddAttributeError(path.Root("endpoint"), "Invalid LLM Endpoint", "endpoint must not have leading or trailing whitespace.")
	}
}

func validateLLMIntegrationMaxTokens(value types.Int64, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	if value.ValueInt64() < 0 {
		diags.AddAttributeError(path.Root("max_tokens"), "Invalid LLM Max Tokens", "max_tokens must be greater than or equal to 0.")
	}
}

func supportedModelsFromSet(ctx context.Context, value types.Set, p path.Path) ([]v1.SupportedModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		diags.AddAttributeError(p, "Missing LLM Models", "Set at least one model for an Ona runner LLM integration.")
		return nil, diags
	}

	var raw []string
	diags.Append(value.ElementsAs(ctx, &raw, false)...)
	if diags.HasError() {
		return nil, diags
	}
	if len(raw) == 0 {
		diags.AddAttributeError(p, "Missing LLM Models", "Set at least one model for an Ona runner LLM integration.")
		return nil, diags
	}

	sort.Strings(raw)
	result := make([]v1.SupportedModel, 0, len(raw))
	for _, model := range raw {
		apiModel, ok := supportedModelFromString(model)
		if !ok {
			diags.AddAttributeError(
				p,
				"Invalid LLM Model",
				fmt.Sprintf("Unsupported model %q. Supported values are: %s.", model, strings.Join(supportedModelNames(), ", ")),
			)
			continue
		}
		result = append(result, apiModel)
	}
	return result, diags
}

func setFromSupportedModels(ctx context.Context, models []v1.SupportedModel) (types.Set, diag.Diagnostics) {
	values := make([]string, 0, len(models))
	for _, model := range models {
		name := supportedModelToString(model)
		if name == "" {
			continue
		}
		values = append(values, name)
	}
	sort.Strings(values)
	return types.SetValueFrom(ctx, types.StringType, values)
}

func addLLMIntegrationWriteError(diags *diag.Diagnostics, summary string, data LLMIntegrationModel, err error) {
	if isRunnerPublicKeyMissingError(err) {
		diags.AddError(
			"Runner Public Key Is Not Available",
			fmt.Sprintf(
				"Ona cannot encrypt the LLM API key for runner %q because the runner has not registered its public key yet. Deploy the runner first, wait for it to register, then rerun this Terraform configuration.",
				data.RunnerID.ValueString(),
			),
		)
		return
	}

	providerdiag.AddAPIError(diags, summary, "writing the Ona runner LLM integration", err)
}

func supportedModelFromString(value string) (v1.SupportedModel, bool) {
	switch value {
	case "sonnet_3_5":
		return v1.SupportedModel_SUPPORTED_MODEL_SONNET_3_5, true
	case "sonnet_3_7":
		return v1.SupportedModel_SUPPORTED_MODEL_SONNET_3_7, true
	case "sonnet_3_7_extended":
		return v1.SupportedModel_SUPPORTED_MODEL_SONNET_3_7_EXTENDED, true
	case "sonnet_4":
		return v1.SupportedModel_SUPPORTED_MODEL_SONNET_4, true
	case "sonnet_4_extended":
		return v1.SupportedModel_SUPPORTED_MODEL_SONNET_4_EXTENDED, true
	case "sonnet_4_5":
		return v1.SupportedModel_SUPPORTED_MODEL_SONNET_4_5, true
	case "sonnet_4_5_extended":
		return v1.SupportedModel_SUPPORTED_MODEL_SONNET_4_5_EXTENDED, true
	case "sonnet_4_6":
		return v1.SupportedModel_SUPPORTED_MODEL_SONNET_4_6, true
	case "sonnet_4_6_extended":
		return v1.SupportedModel_SUPPORTED_MODEL_SONNET_4_6_EXTENDED, true
	case "sonnet_5":
		return v1.SupportedModel_SUPPORTED_MODEL_SONNET_5, true
	case "opus_4":
		return v1.SupportedModel_SUPPORTED_MODEL_OPUS_4, true
	case "opus_4_extended":
		return v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_EXTENDED, true
	case "opus_4_5":
		return v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_5, true
	case "opus_4_5_extended":
		return v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_5_EXTENDED, true
	case "opus_4_6":
		return v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_6, true
	case "opus_4_6_extended":
		return v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_6_EXTENDED, true
	case "opus_4_7":
		return v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_7, true
	case "opus_4_8":
		return v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_8, true
	case "haiku_4_5":
		return v1.SupportedModel_SUPPORTED_MODEL_HAIKU_4_5, true
	case "openai_4o":
		return v1.SupportedModel_SUPPORTED_MODEL_OPENAI_4O, true
	case "openai_4o_mini":
		return v1.SupportedModel_SUPPORTED_MODEL_OPENAI_4O_MINI, true
	case "openai_o1":
		return v1.SupportedModel_SUPPORTED_MODEL_OPENAI_O1, true
	case "openai_o1_mini":
		return v1.SupportedModel_SUPPORTED_MODEL_OPENAI_O1_MINI, true
	case "openai_auto":
		return v1.SupportedModel_SUPPORTED_MODEL_OPENAI_AUTO, true
	default:
		return v1.SupportedModel_SUPPORTED_MODEL_UNSPECIFIED, false
	}
}

func supportedModelToString(model v1.SupportedModel) string {
	switch model {
	case v1.SupportedModel_SUPPORTED_MODEL_SONNET_3_5:
		return "sonnet_3_5"
	case v1.SupportedModel_SUPPORTED_MODEL_SONNET_3_7:
		return "sonnet_3_7"
	case v1.SupportedModel_SUPPORTED_MODEL_SONNET_3_7_EXTENDED:
		return "sonnet_3_7_extended"
	case v1.SupportedModel_SUPPORTED_MODEL_SONNET_4:
		return "sonnet_4"
	case v1.SupportedModel_SUPPORTED_MODEL_SONNET_4_EXTENDED:
		return "sonnet_4_extended"
	case v1.SupportedModel_SUPPORTED_MODEL_SONNET_4_5:
		return "sonnet_4_5"
	case v1.SupportedModel_SUPPORTED_MODEL_SONNET_4_5_EXTENDED:
		return "sonnet_4_5_extended"
	case v1.SupportedModel_SUPPORTED_MODEL_SONNET_4_6:
		return "sonnet_4_6"
	case v1.SupportedModel_SUPPORTED_MODEL_SONNET_4_6_EXTENDED:
		return "sonnet_4_6_extended"
	case v1.SupportedModel_SUPPORTED_MODEL_SONNET_5:
		return "sonnet_5"
	case v1.SupportedModel_SUPPORTED_MODEL_OPUS_4:
		return "opus_4"
	case v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_EXTENDED:
		return "opus_4_extended"
	case v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_5:
		return "opus_4_5"
	case v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_5_EXTENDED:
		return "opus_4_5_extended"
	case v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_6:
		return "opus_4_6"
	case v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_6_EXTENDED:
		return "opus_4_6_extended"
	case v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_7:
		return "opus_4_7"
	case v1.SupportedModel_SUPPORTED_MODEL_OPUS_4_8:
		return "opus_4_8"
	case v1.SupportedModel_SUPPORTED_MODEL_HAIKU_4_5:
		return "haiku_4_5"
	case v1.SupportedModel_SUPPORTED_MODEL_OPENAI_4O:
		return "openai_4o"
	case v1.SupportedModel_SUPPORTED_MODEL_OPENAI_4O_MINI:
		return "openai_4o_mini"
	case v1.SupportedModel_SUPPORTED_MODEL_OPENAI_O1:
		return "openai_o1"
	case v1.SupportedModel_SUPPORTED_MODEL_OPENAI_O1_MINI:
		return "openai_o1_mini"
	case v1.SupportedModel_SUPPORTED_MODEL_OPENAI_AUTO:
		return "openai_auto"
	default:
		return ""
	}
}

func supportedModelNames() []string {
	names := []string{
		"haiku_4_5",
		"openai_4o",
		"openai_4o_mini",
		"openai_auto",
		"openai_o1",
		"openai_o1_mini",
		"opus_4",
		"opus_4_extended",
		"opus_4_5",
		"opus_4_5_extended",
		"opus_4_6",
		"opus_4_6_extended",
		"opus_4_7",
		"opus_4_8",
		"sonnet_3_5",
		"sonnet_3_7",
		"sonnet_3_7_extended",
		"sonnet_4",
		"sonnet_4_5",
		"sonnet_4_5_extended",
		"sonnet_4_6",
		"sonnet_4_6_extended",
		"sonnet_4_extended",
		"sonnet_5",
	}
	sort.Strings(names)
	return names
}

func llmIntegrationPhaseToString(phase v1.LLMIntegrationPhase) string {
	switch phase {
	case v1.LLMIntegrationPhase_LLM_INTEGRATION_PHASE_AVAILABLE:
		return "available"
	case v1.LLMIntegrationPhase_LLM_INTEGRATION_PHASE_UNAVAILABLE:
		return "unavailable"
	case v1.LLMIntegrationPhase_LLM_INTEGRATION_PHASE_DISABLED:
		return "disabled"
	default:
		return ""
	}
}

func llmProviderToString(provider v1.LLMProvider) string {
	switch provider {
	case v1.LLMProvider_LLM_PROVIDER_ANTHROPIC:
		return "anthropic"
	case v1.LLMProvider_LLM_PROVIDER_OPENAI:
		return "openai"
	default:
		return ""
	}
}

func isKnownInt64(value types.Int64) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func preserveInt64(current types.Int64, planned types.Int64) types.Int64 {
	if planned.IsNull() || planned.IsUnknown() {
		return current
	}
	return planned
}

func int64ValueChanged(current types.Int64, prior types.Int64) bool {
	if current.IsUnknown() || prior.IsUnknown() {
		return false
	}
	if current.IsNull() && prior.IsNull() {
		return false
	}
	if current.IsNull() != prior.IsNull() {
		return true
	}
	return current.ValueInt64() != prior.ValueInt64()
}

func boolValueChanged(current types.Bool, prior types.Bool) bool {
	if current.IsUnknown() || prior.IsUnknown() {
		return false
	}
	if current.IsNull() && prior.IsNull() {
		return false
	}
	if current.IsNull() != prior.IsNull() {
		return true
	}
	return current.ValueBool() != prior.ValueBool()
}

func setValueChanged(current types.Set, prior types.Set) bool {
	if current.IsUnknown() || prior.IsUnknown() {
		return false
	}
	if current.IsNull() && prior.IsNull() {
		return false
	}
	if current.IsNull() != prior.IsNull() {
		return true
	}
	return !current.Equal(prior)
}
