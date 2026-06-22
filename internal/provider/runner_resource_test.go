// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	onaclient "github.com/ona/terraform-provider-ona/internal/client"
)

func TestAccRunnerResourceImport(t *testing.T) {
	server := newRunnerAPIServer(t)
	defer server.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:        testAccRunnerResourceConfig(server.URL),
				ResourceName:  "ona_runner.test",
				ImportState:   true,
				ImportStateId: "runner-1",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 imported resource; got %d", len(states))
					}
					if got := states[0].ID; got != "runner-1" {
						return fmt.Errorf("imported state ID = %q, want runner-1", got)
					}
					if got := states[0].Attributes["id"]; got != "runner-1" {
						return fmt.Errorf("imported attribute id = %q, want runner-1", got)
					}
					if got := states[0].Attributes["name"]; got != "Frankfurt Runner" {
						return fmt.Errorf("imported attribute name = %q, want Frankfurt Runner", got)
					}
					return nil
				},
			},
		},
	})
}

func TestRunnerResourceFindRunner(t *testing.T) {
	t.Parallel()

	server := newRunnerAPIServer(t)
	defer server.Close()

	api, err := onaclient.New(onaclient.Config{
		Host:  server.URL,
		Token: "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}

	r := &RunnerResource{client: api}
	got, err := r.findRunner(t.Context(), "runner-2")
	if err != nil {
		t.Fatal(err)
	}
	if got.GetRunnerId() != "runner-2" {
		t.Fatalf("runner ID = %q, want runner-2", got.GetRunnerId())
	}
	if got.GetName() != "London Runner" {
		t.Fatalf("runner name = %q, want London Runner", got.GetName())
	}

	missing, err := r.findRunner(t.Context(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("missing runner = %#v, want nil", missing)
	}
}

func testAccRunnerResourceConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

resource "ona_runner" "test" {}
`, host)
}

func newRunnerAPIServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/gitpod.v1.RunnerService/ListRunners" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want Bearer test-token", got)
		}

		var req onaclient.ListRunnersRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		token := ""
		if req.Pagination != nil {
			token = req.Pagination.Token
		}
		switch token {
		case "":
			if err := json.NewEncoder(w).Encode(onaclient.ListRunnersResponse{
				Runners: []*onaclient.Runner{
					{RunnerID: "runner-1", Name: "Frankfurt Runner"},
				},
				Pagination: &onaclient.PaginationResponse{NextToken: "next"},
			}); err != nil {
				t.Errorf("encode response: %v", err)
			}
		case "next":
			if err := json.NewEncoder(w).Encode(onaclient.ListRunnersResponse{
				Runners: []*onaclient.Runner{
					{RunnerID: "runner-2", Name: "London Runner"},
				},
				Pagination: &onaclient.PaginationResponse{},
			}); err != nil {
				t.Errorf("encode response: %v", err)
			}
		default:
			t.Errorf("pagination token = %q, want empty or next", token)
			http.Error(w, "unexpected pagination token", http.StatusBadRequest)
		}
	}))
}
