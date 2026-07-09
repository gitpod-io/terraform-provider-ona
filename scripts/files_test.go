package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPrepareWorkdirRemovesGeneratedTerraformState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := map[string]string{
		"import-map.json":          "{}",
		"mapping.json":             "{}",
		"inventory.json":           "{}",
		"projects.tf":              "resource",
		"generated.raw.tf.txt":     "raw",
		"terraformrc":              "rc",
		"terraform.sh":             "wrapper",
		"terraform.tfstate":        "state",
		"terraform.tfstate.backup": "backup",
		".terraform.lock.hcl":      "lock",
		"validation.tfplan":        "plan",
		"plan":                     "saved plan",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, ".terraform"), 0755); err != nil {
		t.Fatalf("create .terraform: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "keepdir"), 0755); err != nil {
		t.Fatalf("create keepdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keepdir", "notes.txt"), []byte("keep"), 0644); err != nil {
		t.Fatalf("write keepdir fixture: %v", err)
	}

	var got struct {
		Err       string
		Files     []string
		BinExists bool
	}
	if err := prepareWorkdir(dir); err != nil {
		got.Err = err.Error()
	} else if entries, err := os.ReadDir(dir); err != nil {
		got.Err = err.Error()
	} else {
		for _, entry := range entries {
			got.Files = append(got.Files, entry.Name())
		}
		_, err := os.Stat(filepath.Join(dir, ".bin"))
		got.BinExists = err == nil
	}

	expected := struct {
		Err       string
		Files     []string
		BinExists bool
	}{
		Files:     []string{".bin", "import-map.json", "keepdir"},
		BinExists: true,
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("prepareWorkdir() mismatch (-want +got):\n%s", diff)
	}
}

func TestWriteImportBlocksSkipsUnsupportedProviderTypes(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result string
		Err    string
	}

	tests := []struct {
		Name      string
		Inventory inventory
		Expected  Expectation
	}{
		{
			Name: "writes_only_registered_provider_resources",
			Inventory: inventory{
				Resources: []inventoryResource{
					{Type: "ona_project", Address: "ona_project.next", ImportID: "project-1", Name: "next"},
					{Type: "ona_runner", Address: "ona_runner.frankfurt", ImportID: "runner-1", Name: "frankfurt"},
					{Type: "ona_environment_class", Address: "ona_environment_class.large", ImportID: "class-1", Name: "large"},
				},
			},
			Expected: Expectation{
				Result: `import {
  to = ona_project.next
  id = "project-1"
}

import {
  to = ona_runner.frankfurt
  id = "runner-1"
}

import {
  to = ona_environment_class.large
  id = "class-1"
}

`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "imports.tf")
			var got Expectation
			if err := writeImportBlocks(path, tc.Inventory); err != nil {
				got.Err = err.Error()
			} else if data, err := os.ReadFile(path); err != nil {
				got.Err = err.Error()
			} else {
				got.Result = string(data)
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("writeImportBlocks() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
