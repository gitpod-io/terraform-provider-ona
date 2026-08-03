// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"time"

	"connectrpc.com/connect"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func timestampString(value *timestamppb.Timestamp) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(value.AsTime().UTC().Format(time.RFC3339))
}

func groupNotFound(err error) bool {
	return connect.CodeOf(err) == connect.CodeNotFound
}
