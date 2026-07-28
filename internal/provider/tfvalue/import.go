// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package tfvalue

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// SplitImportID validates and splits a slash-delimited import identifier.
func SplitImportID(id string, count int, expected string) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	parts := strings.Split(id, "/")
	if len(parts) != count {
		diags.AddError("Invalid Import ID", "Expected import ID format: "+expected+".")
		return nil, diags
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			diags.AddError("Invalid Import ID", "Expected import ID format: "+expected+".")
			return nil, diags
		}
	}
	return parts, diags
}

// SetImportString writes a string import component to a root state attribute.
func SetImportString(ctx context.Context, resp *resource.ImportStateResponse, name, value string) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), types.StringValue(value))...)
}
