package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	onaclient "github.com/ona/terraform-provider-ona/internal/client"
)

func main() {
	var cfg config
	flag.StringVar(&cfg.host, "host", envDefault("ONA_HOST", onaclient.DefaultHost), "Ona host")
	flag.StringVar(&cfg.token, "token", os.Getenv("ONA_TOKEN"), "Ona personal access token; defaults to ONA_TOKEN")
	flag.StringVar(&cfg.orgID, "org-id", "", "organization ID; defaults to the PAT's authenticated organization")
	flag.StringVar(&cfg.workdir, "workdir", "ona-terraform-import", "directory for the import map, generated HCL, and Terraform state")
	flag.StringVar(&cfg.providerDir, "provider-dir", ".", "Terraform provider source directory")
	flag.StringVar(&cfg.terraform, "terraform", "terraform", "terraform executable")
	flag.Var(&cfg.resourceTypes, "resource-type", "Terraform resource type to import; repeat or comma-separate")
	flag.Var(&cfg.resourceTypes, "resource-kind", "Alias for -resource-type")
	flag.Var(&cfg.resourceIDs, "resource-id", "resource UUID/import ID to import; repeat or comma-separate")
	flag.IntVar(&cfg.terraformParallelism, "terraform-parallelism", 2, "Terraform plan parallelism for API reads")
	flag.BoolVar(&cfg.includeSystemGroups, "include-system-groups", false, "include system-managed/direct-share groups")
	flag.BoolVar(&cfg.skipTerraform, "skip-terraform", false, "only write the import map and import blocks")
	flag.BoolVar(&cfg.skipValidate, "skip-validate", false, "skip terraform validate and production-safe plan check")
	flag.BoolVar(&cfg.refreshImportMap, "refresh-import-map", false, "ignore an existing import-map.json and rediscover resources")
	flag.Parse()

	if strings.TrimSpace(cfg.token) == "" {
		failf("missing token: pass -token or set ONA_TOKEN")
	}
	if err := run(context.Background(), cfg); err != nil {
		failf("%s", err)
	}
}

