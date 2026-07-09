package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRewriteGeneratedConfigReferences(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result string
		Err    string
	}

	tests := []struct {
		Name      string
		Input     string
		Inventory inventory
		Expected  Expectation
	}{
		{
			Name: "rewrites_tuple_references_one_per_line",
			Input: `resource "ona_project" "next" {
  prebuild_configuration {
    environment_class_ids = ["class-1", "class-2", "class-3"]
  }
}
`,
			Inventory: inventory{
				Resources: []inventoryResource{
					{Type: "ona_environment_class", Address: "ona_environment_class.frankfurt_large", UUID: "class-1"},
					{Type: "ona_environment_class", Address: "ona_environment_class.gcp_large", UUID: "class-2"},
					{Type: "ona_environment_class", Address: "ona_environment_class.virginia_large", UUID: "class-3"},
				},
			},
			Expected: Expectation{
				Result: `resource "ona_project" "next" {
  prebuild_configuration {
    environment_class_ids = [
      ona_environment_class.frankfurt_large.id,
      ona_environment_class.gcp_large.id,
      ona_environment_class.virginia_large.id,
    ]
  }
}
`,
			},
		},
		{
			Name: "rewrites_project_environment_class_id",
			Input: `resource "ona_project" "next" {
  environment_classes {
    id        = "class-1"
    order     = 0
    prebuilds = true
  }
}
`,
			Inventory: inventory{
				Resources: []inventoryResource{
					{Type: "ona_environment_class", Address: "ona_environment_class.frankfurt_large", UUID: "class-1"},
				},
			},
			Expected: Expectation{
				Result: `resource "ona_project" "next" {
  environment_classes {
    id        = ona_environment_class.frankfurt_large.id
    order     = 0
    prebuilds = true
  }
}
`,
			},
		},
		{
			Name: "rewrites_group_service_account_member_id",
			Input: `resource "ona_group" "platform" {
  member {
    id        = "service-account-1"
    principal = "service_account"
  }
}
`,
			Inventory: inventory{
				Resources: []inventoryResource{
					{Type: "external_service_account", Address: "local.service_accounts.bot", UUID: "service-account-1"},
				},
			},
			Expected: Expectation{
				Result: `resource "ona_group" "platform" {
  member {
    id        = local.service_accounts.bot.id
    principal = "service_account"
  }
}
`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "generated.tf")
			var got Expectation
			if err := os.WriteFile(path, []byte(tc.Input), 0644); err != nil {
				got.Err = err.Error()
			} else if err := rewriteGeneratedConfig(path, tc.Inventory); err != nil {
				got.Err = err.Error()
			} else if data, err := os.ReadFile(path); err != nil {
				got.Err = err.Error()
			} else {
				got.Result = string(data)
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("rewriteGeneratedConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
