// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGeneratedTerraformProviderCodeIsCurrent(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Err string
	}

	var got Expectation
	providerDir, err := findProviderDir()
	if err != nil {
		got.Err = err.Error()
	} else {
		cmd := exec.CommandContext(t.Context(), "go", "run", "./tfgen/cmd/generate", "-check")
		cmd.Dir = providerDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			got.Err = fmt.Sprintf("%v\n%s", err, out)
		}
	}

	expected := Expectation{}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("generated Terraform provider code mismatch (-want +got):\n%s", diff)
	}
}