func run(ctx context.Context, cfg config) error {
	logStepf("connecting to Ona API at %s", cfg.host)
	api, err := onaclient.New(onaclient.Config{
		Host:      cfg.host,
		Token:     cfg.token,
		UserAgent: onaclient.UserAgent + " terraform-import",
	})
	if err != nil {
		return fmt.Errorf("create Ona API client: %w", err)
	}
	cfg.apiBaseURL = api.APIBaseURL

	logStepf("preparing import directory %s", cfg.workdir)
	if err := prepareWorkdir(cfg.workdir); err != nil {
		return err
	}

	importMapPath := filepath.Join(cfg.workdir, importMapFileName)
	inv, reusedImportMap, err := loadOrBuildImportMap(ctx, api, cfg, importMapPath)
	if err != nil {
		return err
	}
	if reusedImportMap {
		inv, err = ensureExternalReferences(ctx, api, cfg, importMapPath, inv)
		if err != nil {
			return err
		}
	}
	importInv, err := selectInventory(inv, cfg)
	if err != nil {
		return err
	}
	logStepf("import map contains %d resources for organization %s", len(inv.Resources), inv.OrganizationID)
	if isResourceSelectionConfigured(cfg) {
		logStepf("selected %d resources for Terraform import", countImportableResources(importInv))
	}
	if err := writeTerraformScaffold(cfg.workdir); err != nil {
		return err
	}
	if err := writeImportBlocks(filepath.Join(cfg.workdir, "imports.tf"), importInv); err != nil {
		return err
	}
	if err := writeReferenceLocals(filepath.Join(cfg.workdir, "references.tf"), importInv); err != nil {
		return err
	}
	logStepf("wrote %s, versions.tf, provider.tf, references.tf, and imports.tf", importMapFileName)
	if cfg.skipTerraform {
		logStepf("skip-terraform enabled; stopping after import-map/import-block generation")
		return nil
	}

	logStepf("building local Terraform provider and Terraform CLI config")
	terraformEnv, err := prepareTerraformProvider(cfg)
	if err != nil {
		return err
	}

	logStepf("running Terraform native config generation")
	generatedPath := filepath.Join(cfg.workdir, "generated.tf")
	if err := runTerraform(cfg, terraformEnv, appendPlanParallelism(cfg, "plan", "-input=false", "-generate-config-out=generated.tf")...); err != nil {
		return fmt.Errorf("terraform native config generation failed: %w", err)
	}
	rawData, err := os.ReadFile(generatedPath)
	if err != nil {
		return fmt.Errorf("read generated config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.workdir, "generated.raw.tf.txt"), rawData, 0644); err != nil {
		return fmt.Errorf("write raw generated config copy: %w", err)
	}

	logStepf("rewriting UUID literals to Terraform references")
	if err := rewriteGeneratedConfig(generatedPath, importInv); err != nil {
		return err
	}
	logStepf("splitting generated Terraform into resource files")
	resourceFiles, err := splitGeneratedConfig(generatedPath, cfg.workdir)
	if err != nil {
		return err
	}
	logStepf("wrote %d resource files", len(resourceFiles))
	logStepf("formatting generated Terraform")
	if err := runTerraform(cfg, terraformEnv, "fmt"); err != nil {
		return fmt.Errorf("terraform fmt failed: %w", err)
	}
	if !cfg.skipValidate {
		logStepf("validating generated Terraform")
		if err := runTerraform(cfg, terraformEnv, "validate"); err != nil {
			return fmt.Errorf("terraform validate failed: %w", err)
		}
		logStepf("checking production plan for no remote mutations")
		if err := validatePlan(cfg, terraformEnv); err != nil {
			return err
		}
	}

	logStepf("import config generation complete: %s", cfg.workdir)
	logStepf("import map: %s", filepath.Join(cfg.workdir, importMapFileName))
	for _, path := range resourceFiles {
		logStepf("resource config: %s", path)
	}
	return nil
}

func loadOrBuildImportMap(ctx context.Context, api *onaclient.Client, cfg config, path string) (inventory, bool, error) {
	if !cfg.refreshImportMap {
		inv, err := readImportMap(path)
		if err == nil {
			logStepf("reusing existing import map %s", path)
			return inv, true, nil
		}
		if !os.IsNotExist(err) {
			return inventory{}, false, err
		}
	}

	if cfg.refreshImportMap {
		logStepf("refresh-import-map enabled; rediscovering resources")
	} else {
		logStepf("no %s found; discovering resources", importMapFileName)
	}
	s, err := collect(ctx, api, cfg)
	if err != nil {
		return inventory{}, false, err
	}
	inv := buildInventory(s)
	if err := writeJSON(path, inv); err != nil {
		return inventory{}, false, err
	}
	logStepf("wrote import map %s", path)
	return inv, false, nil
}

func ensureExternalReferences(ctx context.Context, api *onaclient.Client, cfg config, path string, inv inventory) (inventory, error) {
	inv, changed, err := ensureProjectEnvironmentClassReferenceIDs(ctx, api, inv)
	if err != nil {
		return inventory{}, err
	}
	updated, err := collectExternalReferences(ctx, api, cfg, inv)
	if err != nil {
		return inventory{}, err
	}
	if !changed && reflect.DeepEqual(updated.Resources, inv.Resources) {
		return updated, nil
	}
	if err := writeJSON(path, updated); err != nil {
		return inventory{}, err
	}
	logStepf("updated import map %s with referenced objects", path)
	return updated, nil
}

func logStepf(format string, args ...any) {
	ts := time.Now().Format("15:04:05")
	fmt.Fprintf(os.Stderr, "[%s] %s\n", ts, fmt.Sprintf(format, args...))
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
