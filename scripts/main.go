package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	var cfg config
	flag.StringVar(&cfg.host, "host", envDefault("ONA_HOST", defaultOnaHost), "Ona host")
	flag.StringVar(&cfg.token, "token", os.Getenv("ONA_TOKEN"), "Ona API token; defaults to ONA_TOKEN")
	flag.StringVar(&cfg.queryFile, "query-file", "", "path to the .tfquery.hcl configuration to run")
	flag.StringVar(&cfg.workdir, "workdir", "ona-terraform-import", "staging directory for generated Terraform configuration")
	flag.StringVar(&cfg.providerDir, "provider-dir", ".", "Terraform provider source directory")
	flag.StringVar(&cfg.terraform, "terraform", "terraform", "terraform executable")
	flag.IntVar(&cfg.terraformParallelism, "terraform-parallelism", 2, "Terraform plan parallelism for validation reads")
	flag.BoolVar(&cfg.skipValidate, "skip-validate", false, "skip terraform validate and production-safe plan check")
	flag.Parse()

	if strings.TrimSpace(cfg.token) == "" {
		failf("missing token: pass -token or set ONA_TOKEN")
	}
	if strings.TrimSpace(cfg.queryFile) == "" {
		failf("missing query file: pass -query-file with a .tfquery.hcl configuration")
	}
	if err := run(cfg); err != nil {
		failf("%s", err)
	}
}

func run(cfg config) error {
	queryData, err := os.ReadFile(cfg.queryFile)
	if err != nil {
		return fmt.Errorf("read query configuration %s: %w", cfg.queryFile, err)
	}

	logStepf("preparing Query staging directory %s", cfg.workdir)
	if err := prepareWorkdir(cfg.workdir); err != nil {
		return err
	}
	if err := writeTerraformScaffold(cfg.workdir); err != nil {
		return err
	}
	queryPath := filepath.Join(cfg.workdir, "query.tfquery.hcl")
	if err := os.WriteFile(queryPath, queryData, 0644); err != nil {
		return fmt.Errorf("write query configuration: %w", err)
	}
	logStepf("staged Query configuration %s", queryPath)

	logStepf("building local Terraform provider and Terraform CLI config")
	terraformEnv, err := prepareTerraformProvider(cfg)
	if err != nil {
		return err
	}

	logStepf("discovering resources with Terraform Query")
	generatedPath := filepath.Join(cfg.workdir, "generated.tf")
	if err := runTerraform(cfg, terraformEnv, "query", "-generate-config-out=generated.tf"); err != nil {
		return fmt.Errorf("terraform query config generation failed: %w", err)
	}
	rawData, err := os.ReadFile(generatedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("terraform query did not generate configuration; check that the query matched resources")
		}
		return fmt.Errorf("read generated config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.workdir, "generated.raw.tf.txt"), rawData, 0644); err != nil {
		return fmt.Errorf("write raw generated config copy: %w", err)
	}

	logStepf("rewriting discovered identity literals to Terraform references")
	if err := rewriteGeneratedConfig(generatedPath); err != nil {
		return err
	}
	logStepf("splitting generated Terraform by resource type")
	resourceFiles, err := splitGeneratedConfig(generatedPath, cfg.workdir)
	if err != nil {
		return err
	}
	logStepf("wrote %d generated Terraform files", len(resourceFiles))

	logStepf("formatting generated Terraform")
	if err := runTerraform(cfg, terraformEnv, "fmt"); err != nil {
		return fmt.Errorf("terraform fmt failed: %w", err)
	}
	if !cfg.skipValidate {
		logStepf("validating generated Terraform")
		if err := runTerraform(cfg, terraformEnv, "validate"); err != nil {
			return fmt.Errorf("terraform validate failed: %w", err)
		}
		logStepf("checking the import plan for no remote mutations")
		if err := validatePlan(cfg, terraformEnv); err != nil {
			return err
		}
	}

	logStepf("Query post-processing complete: %s", cfg.workdir)
	for _, path := range resourceFiles {
		logStepf("generated config: %s", path)
	}
	return nil
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
