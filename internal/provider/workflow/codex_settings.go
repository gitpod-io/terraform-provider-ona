// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	"context"
	"fmt"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

const codexAgentID = "00000000-0000-0000-0000-000000007800"

// These deprecated enum values remain readable for backward compatibility.
// Numeric aliases avoid coupling provider lint to deprecated generated symbols.
const (
	codexOpenAIModelGPT54Mini       = v1.CodexOpenAIModel(3)
	codexOpenAIModelGPT53Codex      = v1.CodexOpenAIModel(4)
	codexOpenAIModelGPT53CodexSpark = v1.CodexOpenAIModel(5)
	codexOpenAIModelGPT52           = v1.CodexOpenAIModel(6)
)

var (
	codexModelValues = []string{
		"gpt-5.5", "gpt-5.4", "gpt-5.3-codex", "gpt-5.3-codex-spark", "gpt-5.2", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna",
	}
	codexReasoningEffortValues = []string{"low", "medium", "high", "xhigh"}
	codexServiceTierValues     = []string{"fast"}
)

func codexSettingsFromObject(ctx context.Context, value types.Object, diags *diag.Diagnostics) *v1.CodexSettings {
	if value.IsNull() {
		return nil
	}
	if value.IsUnknown() {
		diags.AddAttributeError(path.Root("codex_settings"), "Unknown Codex Settings", "Codex settings must be known before apply.")
		return nil
	}
	var model CodexSettingsModel
	diags.Append(value.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil
	}
	result := &v1.CodexSettings{}
	if !model.Model.IsNull() && !model.Model.IsUnknown() {
		if mapped, ok := codexModelFromString(model.Model.ValueString()); ok {
			result.Model = mapped
		} else {
			diags.AddAttributeError(path.Root("codex_settings").AtName("model"), "Unsupported Codex Model", fmt.Sprintf("Supported values are %v.", codexModelValues))
		}
	}
	if !model.ReasoningEffort.IsNull() && !model.ReasoningEffort.IsUnknown() {
		if mapped, ok := codexReasoningEffortFromString(model.ReasoningEffort.ValueString()); ok {
			result.ReasoningEffort = mapped
		} else {
			diags.AddAttributeError(path.Root("codex_settings").AtName("reasoning_effort"), "Unsupported Codex Reasoning Effort", fmt.Sprintf("Supported values are %v.", codexReasoningEffortValues))
		}
	}
	if !model.ServiceTier.IsNull() && !model.ServiceTier.IsUnknown() {
		if mapped, ok := codexServiceTierFromString(model.ServiceTier.ValueString()); ok {
			result.ServiceTier = mapped
		} else {
			diags.AddAttributeError(path.Root("codex_settings").AtName("service_tier"), "Unsupported Codex Service Tier", fmt.Sprintf("Supported values are %v.", codexServiceTierValues))
		}
	}
	return result
}

func codexSettingsToObject(ctx context.Context, remote *v1.CodexSettings, diags *diag.Diagnostics) types.Object {
	if remote == nil {
		return types.ObjectNull(codexSettingsAttributeTypes)
	}
	model := CodexSettingsModel{Model: types.StringNull(), ReasoningEffort: types.StringNull(), ServiceTier: types.StringNull()}
	if remote.GetModel() != v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_UNSPECIFIED {
		if value, ok := codexModelToString(remote.GetModel()); ok {
			model.Model = types.StringValue(value)
		} else {
			diags.AddAttributeError(path.Root("codex_settings").AtName("model"), "Unsupported Remote Codex Model", fmt.Sprintf("The Ona API returned unsupported Codex model %q.", remote.GetModel().String()))
		}
	}
	if remote.GetReasoningEffort() != v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_UNSPECIFIED {
		if value, ok := codexReasoningEffortToString(remote.GetReasoningEffort()); ok {
			model.ReasoningEffort = types.StringValue(value)
		} else {
			diags.AddAttributeError(path.Root("codex_settings").AtName("reasoning_effort"), "Unsupported Remote Codex Reasoning Effort", fmt.Sprintf("The Ona API returned unsupported Codex reasoning effort %q.", remote.GetReasoningEffort().String()))
		}
	}
	if remote.GetServiceTier() != v1.CodexServiceTier_CODEX_SERVICE_TIER_UNSPECIFIED {
		if value, ok := codexServiceTierToString(remote.GetServiceTier()); ok {
			model.ServiceTier = types.StringValue(value)
		} else {
			diags.AddAttributeError(path.Root("codex_settings").AtName("service_tier"), "Unsupported Remote Codex Service Tier", fmt.Sprintf("The Ona API returned unsupported Codex service tier %q.", remote.GetServiceTier().String()))
		}
	}
	if diags.HasError() {
		return types.ObjectNull(codexSettingsAttributeTypes)
	}
	return objectValueFrom(ctx, codexSettingsAttributeTypes, model, diags)
}

