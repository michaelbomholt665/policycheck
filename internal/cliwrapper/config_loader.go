// internal/cliwrapper/config_loader.go
// Loads wrapper configuration from global and repository-scoped TOML sources.
// Enforces merge ordering so repo config cannot weaken global security policy.
package cliwrapper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	// RepoConfigFilename is the repo-level config file the loader searches for upward.
	RepoConfigFilename = "policy-gate.toml"
	globalConfigDir    = "policycheck"
	globalConfigFile   = "config.toml"
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
		if strictnessErr := ValidateConfigStrictnessOrder(globalCfg, repoCfg); strictnessErr != nil {
			return WrapperLoadResult{}, fmt.Errorf("wrapper config loader: %w", strictnessErr)
		}
	}

	merged = mergeWrapperConfigs(DefaultWrapperConfig(), merged)
	merged, err = NormalizeWrapperConfig(merged)
	if err != nil {
		return WrapperLoadResult{}, fmt.Errorf("wrapper config loader: normalize merged config: %w", err)
	}

	return WrapperLoadResult{
		Merged:     merged,
		GlobalPath: globalPath,
		RepoPath:   repoPath,
	}, nil
}

// loadGlobal reads the global config from l.GlobalConfigPath.
//
// Returns zero-value config and empty path when GlobalConfigPath is empty or
// the file does not exist.
func (l *WrapperConfigLoader) loadGlobal() (WrapperConfig, string, error) {
	if l.GlobalConfigPath == "" {
		return WrapperConfig{}, "", nil
	}
	if !fileExists(l.GlobalConfigPath) {
		return WrapperConfig{}, "", nil
	}

	cfg, err := readWrapperConfigFile(l.GlobalConfigPath)
	if err != nil {
		return WrapperConfig{}, "", err
	}

	return cfg, l.GlobalConfigPath, nil
}

// loadRepo walks upward from l.StartDir until it finds RepoConfigFilename
// or reaches the filesystem root. Returns zero-value config and empty path
// when no file is found.
func (l *WrapperConfigLoader) loadRepo() (WrapperConfig, string, error) {
	dir, err := l.resolveStartDir()
	if err != nil {
		return WrapperConfig{}, "", fmt.Errorf("resolve start dir: %w", err)
	}

	path, err := ResolveRepoConfigPath(dir)
	if err != nil {
		return WrapperConfig{}, "", fmt.Errorf("resolve repo config path: %w", err)
	}
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
	if decodeErr := decodeWrapperConfig(data, &cfg); decodeErr != nil {
		return WrapperConfig{}, fmt.Errorf("parse %s: %w", path, decodeErr)
	}

	if validateErr := ValidateWrapperConfig(cfg); validateErr != nil {
		return WrapperConfig{}, fmt.Errorf("validate %s: %w", path, validateErr)
	}

	normalized, err := NormalizeWrapperConfig(cfg)
	if err != nil {
		return WrapperConfig{}, fmt.Errorf("normalize %s: %w", path, err)
	}

	return normalized, nil
}

// mergeWrapperConfigs applies repo values over global values.
//
// Fields in global that are absent in repo are preserved.
func mergeWrapperConfigs(global, repo WrapperConfig) WrapperConfig {
	merged := global

	merged.Security = mergeSecurityConfig(global.Security, repo.Security)
	merged.Tooling.Gates = mergeToolingGates(global.Tooling.Gates, repo.Tooling.Gates)
	merged.Macros = mergeMacros(global.Macros, repo.Macros)

	merged.UI = mergeUIConfig(global.UI, repo.UI)

	return merged
}

// mergeSecurityConfig applies repo security settings over global security settings.
func mergeSecurityConfig(global, repo WrapperSecurityConfig) WrapperSecurityConfig {
	merged := global

	if len(repo.BlockOn) > 0 {
		merged.BlockOn = cloneStringSlice(repo.BlockOn)
	}
	if len(repo.WarnOn) > 0 {
		merged.WarnOn = cloneStringSlice(repo.WarnOn)
	}
	if len(repo.AllowOn) > 0 {
		merged.AllowOn = cloneStringSlice(repo.AllowOn)
	}
	if repo.OSVMode != "" {
		merged.OSVMode = repo.OSVMode
	}

	return merged
}

