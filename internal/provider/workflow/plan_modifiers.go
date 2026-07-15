// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

type durationSemanticEqualityModifier struct{}

func (durationSemanticEqualityModifier) Description(context.Context) string {
	return "Treats equivalent Go duration strings as equal."
}

func (m durationSemanticEqualityModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (durationSemanticEqualityModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() || req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	planned, plannedErr := parseDuration(req.PlanValue.ValueString())
	state, stateErr := parseDuration(req.StateValue.ValueString())
	if plannedErr == nil && stateErr == nil && planned == state {
		resp.PlanValue = req.StateValue
	}
}
