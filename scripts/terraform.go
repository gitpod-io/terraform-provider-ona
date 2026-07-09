package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func prepareTerraformProvider(cfg config) ([]string, error) {
	providerDir, err := filepath.Abs(cfg.providerDir)
	if err != nil {
		return nil, fmt.Errorf("resolve provider dir: %w", err)
	}
	binDir, err := filepath.Abs(filepath.Join(cfg.workdir, ".bin"))
	if err != nil {
		return nil, fmt.Errorf("resolve provider bin dir: %w", err)
	}
	bin := filepath.Join(binDir, "terraform-provider-ona")

	logStepf("building provider from %s", providerDir)
	cmd := exec.Command("go", "build", "-o", bin, providerDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("build Terraform provider: %w\n%s", err, out)
	}

	terraformRC := filepath.Join(cfg.workdir, "terraformrc")
	contents := fmt.Sprintf(`provider_installation {
  dev_overrides {
    %q = %q
  }

  direct {}
}
`, providerSource, binDir)
	if err := os.WriteFile(terraformRC, []byte(contents), 0644); err != nil {
		return nil, fmt.Errorf("write terraformrc: %w", err)
	}
	if err := writeTerraformWrapper(cfg.workdir); err != nil {
		return nil, err
	}

	env := os.Environ()
	env = append(env,
		"TF_CLI_CONFIG_FILE="+terraformRC,
		"ONA_TOKEN="+cfg.token,
		"ONA_HOST="+cfg.host,
	)
	return env, nil
}

func writeTerraformWrapper(dir string) error {
	path := filepath.Join(dir, "terraform.sh")
	contents := `#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
export TF_CLI_CONFIG_FILE="$script_dir/terraformrc"

if [ -z "${ONA_TOKEN:-}" ]; then
  echo "missing ONA_TOKEN" >&2
  exit 1
fi

exec "${TERRAFORM:-terraform}" -chdir="$script_dir" "$@"
`
	if err := os.WriteFile(path, []byte(contents), 0755); err != nil {
		return fmt.Errorf("write terraform wrapper: %w", err)
	}
	return nil
}

func validatePlan(cfg config, env []string) error {
	planPath := "validation.tfplan"
	err := runTerraformAllowExit2(cfg, env, appendPlanParallelism(cfg, "plan", "-input=false", "-detailed-exitcode", "-out="+planPath)...)
	if err != nil {
		return fmt.Errorf("terraform validation plan failed: %w", err)
	}
	defer func() {
		if err := os.Remove(filepath.Join(cfg.workdir, planPath)); err != nil && !errors.Is(err, os.ErrNotExist) {
			logStepf("remove validation plan: %s", err)
		}
	}()

	cmd := exec.Command(cfg.terraform, "-chdir="+cfg.workdir, "show", "-json", planPath)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("terraform show failed: %w\n%s", err, exitErr.Stderr)
		}
		return fmt.Errorf("terraform show failed: %w", err)
	}
	var plan terraformPlan
	if err := json.Unmarshal(out, &plan); err != nil {
		return fmt.Errorf("parse terraform plan JSON: %w", err)
	}
	var mutations []string
	for _, rc := range plan.ResourceChanges {
		if safePlanChange(rc) {
			continue
		}
		mutations = append(mutations, fmt.Sprintf("%s actions=%s", rc.Address, strings.Join(rc.Change.Actions, ",")))
	}
	if len(mutations) > 0 {
		return fmt.Errorf("generated configuration is not production-noop; Terraform proposed remote mutations:\n%s", strings.Join(mutations, "\n"))
	}
	return nil
}

func appendPlanParallelism(cfg config, args ...string) []string {
	if cfg.terraformParallelism <= 0 {
		return args
	}
	return append(args, "-parallelism="+fmt.Sprint(cfg.terraformParallelism))
}

type terraformPlan struct {
	ResourceChanges []terraformResourceChange `json:"resource_changes"`
}

type terraformResourceChange struct {
	Address   string           `json:"address"`
	Importing *json.RawMessage `json:"importing"`
	Change    struct {
		Actions []string `json:"actions"`
	} `json:"change"`
}

func safePlanChange(change terraformResourceChange) bool {
	if len(change.Change.Actions) == 0 {
		return true
	}
	if len(change.Change.Actions) == 1 && change.Change.Actions[0] == "no-op" {
		return true
	}
	return change.Importing != nil && len(change.Change.Actions) == 1 && change.Change.Actions[0] == "no-op"
}

func runTerraform(cfg config, env []string, args ...string) error {
	logStepf("terraform %s", strings.Join(args, " "))
	cmd := exec.Command(cfg.terraform, append([]string{"-chdir=" + cfg.workdir}, args...)...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", strings.Join(cmd.Args, " "), err, out)
	}
	if len(out) > 0 {
		logStepf("terraform output:\n%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func runTerraformAllowExit2(cfg config, env []string, args ...string) error {
	logStepf("terraform %s", strings.Join(args, " "))
	cmd := exec.Command(cfg.terraform, append([]string{"-chdir=" + cfg.workdir}, args...)...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err == nil {
		if len(out) > 0 {
			logStepf("terraform output:\n%s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		if len(out) > 0 {
			logStepf("terraform output:\n%s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	return fmt.Errorf("%s: %w\n%s", strings.Join(cmd.Args, " "), err, out)
}