// mergeUIConfig applies repo UI settings over global UI settings.
func mergeUIConfig(global, repo WrapperUIConfig) WrapperUIConfig {
	merged := global

	if repo.Color != nil {
		merged.Color = repo.Color
	}

	if repo.Verbose != nil {
		merged.Verbose = repo.Verbose
	}

	return merged
}

// ResolveRepoConfigPath walks upward from startDir to find RepoConfigFilename.
func ResolveRepoConfigPath(startDir string) (string, error) {
	if strings.TrimSpace(startDir) == "" {
		return "", fmt.Errorf("start dir must not be empty")
	}

	return walkUpForFile(startDir, RepoConfigFilename), nil
}

// DefaultGlobalConfigPath returns the machine-global wrapper config path.
func DefaultGlobalConfigPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	return filepath.Join(root, globalConfigDir, globalConfigFile), nil
}

// MarshalWrapperConfig renders a wrapper config as TOML using the documented schema shape.
func MarshalWrapperConfig(cfg WrapperConfig) ([]byte, error) {
	normalized, err := NormalizeWrapperConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal wrapper config: normalize: %w", err)
	}

	return toml.Marshal(encodeWrapperConfig(normalized))
}

// walkUpForFile walks from dir toward the root looking for filename.
//
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

// rawWrapperConfig mirrors the TOML wrapper schema before normalization.
type rawWrapperConfig struct {
	Security rawWrapperSecurityConfig      `toml:"security"`
	Tooling  rawWrapperToolingConfig       `toml:"tooling"`
	Macros   map[string]WrapperMacroConfig `toml:"macros"`
	UI       WrapperUIConfig               `toml:"ui"`
}

// rawWrapperSecurityConfig supports both the new block_on schema and legacy thresholds.
type rawWrapperSecurityConfig struct {
	BlockThreshold string   `toml:"block_threshold,omitempty"`
	BlockOn        []string `toml:"block_on,omitempty"`
	WarnOn         []string `toml:"warn_on,omitempty"`
	AllowOn        []string `toml:"allow_on,omitempty"`
	OSVMode        string   `toml:"osv_mode,omitempty"`
}

// rawWrapperToolingConfig mirrors the named tooling-gate TOML table.
type rawWrapperToolingConfig struct {
	Gates map[string]WrapperToolingGate `toml:"gates"`
}

// decodeWrapperConfig decodes TOML into the shared wrapper config shape.
func decodeWrapperConfig(raw []byte, cfg *WrapperConfig) error {
	var decoded rawWrapperConfig
	if err := toml.Unmarshal(raw, &decoded); err != nil {
		return err
	}

	cfg.Security = WrapperSecurityConfig{
		BlockOn: thresholdToBlockOn(decoded.Security.BlockThreshold, decoded.Security.BlockOn),
		WarnOn:  decoded.Security.WarnOn,
		AllowOn: decoded.Security.AllowOn,
		OSVMode: decoded.Security.OSVMode,
	}
	cfg.Tooling = WrapperToolingConfig{
		Gates: mapToToolingGates(decoded.Tooling.Gates),
	}
	cfg.Macros = mapToMacros(decoded.Macros)
	cfg.UI = decoded.UI

	return nil
}

// encodeWrapperConfig converts the shared wrapper config into TOML-friendly tables.
func encodeWrapperConfig(cfg WrapperConfig) rawWrapperConfig {
	return rawWrapperConfig{
		Security: rawWrapperSecurityConfig{
			BlockOn: cfg.Security.BlockOn,
			WarnOn:  cfg.Security.WarnOn,
			AllowOn: cfg.Security.AllowOn,
			OSVMode: cfg.Security.OSVMode,
		},
		Tooling: rawWrapperToolingConfig{
			Gates: toolingGatesToMap(cfg.Tooling.Gates),
		},
		Macros: macrosToMap(cfg.Macros),
		UI:     cfg.UI,
	}
}

