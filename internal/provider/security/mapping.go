// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package security

import (
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func validatePolicyModel(data PolicyModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if !data.Name.IsUnknown() && !data.Name.IsNull() {
		name := data.Name.ValueString()
		if name == "" || len(name) > 80 {
			diags.AddAttributeError(path.Root("name"), "Invalid Security Policy Name", "Security policy name must be between 1 and 80 characters.")
		}
	}
	_, specDiags := securityPolicySpecFromModel(data.Spec, path.Root("spec"))
	diags.Append(specDiags...)
	return diags
}

func securityPolicySpecFromModel(model *SpecModel, root path.Path) (*v1.SecurityPolicy_Spec, diag.Diagnostics) {
	var diags diag.Diagnostics
	if model == nil {
		diags.AddAttributeError(root, "Missing Security Policy Spec", "Set a spec block before creating an Ona security policy.")
		return nil, diags
	}
	if model.Ports != nil {
		diags.AddAttributeError(root.AtName("ports"), "Unsupported Security Policy Section", "The public Ona API client does not currently expose port security policies.")
	}
	if model.Files != nil {
		diags.AddAttributeError(root.AtName("files"), "Unsupported Security Policy Section", "The public Ona API client does not currently expose file security policies.")
	}
	if model.BlockDevices != nil {
		diags.AddAttributeError(root.AtName("block_devices"), "Unsupported Security Policy Section", "The public Ona API client does not currently expose block device security policies.")
	}
	if model.Data != nil {
		diags.AddAttributeError(root.AtName("data"), "Unsupported Security Policy Section", "The public Ona API client does not currently expose data security policies.")
	}

	spec := &v1.SecurityPolicy_Spec{}
	if model.Executables != nil {
		spec.Executables = executablePolicyFromModel(model.Executables, root.AtName("executables"), &diags)
	}
	return spec, diags
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
