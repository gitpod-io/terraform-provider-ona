package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func prepareWorkdir(dir string) error {
	if strings.TrimSpace(dir) == "" || dir == "." || dir == "/" {
		return fmt.Errorf("refusing unsafe workdir %q", dir)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".bin"), 0755); err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read workdir: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == importMapFileName {
			continue
		}
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

func shouldRemoveWorkdirDir(name string) bool {
	switch name {
	case ".bin", ".terraform":
		return true
	default:
		return false
	}
}

func shouldRemoveWorkdirFile(name string) bool {
	if name == importMapFileName {
		return false
	}
	switch name {
	case "inventory.json", "mapping.json":
		return true
	case ".terraform.lock.hcl", "terraform.tfstate", "terraform.tfstate.backup", "terraformrc", "terraform.sh", "plan":
		return true
	}
	return strings.HasSuffix(name, ".tf") ||
		strings.HasSuffix(name, ".tf.txt") ||
		strings.HasSuffix(name, ".tfplan")
}

func readImportMap(path string) (inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return inventory{}, err
	}
	var inv inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		return inventory{}, fmt.Errorf("read import map %s: %w", path, err)
	}
	if inv.OrganizationID == "" {
		return inventory{}, fmt.Errorf("read import map %s: missing organization_id", path)
	}
	return inv, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeTerraformScaffold(dir string) error {
	var versions strings.Builder
	linef(&versions, "terraform {")
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

func writeImportBlocks(path string, inv inventory) error {
	var out strings.Builder
	for _, r := range inv.Resources {
		if r.ImportID == "" {
			continue
		}
		if !isProviderImportableResource(r) {
			linef(&out, "# skipped %s %s: provider does not register this resource type yet", r.Type, r.Name)
			linef(&out, "")
			continue
		}
		if r.SkipReason != "" {
			linef(&out, "# skipped %s %s: %s", r.Type, r.Name, r.SkipReason)
			linef(&out, "")
			continue
		}
		linef(&out, "import {")
		linef(&out, "  to = %s", r.Address)
		linef(&out, "  id = %s", q(r.ImportID))
		linef(&out, "}")
		linef(&out, "")
	}
	return os.WriteFile(path, []byte(out.String()), 0644)
}

func writeReferenceLocals(path string, inv inventory) error {
	serviceAccounts := externalResources(inv, "external_service_account")
	if len(serviceAccounts) == 0 {
		return nil
	}

	var out strings.Builder
	linef(&out, "locals {")
	writeLocalObjectMap(&out, "service_accounts", serviceAccounts)
	linef(&out, "}")
	return os.WriteFile(path, []byte(out.String()), 0644)
}

func externalResources(inv inventory, typ string) []inventoryResource {
	var result []inventoryResource
	for _, r := range inv.Resources {
		if r.Type == typ && r.UUID != "" {
			result = append(result, r)
		}
	}
	return result
}

func writeLocalObjectMap(out *strings.Builder, name string, resources []inventoryResource) {
	if len(resources) == 0 {
		return
	}
	linef(out, "  %s = {", name)
	for _, r := range resources {
		label := r.Address[strings.LastIndex(r.Address, ".")+1:]
		linef(out, "    %s = {", label)
		linef(out, "      id   = %s", q(r.UUID))
		linef(out, "      name = %s", q(r.Name))
		var keys []string
		for key := range r.References {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := r.References[key]
			linef(out, "      %s = %s", key, q(value))
		}
		linef(out, "    }")
	}
	linef(out, "  }")
}

func linef(out *strings.Builder, format string, args ...any) {
	fmt.Fprintf(out, format+"\n", args...)
}

func q(s string) string {
	return strconv.Quote(s)
}
