// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"testing"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPopulateTeamModel(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		ID            string
		Name          string
		CreatedAt     string
		CreatedAtNull bool
	}
	tests := []struct {
		Name     string
		Input    *v1.Team
		Expected Expectation
	}{
		{
			Name: "maps_team",
			Input: &v1.Team{
				Id:        "team-1",
				Name:      "Platform Engineering",
				CreatedAt: timestamppb.New(time.Date(2026, 7, 27, 12, 30, 0, 123, time.UTC)),
			},
			Expected: Expectation{
				ID:        "team-1",
				Name:      "Platform Engineering",
				CreatedAt: "2026-07-27T12:30:00Z",
			},
		},
		{
			Name:     "maps_missing_created_at_to_null",
			Input:    &v1.Team{Id: "team-2", Name: "Infrastructure"},
			Expected: Expectation{ID: "team-2", Name: "Infrastructure", CreatedAtNull: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var model TeamModel
			populateTeamModel(&model, tc.Input)
			got := Expectation{
				ID:            model.ID.ValueString(),
				Name:          model.Name.ValueString(),
				CreatedAt:     model.CreatedAt.ValueString(),
				CreatedAtNull: model.CreatedAt.IsNull(),
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("populateTeamModel() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPreserveTeamPlannedInputs(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Name string
	}
	tests := []struct {
		Name     string
		Planned  types.String
		Expected Expectation
	}{
		{
			Name:     "preserves_known_planned_name",
			Planned:  types.StringValue("Planned Name"),
			Expected: Expectation{Name: "Planned Name"},
		},
		{
			Name:     "keeps_api_name_for_null_plan",
			Planned:  types.StringNull(),
			Expected: Expectation{Name: "API Name"},
		},
		{
			Name:     "keeps_api_name_for_unknown_plan",
			Planned:  types.StringUnknown(),
			Expected: Expectation{Name: "API Name"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			model := TeamModel{Name: types.StringValue("API Name")}
			preserveTeamPlannedInputs(&model, TeamModel{Name: tc.Planned})
			got := Expectation{Name: model.Name.ValueString()}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("preserveTeamPlannedInputs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
