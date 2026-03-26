// internal/cliwrapper/config_loader.go
package cliwrapper

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const (
	// wrapperConfigFilename is the file the loader searches for upward.
	wrapperConfigFilename = "wrapper-gate.toml"
)

// WrapperConfigLoader loads global and repo-scoped wrapper config files and
// merges them into a single WrapperConfig, enforcing the stricter-only rule
// for security thresholds.
//
// WrapperConfigLoader is always instantiated fresh per command; it carries no
// mutable state after Load returns.
type WrapperConfigLoader struct {
	// GlobalConfigPath is the absolute path to the global config file.
	// If empty, the loader skips global config loading.
	GlobalConfigPath string

	// StartDir is the directory from which the upward repo-root search begins.
	// Defaults to the current working directory if empty.
	StartDir string
}

// WrapperLoadResult contains the outcome of a successful Load call.
type WrapperLoadResult struct {
	// Merged is the final merged config after applying repo overrides.
	Merged WrapperConfig
	// GlobalPath is the resolved global config path (empty if not found).
	GlobalPath string
	// RepoPath is the resolved repo config path (empty if not found).
	RepoPath string
}

// Load performs global + repo config resolution, merges the results, and
// validates the merged strictness ordering.
//
// Missing repo config is not an error: Load falls back to global-only
// behaviour. A missing global config is also not fatal if StartDir yields a
// valid repo config.
func (l *WrapperConfigLoader) Load() (WrapperLoadResult, error) {
	globalCfg, globalPath, err := l.loadGlobal()
	if err != nil {
		return WrapperLoadResult{}, fmt.Errorf("wrapper config loader: global: %w", err)
	}

	repoCfg, repoPath, err := l.loadRepo()
	if err != nil {
		return WrapperLoadResult{}, fmt.Errorf("wrapper config loader: repo: %w", err)
	}

	merged := mergeWrapperConfigs(globalCfg, repoCfg)

	if globalPath != "" && repoPath != "" {
		if err := ValidateConfigStrictnessOrder(globalCfg, repoCfg); err != nil {
			return WrapperLoadResult{}, fmt.Errorf("wrapper config loader: %w", err)
		}
	}

	return WrapperLoadResult{
		Merged:     merged,
		GlobalPath: globalPath,
		RepoPath:   repoPath,
	}, nil
}

// loadGlobal reads the global config from l.GlobalConfigPath.
// Returns zero-value config and empty path when GlobalConfigPath is empty or
// the file does not exist.
func (l *WrapperConfigLoader) loadGlobal() (WrapperConfig, string, error) {
	if l.GlobalConfigPath == "" {
		return WrapperConfig{}, "", nil
	}

	cfg, err := readWrapperConfigFile(l.GlobalConfigPath)
	if err != nil {
		return WrapperConfig{}, "", err
	}

	return cfg, l.GlobalConfigPath, nil
}

// loadRepo walks upward from l.StartDir until it finds wrapperConfigFilename
// or reaches the filesystem root. Returns zero-value config and empty path
// when no file is found.
func (l *WrapperConfigLoader) loadRepo() (WrapperConfig, string, error) {
	dir, err := l.resolveStartDir()
	if err != nil {
		return WrapperConfig{}, "", fmt.Errorf("resolve start dir: %w", err)
	}

	path := walkUpForFile(dir, wrapperConfigFilename)
	if path == "" {
		return WrapperConfig{}, "", nil
	}

	cfg, err := readWrapperConfigFile(path)
	if err != nil {
		return WrapperConfig{}, "", err
	}

	return cfg, path, nil
}

// resolveStartDir returns l.StartDir when set, or the working directory.
func (l *WrapperConfigLoader) resolveStartDir() (string, error) {
	if l.StartDir != "" {
		return l.StartDir, nil
	}

	return os.Getwd()
}

// readWrapperConfigFile parses a TOML file into a WrapperConfig and validates
// its structural shape.
func readWrapperConfigFile(path string) (WrapperConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WrapperConfig{}, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg WrapperConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return WrapperConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}

	if err := ValidateWrapperConfig(cfg); err != nil {
		return WrapperConfig{}, fmt.Errorf("validate %s: %w", path, err)
	}

	return cfg, nil
}

// mergeWrapperConfigs applies repo values over global values.
// Fields in global that are absent in repo are preserved.
func mergeWrapperConfigs(global, repo WrapperConfig) WrapperConfig {
	merged := global

	if repo.Security.BlockThreshold != "" {
		merged.Security.BlockThreshold = repo.Security.BlockThreshold
	}

	if len(repo.Tooling.Gates) > 0 {
		merged.Tooling.Gates = repo.Tooling.Gates
	}

	if len(repo.Macros) > 0 {
		merged.Macros = repo.Macros
	}

	merged.UI = mergeUIConfig(global.UI, repo.UI)

	return merged
}

// mergeUIConfig applies repo UI settings over global UI settings.
func mergeUIConfig(global, repo WrapperUIConfig) WrapperUIConfig {
	merged := global

	if repo.Color {
		merged.Color = repo.Color
	}

	if repo.Quiet {
		merged.Quiet = repo.Quiet
	}

	return merged
}

// walkUpForFile walks from dir toward the root looking for filename.
// Returns the absolute path of the first match, or empty string if not found.
func walkUpForFile(dir, filename string) string {
	current := filepath.Clean(dir)

	for {
		candidate := filepath.Join(current, filename)
		if fileExists(candidate) {
			return candidate
		}

		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}

		current = parent
	}
}

// fileExists reports whether path refers to a readable regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}
