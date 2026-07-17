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
		spec.Ports = portPolicyFromModel(model.Ports, root.AtName("ports"), &diags)
	}
	if model.Executables != nil {
		spec.Executables = executablePolicyFromModel(model.Executables, root.AtName("executables"), &diags)
	}
	if model.Files != nil {
		spec.Files = filePolicyFromModel(ctx, model.Files, root.AtName("files"), &diags)
	}
	if model.BlockDevices != nil {
		spec.BlockDevices = blockDevicePolicyFromModel(model.BlockDevices, root.AtName("block_devices"), &diags)
	}
	if model.Data != nil {
		spec.Data = dataPolicyFromModel(model.Data, root.AtName("data"), &diags)
	}
	return spec, diags
}

func portPolicyFromModel(model *PortPolicyModel, root path.Path, diags *diag.Diagnostics) *v1.SecurityPolicy_Spec_PortPolicy {
	policy := &v1.SecurityPolicy_Spec_PortPolicy{
		DefaultEffect: effectFromString(model.DefaultEffect, root.AtName("default_effect"), diags),
	}
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
		policy.Rules = append(policy.Rules, &v1.SecurityPolicy_Spec_PortPolicy_Rule{
			Range: &v1.SecurityPolicy_Spec_PortPolicy_Range{
				From: uint32(from),
				To:   uint32(to),
			},
			Effect: effectFromString(rule.Effect, rulePath.AtName("effect"), diags),
		})
	}
	return policy
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

func filePolicyFromModel(ctx context.Context, model *FilePolicyModel, root path.Path, diags *diag.Diagnostics) *v1.SecurityPolicy_Spec_FilePolicy {
	policy := &v1.SecurityPolicy_Spec_FilePolicy{
		DefaultEffect: effectFromString(model.DefaultEffect, root.AtName("default_effect"), diags),
	}
	if isKnownSet(model.DefaultActions) {
		actions, actionDiags := fileActionsFromSet(ctx, model.DefaultActions, root.AtName("default_actions"))
		diags.Append(actionDiags...)
		policy.DefaultActions = actions
	}
	for idx, rule := range model.Rules {
		rulePath := root.AtName("rule").AtListIndex(idx)
		fileRule := &v1.SecurityPolicy_Spec_FilePolicy_Rule{
			Path:   rule.Path.ValueString(),
			Effect: effectFromString(rule.Effect, rulePath.AtName("effect"), diags),
		}
		if isKnownSet(rule.Actions) {
			actions, actionDiags := fileActionsFromSet(ctx, rule.Actions, rulePath.AtName("actions"))
			diags.Append(actionDiags...)
			fileRule.Actions = actions
		}
		policy.Rules = append(policy.Rules, fileRule)
	}
	return policy
}

func blockDevicePolicyFromModel(model *BlockDevicePolicyModel, root path.Path, diags *diag.Diagnostics) *v1.SecurityPolicy_Spec_BlockDevicePolicy {
	return &v1.SecurityPolicy_Spec_BlockDevicePolicy{
		DefaultEffect: effectFromString(model.DefaultEffect, root.AtName("default_effect"), diags),
	}
}

func dataPolicyFromModel(model *DataPolicyModel, root path.Path, diags *diag.Diagnostics) *v1.SecurityPolicy_Spec_DataPolicy {
	policy := &v1.SecurityPolicy_Spec_DataPolicy{
		DefaultEffect: effectFromString(model.DefaultEffect, root.AtName("default_effect"), diags),
	}
	for idx, rule := range model.Rules {
		rulePath := root.AtName("rule").AtListIndex(idx)
		dataRule := &v1.SecurityPolicy_Spec_DataPolicy_Rule{
			Effect: effectFromString(rule.Effect, rulePath.AtName("effect"), diags),
		}
		if rule.Source == nil {
			diags.AddAttributeError(rulePath.AtName("source"), "Missing Data Source", "Set a source block for each data rule.")
		} else {
			dataRule.Source = dataSourceFromModel(rule.Source, rulePath.AtName("source"), diags)
		}
		if rule.Destination == nil {
			diags.AddAttributeError(rulePath.AtName("destination"), "Missing Data Destination", "Set a destination block for each data rule.")
		} else {
			dataRule.Destination = &v1.SecurityPolicy_Spec_DataPolicy_Destination{
				Host: rule.Destination.Host.ValueString(),
			}
		}
		policy.Rules = append(policy.Rules, dataRule)
	}
	return policy
}

