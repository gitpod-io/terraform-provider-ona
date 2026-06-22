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
		Files:     []string{".bin", "import-map.json"},
		BinExists: true,
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("prepareWorkdir() mismatch (-want +got):\n%s", diff)
	}
}
