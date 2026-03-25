// internal/policycheck/config/template.go
// Manages the embedded default config template and first-run file creation.

package config

const ScopeProjectRepo = true


import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed policy_gate_default.toml
var defaultPolicyGateTemplate string

// DefaultPolicyConfigPath is the default config file name resolved relative to the repo root.
const DefaultPolicyConfigPath = "policy-gate.toml"

// EnsurePolicyConfigFile creates the config file from the embedded template if it does not exist.
// Returns the path to the created file, or empty string if the file already existed.
func EnsurePolicyConfigFile(repoRoot, configPath, fullPath string, allowCreate bool) (string, error) {
	if _, err := os.Stat(fullPath); err == nil {
		return "", nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat config: %w", err)
	}
	if !allowCreate {
		return "", fmt.Errorf("config file is missing and auto-create is disabled")
	}

	targetPath := fullPath
	if configPath == DefaultPolicyConfigPath && !filepath.IsAbs(configPath) {
		targetPath = filepath.Join(repoRoot, DefaultPolicyConfigPath)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(targetPath, []byte(defaultPolicyGateTemplate), 0o644); err != nil {
		return "", fmt.Errorf("write default config: %w", err)
	}
	return targetPath, nil
}

// DefaultPolicyReviewTargets returns the list of configuration keys to highlight after first-run.
func DefaultPolicyReviewTargets() []PolicyReviewTarget {
	keys := []string{
		"production_roots",
		"secret_scan_roots",
		"function_quality_roots",
		"required_files",
	}
	keyPaths := map[string]string{
		"production_roots":       "paths.production_roots",
		"secret_scan_roots":      "paths.secret_scan_roots",
		"function_quality_roots": "paths.function_quality_roots",
		"required_files":         "cli_formatter.required_files",
	}

	targets := make([]PolicyReviewTarget, 0, len(keys))
	for _, key := range keys {
		targets = append(targets, PolicyReviewTarget{
			KeyPath: keyPaths[key],
			Line:    findTemplateKeyLine(defaultPolicyGateTemplate, key),
		})
	}
	return targets
}

// FormatPolicyReviewTargets returns formatted hint strings for each review-worthy config key.
func FormatPolicyReviewTargets(configPath string) []string {
	targets := DefaultPolicyReviewTargets()
	lines := make([]string, 0, len(targets))
	for _, target := range targets {
		lines = append(lines, "  - "+filepath.Base(configPath)+":"+strconv.Itoa(target.Line)+"     "+target.KeyPath)
	}
	return lines
}

// findTemplateKeyLine returns the 1-indexed line number where key is first defined in the template.
func findTemplateKeyLine(templateText, key string) int {
	lines := strings.Split(templateText, "\n")
	prefix := key + " = "
	for idx, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return idx + 1
		}
	}
	return 1
}
