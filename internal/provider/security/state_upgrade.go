// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package security

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type policyModelV0 struct {
	ID             types.String       `tfsdk:"id"`
	OrganizationID types.String       `tfsdk:"organization_id"`
	Name           types.String       `tfsdk:"name"`
	CreatedAt      types.String       `tfsdk:"created_at"`
	UpdatedAt      types.String       `tfsdk:"updated_at"`
	Spec           *policySpecModelV0 `tfsdk:"spec"`
}

type policyModelV1 struct {
	ID             types.String       `tfsdk:"id"`
	OrganizationID types.String       `tfsdk:"organization_id"`
	Name           types.String       `tfsdk:"name"`
	CreatedAt      types.String       `tfsdk:"created_at"`
	UpdatedAt      types.String       `tfsdk:"updated_at"`
	Spec           *policySpecModelV1 `tfsdk:"spec"`
}

type policySpecModelV1 struct {
	Executables *ExecutablePolicyModel `tfsdk:"executables"`
}

type policySpecModelV0 struct {
	Ports        *portPolicyModelV0        `tfsdk:"ports"`
	Executables  *ExecutablePolicyModel    `tfsdk:"executables"`
	Files        *filePolicyModelV0        `tfsdk:"files"`
	BlockDevices *blockDevicePolicyModelV0 `tfsdk:"block_devices"`
	Data         *dataPolicyModelV0        `tfsdk:"data"`
}

type portPolicyModelV0 struct {
	DefaultEffect types.String      `tfsdk:"default_effect"`
	Rules         []portRuleModelV0 `tfsdk:"rule"`
}

type portRuleModelV0 struct {
	RangeFrom types.Int64  `tfsdk:"range_from"`
	RangeTo   types.Int64  `tfsdk:"range_to"`
	Effect    types.String `tfsdk:"effect"`
}

type filePolicyModelV0 struct {
	DefaultEffect  types.String      `tfsdk:"default_effect"`
	DefaultActions types.Set         `tfsdk:"default_actions"`
	Rules          []fileRuleModelV0 `tfsdk:"rule"`
}

type fileRuleModelV0 struct {
	Path    types.String `tfsdk:"path"`
	Actions types.Set    `tfsdk:"actions"`
	Effect  types.String `tfsdk:"effect"`
}

type blockDevicePolicyModelV0 struct {
	DefaultEffect types.String `tfsdk:"default_effect"`
}

type dataPolicyModelV0 struct {
	DefaultEffect types.String      `tfsdk:"default_effect"`
	Rules         []dataRuleModelV0 `tfsdk:"rule"`
}

type dataRuleModelV0 struct {
	Source      *dataSourceModelV0      `tfsdk:"source"`
	Destination *dataDestinationModelV0 `tfsdk:"destination"`
	Effect      types.String            `tfsdk:"effect"`
}

type dataSourceModelV0 struct {
	File        types.String `tfsdk:"file"`
	Integration types.String `tfsdk:"integration"`
	Selector    types.String `tfsdk:"selector"`
}

type dataDestinationModelV0 struct {
	Host types.String `tfsdk:"host"`
}

func (r *PolicyResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	v0Schema := policyResourceSchemaWithSpec(0, policySpecBlockV0())
	v1Schema := policyResourceSchemaWithSpec(1, policySpecBlockV1())

	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &v0Schema,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var prior policyModelV0
				resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
				if resp.Diagnostics.HasError() {
					return
				}

				upgraded := newPolicyModel(prior.ID, prior.OrganizationID, prior.Name, prior.CreatedAt, prior.UpdatedAt)
				if prior.Spec != nil {
					upgraded.Spec = &SpecModel{Executables: prior.Spec.Executables}
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, &upgraded)...)
			},
		},
		1: {
			PriorSchema: &v1Schema,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var prior policyModelV1
				resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
				if resp.Diagnostics.HasError() {
					return
				}

				upgraded := newPolicyModel(prior.ID, prior.OrganizationID, prior.Name, prior.CreatedAt, prior.UpdatedAt)
				if prior.Spec != nil {
					upgraded.Spec = &SpecModel{Executables: prior.Spec.Executables}
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, &upgraded)...)
			},
		},
	}
}

func newPolicyModel(id, organizationID, name, createdAt, updatedAt types.String) PolicyModel {
	return PolicyModel{
		ID:             id,
		OrganizationID: organizationID,
		Name:           name,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}

func policySpecBlockV1() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		Blocks: map[string]resourceschema.Block{
			"executables": executablePolicyBlockPrior(),
		},
	}
}

func policySpecBlockV0() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		Blocks: map[string]resourceschema.Block{
			"ports":         portPolicyBlockV0(),
			"executables":   executablePolicyBlockPrior(),
			"files":         filePolicyBlockV0(),
			"block_devices": blockDevicePolicyBlockV0(),
			"data":          dataPolicyBlockV0(),
		},
	}
}

func portPolicyBlockV0() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		Attributes: map[string]resourceschema.Attribute{
			"default_effect": effectAttribute("Default port access effect."),
		},
		Blocks: map[string]resourceschema.Block{
			"rule": resourceschema.ListNestedBlock{
				NestedObject: resourceschema.NestedBlockObject{
					Attributes: map[string]resourceschema.Attribute{
						"range_from": resourceschema.Int64Attribute{Required: true},
						"range_to":   resourceschema.Int64Attribute{Required: true},
						"effect":     effectAttribute("Effect for this port range."),
					},
				},
			},
		},
	}
}

func filePolicyBlockV0() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		Attributes: map[string]resourceschema.Attribute{
			"default_effect": effectAttribute("Default file access effect."),
			"default_actions": resourceschema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
		},
		Blocks: map[string]resourceschema.Block{
			"rule": resourceschema.ListNestedBlock{
				NestedObject: resourceschema.NestedBlockObject{
					Attributes: map[string]resourceschema.Attribute{
						"path": resourceschema.StringAttribute{Required: true},
						"actions": resourceschema.SetAttribute{
							Optional:    true,
							Computed:    true,
							ElementType: types.StringType,
						},
						"effect": effectAttribute("Effect for this file path."),
					},
				},
			},
		},
	}
}

func blockDevicePolicyBlockV0() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		Attributes: map[string]resourceschema.Attribute{
			"default_effect": effectAttribute("Default block device access effect."),
		},
	}
}

func dataPolicyBlockV0() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		Attributes: map[string]resourceschema.Attribute{
			"default_effect": effectAttribute("Default data flow effect."),
		},
		Blocks: map[string]resourceschema.Block{
			"rule": resourceschema.ListNestedBlock{
				NestedObject: resourceschema.NestedBlockObject{
					Attributes: map[string]resourceschema.Attribute{
						"effect": effectAttribute("Effect for this data flow."),
					},
					Blocks: map[string]resourceschema.Block{
						"source": resourceschema.SingleNestedBlock{
							Attributes: map[string]resourceschema.Attribute{
								"file":        resourceschema.StringAttribute{Optional: true},
								"integration": resourceschema.StringAttribute{Optional: true},
								"selector":    resourceschema.StringAttribute{Optional: true},
							},
						},
						"destination": resourceschema.SingleNestedBlock{
							Attributes: map[string]resourceschema.Attribute{
								"host": resourceschema.StringAttribute{Required: true},
							},
						},
					},
				},
			},
		},
	}
}
