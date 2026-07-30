// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

type replaceWhenCommandRemoved struct{}

func (replaceWhenCommandRemoved) Description(context.Context) string {
	return "Replacing a configured command with null replaces the skill because the Ona API cannot clear stored command values."
}

func (m replaceWhenCommandRemoved) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (replaceWhenCommandRemoved) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || req.StateValue.IsNull() || req.StateValue.IsUnknown() || req.PlanValue.IsUnknown() {
		return
	}
	if req.PlanValue.IsNull() {
		resp.RequiresReplace = true
	}
}
