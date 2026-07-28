// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package tfvalue

import (
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// StableComputedString constructs a stable computed resource string.
func StableComputedString(description string) resourceschema.StringAttribute {
	return resourceschema.StringAttribute{
		Computed:            true,
		MarkdownDescription: description,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}
}

// ComputedDataSourceString constructs a computed data-source string.
func ComputedDataSourceString(description string) datasourceschema.StringAttribute {
	return datasourceschema.StringAttribute{
		Computed:            true,
		MarkdownDescription: description,
	}
}
