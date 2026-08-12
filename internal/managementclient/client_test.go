// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package managementclient

import (
	"net/http"
	"testing"

	"github.com/gitpod-io/gitpod-sdk-go/v1/v1connect"
	"github.com/google/go-cmp/cmp"
)

func TestNewWithServicesTeamService(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Preserved bool
	}

	teamService := v1connect.NewTeamServiceClient(http.DefaultClient, "https://example.com")
	client := NewWithServices(Services{TeamService: teamService})
	got := Expectation{Preserved: client.TeamService() == teamService}
	expected := Expectation{Preserved: true}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("NewWithServices() TeamService mismatch (-want +got):\n%s", diff)
	}
}
