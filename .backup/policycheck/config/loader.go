// internal/policycheck/config/loader.go
// Loads and fully prepares a PolicyConfig from disk, applying defaults and compiling patterns.

package config

const ScopeProjectRepo = true


import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"policycheck/internal/policycheck/utils"
)

// LoadPolicyConfig loads policy configuration from file, applies defaults, validates, and compiles patterns.
func LoadPolicyConfig(root, configPath string, allowCreate, lenient bool) (PolicyConfig, error) {
	cfg := DefaultPolicyConfig()
	repoRoot, err := DetectRepoRoot(root)
	if err != nil {
		return PolicyConfig{}, fmt.Errorf("detect repo root: %w", err)
	}

	fullPath := ResolvePolicyConfigPath(repoRoot, configPath)
	createdPath, err := EnsurePolicyConfigFile(repoRoot, configPath, fullPath, allowCreate)
	if err != nil {
		return PolicyConfig{}, fmt.Errorf("ensure policy config %s: %w", fullPath, err)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return PolicyConfig{}, fmt.Errorf("read policy config %s: %w", fullPath, err)
	}

	dec := toml.NewDecoder(strings.NewReader(string(data)))
	if !lenient {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(&cfg); err != nil {
		return PolicyConfig{}, fmt.Errorf("parse policy config %s: %w", fullPath, err)
	}

	ApplyPolicyConfigDefaults(&cfg)
	if err := ValidatePolicyConfig(&cfg); err != nil {
		return PolicyConfig{}, fmt.Errorf("validate policy config %s: %w", fullPath, err)
	}

	cfg.SecretLogging.CompiledAllowedLiteralPatterns = CompileSecretAllowPatterns(append(
		append([]string{}, cfg.SecretLogging.AllowedLiteralPatterns...),
		cfg.SecretLogging.Allowlist.LiteralPatterns...,
	))

	cfg.Runtime = PolicyConfigMetadata{
		RepoRoot:          repoRoot,
		ConfigPath:        fullPath,
		CreatedConfigPath: createdPath,
		WasCreated:        createdPath != "",
	}

	return cfg, nil
}

// ResolvePolicyConfigPath resolves the config file path relative to the repo root.
func ResolvePolicyConfigPath(repoRoot, configPath string) string {
	if filepath.IsAbs(configPath) {
		return configPath
	}
	return filepath.Join(repoRoot, configPath)
}

// DetectRepoRoot traverses upwards from start to find the repository root (goes.mod or .git).
func DetectRepoRoot(start string) (string, error) {
	absStart, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve start path: %w", err)
	}

	info, err := os.Stat(absStart)
	if err != nil {
		return "", fmt.Errorf("stat start path: %w", err)
	}
	dir := absStart
	if !info.IsDir() {
		dir = filepath.Dir(absStart)
	}

	for {
		if utils.PathExists(filepath.Join(dir, "go.mod")) || utils.PathExists(filepath.Join(dir, ".git")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no repository root found (searched up to %s)", absStart)
		}
		dir = parent
	}
}

// CompileSecretAllowPatterns compiles a list of regex patterns for secret allowlisting.
func CompileSecretAllowPatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		compiled = append(compiled, re)
	}
	return compiled
}