func dataSourceFromModel(model *DataSourceModel, root path.Path, diags *diag.Diagnostics) *v1.SecurityPolicy_Spec_DataPolicy_Source {
	hasFile := isKnownString(model.File) && model.File.ValueString() != ""
	hasIntegration := isKnownString(model.Integration) && model.Integration.ValueString() != ""
	if hasFile == hasIntegration {
		diags.AddAttributeError(root, "Invalid Data Source Reference", "Set exactly one of source.file or source.integration.")
	}
	source := &v1.SecurityPolicy_Spec_DataPolicy_Source{
		Selector: model.Selector.ValueString(),
	}
	if hasFile {
		source.Reference = &v1.SecurityPolicy_Spec_DataPolicy_Source_File{File: model.File.ValueString()}
	}
	if hasIntegration {
		source.Reference = &v1.SecurityPolicy_Spec_DataPolicy_Source_Integration{Integration: model.Integration.ValueString()}
	}
	return source
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

func fileActionsFromSet(ctx context.Context, set types.Set, attrPath path.Path) ([]v1.SecurityPolicy_Spec_FilePolicy_Action, diag.Diagnostics) {
	var diags diag.Diagnostics
	var values []string
	diags.Append(set.ElementsAs(ctx, &values, false)...)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]v1.SecurityPolicy_Spec_FilePolicy_Action, 0, len(values))
	for _, value := range values {
		switch value {
		case fileActionRead:
			result = append(result, v1.SecurityPolicy_Spec_FilePolicy_ACTION_READ)
		case fileActionWrite:
			result = append(result, v1.SecurityPolicy_Spec_FilePolicy_ACTION_WRITE)
		default:
			diags.AddAttributeError(attrPath, "Invalid File Policy Action", "Supported values are \"read\" and \"write\".")
		}
	}
	return result, diags
}

func fileActionsToSet(ctx context.Context, actions []v1.SecurityPolicy_Spec_FilePolicy_Action) (types.Set, diag.Diagnostics) {
	values := make([]string, 0, len(actions))
	for _, action := range actions {
		switch action {
		case v1.SecurityPolicy_Spec_FilePolicy_ACTION_READ:
			values = append(values, fileActionRead)
		case v1.SecurityPolicy_Spec_FilePolicy_ACTION_WRITE:
			values = append(values, fileActionWrite)
		}
	}
	return types.SetValueFrom(ctx, types.StringType, values)
}

func specModelFromSecurityPolicy(ctx context.Context, spec *v1.SecurityPolicy_Spec) (*SpecModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := &SpecModel{}
	if spec == nil {
		return model, diags
	}
	model.Ports = portPolicyModelFromProto(spec.GetPorts())
	model.Executables = executablePolicyModelFromProto(spec.GetExecutables())
	model.Files, _ = filePolicyModelFromProto(ctx, spec.GetFiles(), &diags)
	model.BlockDevices = blockDevicePolicyModelFromProto(spec.GetBlockDevices())
	model.Data = dataPolicyModelFromProto(spec.GetData())
	return model, diags
}