// mapToToolingGates converts named tooling-gate tables into ordered config entries.
func mapToToolingGates(raw map[string]WrapperToolingGate) []WrapperToolingGate {
	if len(raw) == 0 {
		return nil
	}

	gates := make([]WrapperToolingGate, 0, len(raw))
	for name, gate := range raw {
		gate.Name = name
		gates = append(gates, gate)
	}
	sortToolingGates(gates)

	return gates
}

// toolingGatesToMap converts ordered tooling gates into named TOML tables.
func toolingGatesToMap(gates []WrapperToolingGate) map[string]WrapperToolingGate {
	if len(gates) == 0 {
		return nil
	}

	raw := make(map[string]WrapperToolingGate, len(gates))
	for _, gate := range gates {
		encoded := gate
		encoded.Name = ""
		raw[gate.Name] = encoded
	}

	return raw
}

// mapToMacros converts named macro tables into ordered config entries.
func mapToMacros(raw map[string]WrapperMacroConfig) []WrapperMacroConfig {
	if len(raw) == 0 {
		return nil
	}

	macros := make([]WrapperMacroConfig, 0, len(raw))
	for name, macro := range raw {
		macro.Name = name
		macros = append(macros, macro)
	}
	sortMacros(macros)

	return macros
}

// macrosToMap converts ordered macros into named TOML tables.
func macrosToMap(macros []WrapperMacroConfig) map[string]WrapperMacroConfig {
	if len(macros) == 0 {
		return nil
	}

	raw := make(map[string]WrapperMacroConfig, len(macros))
	for _, macro := range macros {
		encoded := macro
		encoded.Name = ""
		raw[macro.Name] = encoded
	}

	return raw
}

// mergeMacros merges macros by name with repo values overriding global values.
func mergeMacros(global, repo []WrapperMacroConfig) []WrapperMacroConfig {
	if len(global) == 0 && len(repo) == 0 {
		return nil
	}

	merged := make(map[string]WrapperMacroConfig, len(global)+len(repo))
	for _, macro := range global {
		merged[macro.Name] = macro
	}
	for _, macro := range repo {
		merged[macro.Name] = macro
	}

	return mapToMacros(merged)
}

// mergeToolingGates merges tooling gates by name with repo values overriding global values.
func mergeToolingGates(global, repo []WrapperToolingGate) []WrapperToolingGate {
	if len(global) == 0 && len(repo) == 0 {
		return nil
	}

	merged := make(map[string]WrapperToolingGate, len(global)+len(repo))
	for _, gate := range global {
		merged[gate.Name] = gate
	}
	for _, gate := range repo {
		merged[gate.Name] = gate
	}

	return mapToToolingGates(merged)
}

// thresholdToBlockOn translates a legacy block_threshold into block_on labels.
func thresholdToBlockOn(blockThreshold string, blockOn []string) []string {
	if len(blockOn) > 0 {
		return blockOn
	}
	if strings.TrimSpace(blockThreshold) == "" {
		return nil
	}

	thresholdSeverity, err := ParseSeverity(blockThreshold)
	if err != nil {
		return []string{blockThreshold}
	}

	levels := make([]string, 0, 5)
	for _, severity := range []Severity{
		SeverityCritical,
		SeverityHigh,
		SeverityModerate,
		SeverityLow,
		SeverityInfo,
	} {
		if SeverityAtLeast(severity, thresholdSeverity) {
			levels = append(levels, CanonicalSeverityLabel(severity))
		}
	}

	return levels
}

// cloneStringSlice copies a string slice when a merge must preserve immutability.
func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)

	return cloned
}
