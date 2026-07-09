// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/tfgen"
)

const (
	providerModulePath = "github.com/gitpod-io/terraform-provider-ona"
	specRelPath        = "tfgen/provider-code-spec.json"
	outputRelPath      = "internal/provider/runner_resource_gen.go"
)

func main() {
	check := flag.Bool("check", false, "verify generated Terraform provider code is up to date")
	flag.Parse()

	if err := run(*check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(check bool) error {
	providerDir, err := findProviderDir()
	if err != nil {
		return err
	}

	specification, err := tfgen.BuildSpecification(v1.File_gitpod_v1_runner_proto)
	if err != nil {
		return err
	}
	specBytes, err := tfgen.MarshalSpecification(specification)
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "ona-tfgen-*")
	if err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tmp)

	specPath := filepath.Join(tmp, "provider-code-spec.json")
	if err := os.WriteFile(specPath, specBytes, 0600); err != nil {
		return fmt.Errorf("write provider code specification: %w", err)
	}

	outDir := filepath.Join(tmp, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create framework generator output directory: %w", err)
	}
	if err := runFrameworkGenerator(providerDir, specPath, outDir); err != nil {
		return err
	}

	generatedPath := filepath.Join(outDir, "runner_resource_gen.go")
	generated, err := os.ReadFile(generatedPath)
	if err != nil {
		return fmt.Errorf("read generated framework resource: %w", err)
	}
	generated, err = formatGenerated(generated)
	if err != nil {
		return err
	}

	if check {
		return checkCurrent(providerDir, specBytes, generated)
	}

	if err := os.WriteFile(filepath.Join(providerDir, specRelPath), specBytes, 0644); err != nil {
		return fmt.Errorf("write provider code specification: %w", err)
	}
	if err := os.WriteFile(filepath.Join(providerDir, outputRelPath), generated, 0644); err != nil {
		return fmt.Errorf("write generated framework resource: %w", err)
	}
	return nil
}

func runFrameworkGenerator(providerDir, specPath, outDir string) error {
	cmd := exec.Command(
		"go",
		"run",
		"github.com/hashicorp/terraform-plugin-codegen-framework/cmd/tfplugingen-framework",
		"generate",
		"resources",
		"--input",
		specPath,
		"--output",
		outDir,
		"--package",
		"provider",
	)
	cmd.Dir = filepath.Join(providerDir, "tools")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("generate Terraform Plugin Framework code: %w\n%s", err, out)
	}
	return nil
}

func formatGenerated(source []byte) ([]byte, error) {
	source = append([]byte(`// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

`), source...)
	formatted, err := format.Source(source)
	if err != nil {
		return nil, fmt.Errorf("format generated framework resource: %w", err)
	}
	return formatted, nil
}

func checkCurrent(providerDir string, specBytes, generated []byte) error {
	existingSpec, err := os.ReadFile(filepath.Join(providerDir, specRelPath))
	if err != nil {
		return fmt.Errorf("read checked-in provider code specification: %w", err)
	}
	if !bytes.Equal(existingSpec, specBytes) {
		return fmt.Errorf("%s is out of date; run `go run ./tfgen/cmd/generate` from the provider repository root", specRelPath)
	}

	existingGenerated, err := os.ReadFile(filepath.Join(providerDir, outputRelPath))
	if err != nil {
		return fmt.Errorf("read checked-in framework resource: %w", err)
	}
	if !bytes.Equal(existingGenerated, generated) {
		return fmt.Errorf("%s is out of date; run `go run ./tfgen/cmd/generate` from the provider repository root", outputRelPath)
	}
	return nil
}

func findProviderDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if isProviderModule(dir) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find terraform-provider-ona module root")
		}
		dir = parent
	}
}

func isProviderModule(dir string) bool {
	goMod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	return err == nil && strings.Contains(string(goMod), "module "+providerModulePath+"\n")
}