func portPolicyModelFromProto(policy *v1.SecurityPolicy_Spec_PortPolicy) *PortPolicyModel {
	if policy == nil {
		return nil
	}
	model := &PortPolicyModel{
		DefaultEffect: effectToString(policy.GetDefaultEffect()),
	}
	for _, rule := range policy.GetRules() {
		model.Rules = append(model.Rules, PortRuleModel{
			RangeFrom: types.Int64Value(int64(rule.GetRange().GetFrom())),
			RangeTo:   types.Int64Value(int64(rule.GetRange().GetTo())),
			Effect:    effectToString(rule.GetEffect()),
		})
	}
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

func filePolicyModelFromProto(ctx context.Context, policy *v1.SecurityPolicy_Spec_FilePolicy, diags *diag.Diagnostics) (*FilePolicyModel, bool) {
	if policy == nil {
		return nil, true
	}
	defaultActions, actionDiags := fileActionsToSet(ctx, policy.GetDefaultActions())
	diags.Append(actionDiags...)
	model := &FilePolicyModel{
		DefaultEffect:  effectToString(policy.GetDefaultEffect()),
		DefaultActions: defaultActions,
	}
	for _, rule := range policy.GetRules() {
		actions, actionDiags := fileActionsToSet(ctx, rule.GetActions())
		diags.Append(actionDiags...)
		model.Rules = append(model.Rules, FileRuleModel{
			Path:    types.StringValue(rule.GetPath()),
			Actions: actions,
			Effect:  effectToString(rule.GetEffect()),
		})
	}
	return model, !diags.HasError()
}

func blockDevicePolicyModelFromProto(policy *v1.SecurityPolicy_Spec_BlockDevicePolicy) *BlockDevicePolicyModel {
	if policy == nil {
		return nil
	}
	return &BlockDevicePolicyModel{
		DefaultEffect: effectToString(policy.GetDefaultEffect()),
	}
}

func dataPolicyModelFromProto(policy *v1.SecurityPolicy_Spec_DataPolicy) *DataPolicyModel {
	if policy == nil {
		return nil
	}
	model := &DataPolicyModel{
		DefaultEffect: effectToString(policy.GetDefaultEffect()),
	}
	for _, rule := range policy.GetRules() {
		model.Rules = append(model.Rules, DataRuleModel{
			Source:      dataSourceModelFromProto(rule.GetSource()),
			Destination: dataDestinationModelFromProto(rule.GetDestination()),
			Effect:      effectToString(rule.GetEffect()),
		})
	}
	return model
}

func dataSourceModelFromProto(source *v1.SecurityPolicy_Spec_DataPolicy_Source) *DataSourceModel {
	if source == nil {
		return nil
	}
	model := &DataSourceModel{
		Selector: stringOptionalValue(source.GetSelector()),
	}
	switch source.GetReference().(type) {
	case *v1.SecurityPolicy_Spec_DataPolicy_Source_File:
		model.File = types.StringValue(source.GetFile())
		model.Integration = types.StringNull()
	case *v1.SecurityPolicy_Spec_DataPolicy_Source_Integration:
		model.File = types.StringNull()
		model.Integration = types.StringValue(source.GetIntegration())
	default:
		model.File = types.StringNull()
		model.Integration = types.StringNull()
	}
	return model
}

func dataDestinationModelFromProto(destination *v1.SecurityPolicy_Spec_DataPolicy_Destination) *DataDestinationModel {
	if destination == nil {
		return nil
	}
	return &DataDestinationModel{
		Host: types.StringValue(destination.GetHost()),
	}
}

func preserveSpecPlannedInputs(data *SpecModel, planned *SpecModel) {
	if data.Files != nil && planned.Files != nil {
		data.Files.DefaultActions = preserveSet(data.Files.DefaultActions, planned.Files.DefaultActions)
		for i := range data.Files.Rules {
			if i < len(planned.Files.Rules) {
				data.Files.Rules[i].Actions = preserveSet(data.Files.Rules[i].Actions, planned.Files.Rules[i].Actions)
			}
		}
	}
}

func stringOptionalValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}
