// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package warmpool

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestCreateWarmPoolRequest(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result *v1.CreateWarmPoolRequest
		Err    string
	}

	tests := []struct {
		Name     string
		Input    WarmPoolModel
		Expected Expectation
	}{
		{
			Name: "uses_min_and_max_size",
			Input: WarmPoolModel{
				ProjectID:          types.StringValue("project-1"),
				EnvironmentClassID: types.StringValue("class-1"),
				MinSize:            types.Int32Value(0),
				MaxSize:            types.Int32Value(5),
			},
			Expected: Expectation{
				Result: &v1.CreateWarmPoolRequest{
					ProjectId:          "project-1",
					EnvironmentClassId: "class-1",
					MinSize:            ptr[int32](0),
					MaxSize:            ptr[int32](5),
				},
			},
		},
		{
			Name: "rejects_invalid_size_range",
			Input: WarmPoolModel{
				ProjectID:          types.StringValue("project-1"),
				EnvironmentClassID: types.StringValue("class-1"),
				MinSize:            types.Int32Value(6),
				MaxSize:            types.Int32Value(5),
			},
			Expected: Expectation{
				Err: "Invalid Warm Pool Size Range",
			},
		},
		{
			Name: "rejects_missing_project_id",
			Input: WarmPoolModel{
				EnvironmentClassID: types.StringValue("class-1"),
				MinSize:            types.Int32Value(0),
				MaxSize:            types.Int32Value(5),
			},
			Expected: Expectation{
				Err: "Missing Warm Pool Project ID",
			},
		},
		{
			Name: "rejects_unknown_min_size_before_apply",
			Input: WarmPoolModel{
				ProjectID:          types.StringValue("project-1"),
				EnvironmentClassID: types.StringValue("class-1"),
				MinSize:            types.Int32Unknown(),
				MaxSize:            types.Int32Value(5),
			},
			Expected: Expectation{
				Err: "Missing Warm Pool Minimum Size",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, diags := createWarmPoolRequest(tc.Input)
			if diags.HasError() {
				got.Err = diags[0].Summary()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("createWarmPoolRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func ptr[T any](value T) *T {
	return &value
}