func codexSettingsValueIsDefault(value types.Object) bool {
	if value.IsNull() {
		return true
	}
	if value.IsUnknown() {
		return false
	}
	for _, child := range value.Attributes() {
		if child.IsUnknown() || !child.IsNull() {
			return false
		}
	}
	return true
}

func codexModelFromString(value string) (v1.CodexOpenAIModel, bool) {
	switch value {
	case "gpt-5.5":
		return v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_5, true
	case "gpt-5.4":
		return v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_4, true
	case "gpt-5.3-codex":
		return codexOpenAIModelGPT53Codex, true
	case "gpt-5.3-codex-spark":
		return codexOpenAIModelGPT53CodexSpark, true
	case "gpt-5.2":
		return codexOpenAIModelGPT52, true
	case "gpt-5.6-sol":
		return v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_SOL, true
	case "gpt-5.6-terra":
		return v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_TERRA, true
	case "gpt-5.6-luna":
		return v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_LUNA, true
	default:
		return v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_UNSPECIFIED, false
	}
}

func codexModelToString(value v1.CodexOpenAIModel) (string, bool) {
	switch value {
	case v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_5:
		return "gpt-5.5", true
	case v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_4, codexOpenAIModelGPT54Mini:
		return "gpt-5.4", true
	case codexOpenAIModelGPT53Codex:
		return "gpt-5.3-codex", true
	case codexOpenAIModelGPT53CodexSpark:
		return "gpt-5.3-codex-spark", true
	case codexOpenAIModelGPT52:
		return "gpt-5.2", true
	case v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_SOL:
		return "gpt-5.6-sol", true
	case v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_TERRA:
		return "gpt-5.6-terra", true
	case v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_GPT_5_6_LUNA:
		return "gpt-5.6-luna", true
	default:
		return "", false
	}
}

func codexReasoningEffortFromString(value string) (v1.CodexReasoningEffort, bool) {
	switch value {
	case "low":
		return v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_LOW, true
	case "medium":
		return v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_MEDIUM, true
	case "high":
		return v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_HIGH, true
	case "xhigh":
		return v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_EXTRA_HIGH, true
	default:
		return v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_UNSPECIFIED, false
	}
}

func codexReasoningEffortToString(value v1.CodexReasoningEffort) (string, bool) {
	switch value {
	case v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_LOW:
		return "low", true
	case v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_MEDIUM:
		return "medium", true
	case v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_HIGH:
		return "high", true
	case v1.CodexReasoningEffort_CODEX_REASONING_EFFORT_EXTRA_HIGH:
		return "xhigh", true
	default:
		return "", false
	}
}

func codexServiceTierFromString(value string) (v1.CodexServiceTier, bool) {
	if value == "fast" {
		return v1.CodexServiceTier_CODEX_SERVICE_TIER_FAST, true
	}
	return v1.CodexServiceTier_CODEX_SERVICE_TIER_UNSPECIFIED, false
}

func codexServiceTierToString(value v1.CodexServiceTier) (string, bool) {
	if value == v1.CodexServiceTier_CODEX_SERVICE_TIER_FAST {
		return "fast", true
	}
	return "", false
}

func preserveDefaultCodexSettings(target *types.Object, source types.Object) {
	if codexSettingsValueIsDefault(*target) && codexSettingsValueIsDefault(source) {
		*target = source
	}
}
