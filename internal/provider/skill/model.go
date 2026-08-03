// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import "github.com/hashicorp/terraform-plugin-framework/types"

type Model struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Prompt      types.String `tfsdk:"prompt"`
	Command     types.String `tfsdk:"command"`
}
