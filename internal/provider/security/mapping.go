// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package security

import (
	"context"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func validatePolicyModel(ctx context.Context, data PolicyModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if !data.Name.IsUnknown() && !data.Name.IsNull() {
		name := data.Name.ValueString()
		if name == "" || len(name) > 80 {
			diags.AddAttributeError(path.Root("name"), "Invalid Security Policy Name", "Security policy name must be between 1 and 80 characters.")
		}
	}
	_, specDiags := securityPolicySpecFromModel(ctx, data.Spec, path.Root("spec"))
	diags.Append(specDiags...)
	return diags
}

func securityPolicySpecFromModel(ctx context.Context, model *SpecModel, root path.Path) (*v1.SecurityPolicy_Spec, diag.Diagnostics) {
	var diags diag.Diagnostics
	if model == nil {
		diags.AddAttributeError(root, "Missing Security Policy Spec", "Set a spec block before creating an Ona security policy.")
		return nil, diags
	}

	spec := &v1.SecurityPolicy_Spec{}
	if model.Ports != nil {
		validatePortPolicyModel(model.Ports, root.AtName("ports"), &diags)
	}
	if model.Executables != nil {
		spec.Executables = executablePolicyFromModel(model.Executables, root.AtName("executables"), &diags)
	}
	if model.Files != nil {
		validateFilePolicyModel(ctx, model.Files, root.AtName("files"), &diags)
	}
	if model.BlockDevices != nil {
		effectFromString(model.BlockDevices.DefaultEffect, root.AtName("block_devices").AtName("default_effect"), &diags)
	}
	if model.Data != nil {
		validateDataPolicyModel(model.Data, root.AtName("data"), &diags)
	}
	return spec, diags
}

func validatePortPolicyModel(model *PortPolicyModel, root path.Path, diags *diag.Diagnostics) {
	effectFromString(model.DefaultEffect, root.AtName("default_effect"), diags)
	for idx, rule := range model.Rules {
		rulePath := root.AtName("rule").AtListIndex(idx)
		from := rule.RangeFrom.ValueInt64()
		to := rule.RangeTo.ValueInt64()
		if !rule.RangeFrom.IsUnknown() && !rule.RangeFrom.IsNull() && (from < 0 || from > 65535) {
			diags.AddAttributeError(rulePath.AtName("range_from"), "Invalid Port Range", "range_from must be between 0 and 65535.")
		}
		if !rule.RangeTo.IsUnknown() && !rule.RangeTo.IsNull() && (to < 0 || to > 65535) {
			diags.AddAttributeError(rulePath.AtName("range_to"), "Invalid Port Range", "range_to must be between 0 and 65535.")
		}
		if !rule.RangeFrom.IsUnknown() && !rule.RangeFrom.IsNull() && !rule.RangeTo.IsUnknown() && !rule.RangeTo.IsNull() && from > to {
			diags.AddAttributeError(rulePath.AtName("range_to"), "Invalid Port Range", "range_to must be greater than or equal to range_from.")
		}
		effectFromString(rule.Effect, rulePath.AtName("effect"), diags)
	}
}

func executablePolicyFromModel(model *ExecutablePolicyModel, root path.Path, diags *diag.Diagnostics) *v1.SecurityPolicy_Spec_ExecutablePolicy {
	policy := &v1.SecurityPolicy_Spec_ExecutablePolicy{
		DefaultEffect: effectFromString(model.DefaultEffect, root.AtName("default_effect"), diags),
	}
	for idx, rule := range model.Rules {
		rulePath := root.AtName("rule").AtListIndex(idx)
		policy.Rules = append(policy.Rules, &v1.SecurityPolicy_Spec_ExecutablePolicy_Rule{
			Path:   rule.Path.ValueString(),
			Effect: effectFromString(rule.Effect, rulePath.AtName("effect"), diags),
		})
	}
	return policy
}

func validateFilePolicyModel(ctx context.Context, model *FilePolicyModel, root path.Path, diags *diag.Diagnostics) {
	effectFromString(model.DefaultEffect, root.AtName("default_effect"), diags)
	if isKnownSet(model.DefaultActions) {
		actionDiags := validateFileActionsSet(ctx, model.DefaultActions, root.AtName("default_actions"))
		diags.Append(actionDiags...)
	}
	for idx, rule := range model.Rules {
		rulePath := root.AtName("rule").AtListIndex(idx)
		effectFromString(rule.Effect, rulePath.AtName("effect"), diags)
		if isKnownSet(rule.Actions) {
			actionDiags := validateFileActionsSet(ctx, rule.Actions, rulePath.AtName("actions"))
			diags.Append(actionDiags...)
		}
	}
}

func validateDataPolicyModel(model *DataPolicyModel, root path.Path, diags *diag.Diagnostics) {
	effectFromString(model.DefaultEffect, root.AtName("default_effect"), diags)
	for idx, rule := range model.Rules {
		rulePath := root.AtName("rule").AtListIndex(idx)
		effectFromString(rule.Effect, rulePath.AtName("effect"), diags)
		if rule.Source == nil {
			diags.AddAttributeError(rulePath.AtName("source"), "Missing Data Source", "Set a source block for each data rule.")
		} else {
			validateDataSourceModel(rule.Source, rulePath.AtName("source"), diags)
		}
		if rule.Destination == nil {
			diags.AddAttributeError(rulePath.AtName("destination"), "Missing Data Destination", "Set a destination block for each data rule.")
		}
	}
}

func validateDataSourceModel(model *DataSourceModel, root path.Path, diags *diag.Diagnostics) {
	hasFile := isKnownString(model.File) && model.File.ValueString() != ""
	hasIntegration := isKnownString(model.Integration) && model.Integration.ValueString() != ""
	if hasFile == hasIntegration {
		diags.AddAttributeError(root, "Invalid Data Source Reference", "Set exactly one of source.file or source.integration.")
	}
}

func effectFromString(value types.String, attrPath path.Path, diags *diag.Diagnostics) v1.SecurityPolicy_Effect {
	if value.IsUnknown() || value.IsNull() {
		return v1.SecurityPolicy_EFFECT_UNSPECIFIED
	}
	switch value.ValueString() {
	case effectAllow:
		return v1.SecurityPolicy_EFFECT_ALLOW
	case effectBlock:
		return v1.SecurityPolicy_EFFECT_BLOCK
	case effectAudit:
		return v1.SecurityPolicy_EFFECT_AUDIT
	default:
		diags.AddAttributeError(attrPath, "Invalid Security Policy Effect", "Supported values are \"allow\", \"block\", and \"audit\".")
		return v1.SecurityPolicy_EFFECT_UNSPECIFIED
	}
}

func effectToString(effect v1.SecurityPolicy_Effect) types.String {
	switch effect {
	case v1.SecurityPolicy_EFFECT_ALLOW:
		return types.StringValue(effectAllow)
	case v1.SecurityPolicy_EFFECT_BLOCK:
		return types.StringValue(effectBlock)
	case v1.SecurityPolicy_EFFECT_AUDIT:
		return types.StringValue(effectAudit)
	default:
		return types.StringNull()
	}
}

func validateFileActionsSet(ctx context.Context, set types.Set, attrPath path.Path) diag.Diagnostics {
	var diags diag.Diagnostics
	var values []string
	diags.Append(set.ElementsAs(ctx, &values, false)...)
	if diags.HasError() {
		return diags
	}
	for _, value := range values {
		switch value {
		case fileActionRead, fileActionWrite:
		default:
			diags.AddAttributeError(attrPath, "Invalid File Policy Action", "Supported values are \"read\" and \"write\".")
		}
	}
	return diags
}

func specModelFromSecurityPolicy(spec *v1.SecurityPolicy_Spec) *SpecModel {
	model := &SpecModel{}
	if spec == nil {
		return model
	}
	model.Executables = executablePolicyModelFromProto(spec.GetExecutables())
	return model
}

func executablePolicyModelFromProto(policy *v1.SecurityPolicy_Spec_ExecutablePolicy) *ExecutablePolicyModel {
	if policy == nil {
		return nil
	}
	model := &ExecutablePolicyModel{
		DefaultEffect: effectToString(policy.GetDefaultEffect()),
	}
	for _, rule := range policy.GetRules() {
		model.Rules = append(model.Rules, ExecutableRuleModel{
			Path:   types.StringValue(rule.GetPath()),
			Effect: effectToString(rule.GetEffect()),
		})
	}
	return model
}

func preserveSpecPlannedInputs(data *SpecModel, planned *SpecModel) {
	if planned.Ports != nil {
		data.Ports = planned.Ports
	}
	if planned.Files != nil {
		data.Files = planned.Files
	}
	if planned.BlockDevices != nil {
		data.BlockDevices = planned.BlockDevices
	}
	if planned.Data != nil {
		data.Data = planned.Data
	}
	if data.Files != nil && planned.Files != nil {
		data.Files.DefaultActions = preserveSet(data.Files.DefaultActions, planned.Files.DefaultActions)
		for i := range data.Files.Rules {
			if i < len(planned.Files.Rules) {
				data.Files.Rules[i].Actions = preserveSet(data.Files.Rules[i].Actions, planned.Files.Rules[i].Actions)
			}
		}
	}
}
