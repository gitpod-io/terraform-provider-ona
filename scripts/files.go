package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func prepareWorkdir(dir string) error {
	if strings.TrimSpace(dir) == "" || dir == "." || dir == "/" {
		return fmt.Errorf("refusing unsafe workdir %q", dir)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read workdir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && isTerraformStateFile(entry.Name()) {
			return fmt.Errorf("refusing to clean %s because it contains Terraform state; use a disposable staging directory", dir)
		}
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() && shouldRemoveWorkdirDir(name) {
			if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
				return fmt.Errorf("remove stale %s: %w", name, err)
			}
		}
		if !entry.IsDir() && shouldRemoveWorkdirFile(name) {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				return fmt.Errorf("remove stale %s: %w", name, err)
			}
		}
	}
	if err := os.RemoveAll(filepath.Join(dir, ".bin")); err != nil {
		return fmt.Errorf("clear provider bin dir: %w", err)
	}
	return os.MkdirAll(filepath.Join(dir, ".bin"), 0755)
}

func isTerraformStateFile(name string) bool {
	return name == "terraform.tfstate" || name == "terraform.tfstate.backup"
}

func shouldRemoveWorkdirDir(name string) bool {
	switch name {
	case ".bin", ".terraform":
		return true
	default:
		return false
	}
}

func shouldRemoveWorkdirFile(name string) bool {
	switch name {
	case ".terraform.lock.hcl", "terraformrc", "terraform.sh", "plan":
		return true
	}
	return strings.HasSuffix(name, ".tf") ||
		strings.HasSuffix(name, ".tf.txt") ||
		strings.HasSuffix(name, ".tfplan")
}

func writeTerraformScaffold(dir string) error {
	var versions strings.Builder
	linef(&versions, "terraform {")
	linef(&versions, "  required_version = %s", q(">= 1.14.0"))
	linef(&versions, "  required_providers {")
	linef(&versions, "    ona = {")
	linef(&versions, "      source = %s", q(providerSource))
	linef(&versions, "    }")
	linef(&versions, "  }")
	linef(&versions, "}")
	if err := os.WriteFile(filepath.Join(dir, "versions.tf"), []byte(versions.String()), 0644); err != nil {
		return fmt.Errorf("write versions.tf: %w", err)
	}

	var provider strings.Builder
	linef(&provider, "provider \"ona\" {}")
	if err := os.WriteFile(filepath.Join(dir, "provider.tf"), []byte(provider.String()), 0644); err != nil {
		return fmt.Errorf("write provider.tf: %w", err)
	}
	return nil
}

func linef(out *strings.Builder, format string, args ...any) {
	fmt.Fprintf(out, format+"\n", args...)
}

func q(s string) string {
	return strconv.Quote(s)
}
