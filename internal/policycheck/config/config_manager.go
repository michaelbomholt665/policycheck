// internal/policycheck/config/config_manager.go
// Package config manages configuration loading, validation, and defaults for policycheck.
// It defines the central PolicyConfig structure and its sub-components for all checkers.
package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"policycheck/internal/policycheck/utils"
)

// PolicyConfig is the root configuration structure loaded from policy-gate.toml.
type PolicyConfig struct {
	Paths                PolicyPathsConfig                `toml:"paths"`
	FileSize             PolicyFileSizeConfig             `toml:"file_size"`
	FunctionQuality      PolicyFunctionQualityConfig      `toml:"function_quality"`
	Output               PolicyOutputConfig               `toml:"output"`
	SecretLogging        PolicySecretLoggingConfig        `toml:"secret_logging"`
	CLIFormatter         PolicyCLIFormatterConfig         `toml:"cli_formatter"`
	HardcodedRuntimeKnob PolicyHardcodedRuntimeKnobConfig `toml:"hardcoded_runtime_knob"`
	Architecture         PolicyArchitectureConfig         `toml:"architecture"`
	GoVersion            PolicyGoVersionConfig            `toml:"go_version"`
	Hygiene              PolicyHygieneConfig              `toml:"symbol_hygiene"`
	PackageRules         PolicyPackageRulesConfig         `toml:"package_rules"`
	AICompatibility      PolicyAICompatibilityConfig      `toml:"ai_compatibility"`
	ScopeGuard           PolicyScopeGuardConfig           `toml:"scope_guard"`
	RouterImports        PolicyRouterImportsConfig        `toml:"router_imports"`
	Documentation        PolicyDocumentationConfig        `toml:"documentation"`
	CustomRules          []PolicyCustomRule               `toml:"custom_rules"`
	Runtime              PolicyConfigMetadata             `toml:"-"`
}

// PolicyDocumentationConfig defines the file and function documentation rules.
type PolicyDocumentationConfig struct {
	Enabled              bool     `toml:"enabled"`
	Level                string   `toml:"level"` // "loose" or "strict"
	ScanRoots            []string `toml:"scan_roots"`
	GoStyle              string   `toml:"go_style"`         // "google"
	PythonStyle          string   `toml:"python_style"`     // "numpy"
	TypeScriptStyle      string   `toml:"typescript_style"` // "tsdoc"
	EnforceHeaders       bool     `toml:"enforce_headers"`
	EnforceFunctions     bool     `toml:"enforce_functions"`
	RequireShebangPython bool     `toml:"require_shebang_python"`
	PythonShebangRoots   []string `toml:"python_shebang_roots"`
}

// PolicyRouterImportsConfig defines the router import architecture rules.
type PolicyRouterImportsConfig struct {
	Enabled                         bool     `toml:"enabled"`
	BusinessRoots                   []string `toml:"business_roots"`
	AdapterRoots                    []string `toml:"adapter_roots"`
	RouterCoreRoots                 []string `toml:"router_core_roots"`
	RouterBootRoots                 []string `toml:"router_boot_roots"`
	AllowedBusinessImports          []string `toml:"allowed_business_imports"`
	ForbiddenBusinessImportPrefixes []string `toml:"forbidden_business_import_prefixes"`
	ForbiddenAdapterToAdapter       bool     `toml:"forbidden_adapter_to_adapter"`
	Exceptions                      []string `toml:"exceptions"`
}

// PolicyPathsConfig holds all path-related configuration grouped by scan type.
type PolicyPathsConfig struct {
	ProductionRoots                []string          `toml:"production_roots"`
	SecretScanRoots                []string          `toml:"secret_scan_roots"`
	TestScanRoots                  []string          `toml:"test_scan_roots"`
	FileLOCRoots                   []string          `toml:"file_loc_roots"`
	FunctionQualityRoots           []string          `toml:"function_quality_roots"`
	AllowedTestPrefixes            []string          `toml:"allowed_test_prefixes"`
	LOCIgnorePrefixes              []string          `toml:"loc_ignore_prefixes"`
	HardcodedRuntimeKnobScanRoots  []string          `toml:"hardcoded_runtime_knob_scan_roots"`
	HardcodedRuntimeKnobIgnorePath []string          `toml:"hardcoded_runtime_knob_ignore_prefixes"`
	ContractTargets                map[string]string `toml:"contract_targets"`
}

// PolicyFileSizeConfig defines LOC limits and CTX-based penalty parameters.
type PolicyFileSizeConfig struct {
	WarnLOC                   int `toml:"warn_loc"`
	MaxLOC                    int `toml:"max_loc"`
	WarnPenaltyPerCTXFunction int `toml:"warn_penalty_per_ctx_function"`
	MaxPenaltyPerCTXFunction  int `toml:"max_penalty_per_ctx_function"`
	MaxPenaltyCTXThreshold    int `toml:"max_penalty_ctx_threshold"`
	MinWarnLOC                int `toml:"min_warn_loc"`
	MinMaxLOC                 int `toml:"min_max_loc"`
	MinWarnToMaxGap           int `toml:"min_warn_to_max_gap"`
}

// PolicyFunctionQualityConfig defines function-level complexity and LOC thresholds.
type PolicyFunctionQualityConfig struct {
	EnabledLanguages        []string `toml:"enabled_languages"`
	WarnLOC                 int      `toml:"warn_loc"`
	MaxLOC                  int      `toml:"max_loc"`
	GoWarnLOC               int      `toml:"go_warn_loc"`
	GoMaxLOC                int      `toml:"go_max_loc"`
	PythonWarnLOC           int      `toml:"python_warn_loc"`
	PythonMaxLOC            int      `toml:"python_max_loc"`
	TypeScriptWarnLOC       int      `toml:"typescript_warn_loc"`
	TypeScriptMaxLOC        int      `toml:"typescript_max_loc"`
	WarnParameterCount      int      `toml:"warn_parameter_count"`
	MaxParameterCount       int      `toml:"max_parameter_count"`
	MildCTXMin              int      `toml:"mild_ctx_min"`
	ElevatedCTXMin          int      `toml:"elevated_ctx_min"`
	ImmediateRefactorCTXMin int      `toml:"immediate_refactor_ctx_min"`
	ErrorCTXMin             int      `toml:"error_ctx_min"`
	ErrorCTXAndLOCCTX       int      `toml:"error_ctx_and_loc_ctx"`
	ErrorCTXAndLOCLOC       int      `toml:"error_ctx_and_loc_loc"`
	NilGuardRepeatWarnCount int      `toml:"nil_guard_repeat_warn_count"`
}

// PolicyOutputConfig controls summarization of mild complexity warnings.
type PolicyOutputConfig struct {
	MildCTXCompressSummary        bool `toml:"mild_ctx_compress_summary"`
	MildCTXSummaryMinFunctions    int  `toml:"mild_ctx_summary_min_functions"`
	MildCTXPerFileEscalationCount int  `toml:"mild_ctx_per_file_escalation_count"`
	MildCTXPerFileSummaryMinCount int  `toml:"mild_ctx_per_file_summary_min_count"`
}

// PolicySecretLoggingConfig configures the secret detection scanner.
type PolicySecretLoggingConfig struct {
	Keywords                       []string                     `toml:"keywords"`
	LoggerIdentifiers              []string                     `toml:"logger_identifiers"`
	IgnorePathPrefixes             []string                     `toml:"ignore_path_prefixes"`
	AllowedLiteralPatterns         []string                     `toml:"allowed_literal_patterns"`
	BenignHints                    []string                     `toml:"benign_hints"`
	PlaceholderStrings             []string                     `toml:"placeholder_strings"`
	Allowlist                      PolicySecretLoggingAllowlist `toml:"allowlist"`
	Overrides                      map[string]string            `toml:"overrides"`
	CompiledAllowedLiteralPatterns []*regexp.Regexp             `toml:"-"`
}

// PolicySecretLoggingAllowlist holds literal-pattern and pattern-ID allowlists.
type PolicySecretLoggingAllowlist struct {
	LiteralPatterns []string `toml:"literal_patterns"`
	PatternIDs      []string `toml:"pattern_ids"`
}

// PolicyCLIFormatterConfig specifies which files must use the audience-aware formatter.
type PolicyCLIFormatterConfig struct {
	RequiredFiles []string `toml:"required_files"`
}

// PolicyHardcodedRuntimeKnobConfig lists identifier names to flag as hardcoded knobs.
type PolicyHardcodedRuntimeKnobConfig struct {
	Identifiers []string `toml:"identifiers"`
}

// PolicyArchitectureConfig controls enforcement of directory-level architecture rules.
type PolicyArchitectureConfig struct {
	Enforce  bool                      `toml:"enforce"`
	Roots    []PolicyArchitectureRoot  `toml:"roots"`
	Concerns []PolicyArchitectureTopic `toml:"concerns"`
}

// PolicyArchitectureRoot defines a root directory and its allowed/ignored children.
type PolicyArchitectureRoot struct {
	Path            string   `toml:"path"`
	AllowedChildren []string `toml:"allowed_children"`
	IgnoreChildren  []string `toml:"ignore_children"`
}

// PolicyArchitectureTopic maps a conceptual concern to its file locations.
type PolicyArchitectureTopic struct {
	Name          string   `toml:"name"`
	Tags          []string `toml:"tags"`
	Roots         []string `toml:"roots"`
	ConfigPaths   []string `toml:"config_paths"`
	SchemaPaths   []string `toml:"schema_paths"`
	ContractPaths []string `toml:"contract_paths"`
	APIPaths      []string `toml:"api_paths"`
}

// PolicyGoVersionConfig defines allowed Go version prefixes.
type PolicyGoVersionConfig struct {
	AllowedPrefixes []string `toml:"allowed_prefixes"`
}

// PolicyHygieneConfig defines symbol naming and doc style limits.
type PolicyHygieneConfig struct {
	ScanRoots                 []string `toml:"scan_roots"`
	ExcludePrefixes           []string `toml:"exclude_prefixes"`
	MinNameTokens             int      `toml:"min_name_tokens"`
	CrossBackendMinNameTokens int      `toml:"cross_backend_min_name_tokens"`
	ExemptFunctionNames       []string `toml:"exempt_function_names"`
}

// PolicyPackageRulesConfig defines package-level file limits.
type PolicyPackageRulesConfig struct {
	ScanRoots          []string `toml:"scan_roots"`
	ExcludePrefixes    []string `toml:"exclude_prefixes"`
	MaxProductionFiles int      `toml:"max_production_files"`
	MinConcerns        int      `toml:"min_concerns"`
	MaxConcerns        int      `toml:"max_concerns"`
}

// PolicyAICompatibilityConfig defines AI prompt flags required in entry files.
type PolicyAICompatibilityConfig struct {
	RequiredFlags []string `toml:"required_flags"`
}

// PolicyScopeGuardConfig defines forbidden calls for core logic.
type PolicyScopeGuardConfig struct {
	Enabled             bool     `toml:"enabled"`
	Mode                string   `toml:"mode"`
	ForbiddenCalls      []string `toml:"forbidden_calls"`
	AllowedPathPrefixes []string `toml:"allowed_path_prefixes"`
}

const (
	// ScopeGuardModeAllow disables forbidden-call enforcement.
	ScopeGuardModeAllow = "allow"
	// ScopeGuardModeRestrict allows forbidden calls only in approved repo-relative paths.
	ScopeGuardModeRestrict = "restrict"
	// ScopeGuardModeBan disallows forbidden calls in every scanned file.
	ScopeGuardModeBan = "ban"
)

// PolicyCustomRule allows regex-based rules via configuration.
type PolicyCustomRule struct {
	ID       string `toml:"id"`
	Message  string `toml:"message"`
	Pattern  string `toml:"pattern"`
	Severity string `toml:"severity"`
	FileGlob string `toml:"file_glob"`
	Language string `toml:"language"`
	Enabled  bool   `toml:"enabled"`

	CompiledPattern *regexp.Regexp `toml:"-"`
}

// PolicyConfigMetadata holds runtime-only metadata populated after config loading.
type PolicyConfigMetadata struct {
	RepoRoot          string
	ConfigPath        string
	CreatedConfigPath string
	WasCreated        bool
}

// ApplyPolicyConfigDefaults applies default values to all missing fields.
func ApplyPolicyConfigDefaults(cfg *PolicyConfig) error {
	applyDefaultSlice(&cfg.Paths.ProductionRoots, []string{"internal", "cmd"})
	applyDefaultSlice(&cfg.Paths.SecretScanRoots, []string{"internal", "cmd"})
	applyDefaultSlice(&cfg.Paths.TestScanRoots, []string{"cmd", "internal", "."})
	applyDefaultSlice(&cfg.Paths.FileLOCRoots, []string{"internal", "cmd", "test"})
	applyDefaultSlice(&cfg.Paths.FunctionQualityRoots, []string{"internal", "cmd"})
	applyDefaultSlice(&cfg.Paths.AllowedTestPrefixes, []string{"internal/tests/"})
	applyDefaultSlice(&cfg.Paths.LOCIgnorePrefixes, []string{"cmd/policycheck/"})
	applyDefaultSlice(&cfg.Paths.HardcodedRuntimeKnobScanRoots, []string{"internal"})

	applyDefaultSlice(&cfg.GoVersion.AllowedPrefixes, []string{"1.24", "1.25"})
	applyDefaultSlice(&cfg.Hygiene.ScanRoots, []string{"internal", "cmd"})
	applyDefaultSlice(&cfg.Hygiene.ExcludePrefixes, []string{"cmd/policycheck"})
	applyDefaultInt(&cfg.Hygiene.MinNameTokens, 2)
	applyDefaultInt(&cfg.Hygiene.CrossBackendMinNameTokens, 3)
	applyDefaultSlice(&cfg.Hygiene.ExemptFunctionNames, []string{"Close", "Read", "Write"})
	applyDefaultSlice(&cfg.PackageRules.ScanRoots, []string{"cmd", "internal", "test"})
	applyDefaultSlice(&cfg.PackageRules.ExcludePrefixes, []string{})
	applyDefaultInt(&cfg.PackageRules.MaxProductionFiles, 10)
	applyDefaultInt(&cfg.PackageRules.MinConcerns, 1)
	applyDefaultInt(&cfg.PackageRules.MaxConcerns, 2)
	applyDefaultSlice(&cfg.FunctionQuality.EnabledLanguages, []string{"go", "python", "typescript"})
	applyDefaultInt(&cfg.FunctionQuality.WarnParameterCount, 5)
	applyDefaultInt(&cfg.FunctionQuality.MaxParameterCount, 7)
	applyDefaultInt(&cfg.FunctionQuality.GoWarnLOC, 50)
	applyDefaultInt(&cfg.FunctionQuality.GoMaxLOC, 100)
	applyDefaultInt(&cfg.FunctionQuality.PythonWarnLOC, 100)
	applyDefaultInt(&cfg.FunctionQuality.PythonMaxLOC, 150)
	applyDefaultInt(&cfg.FunctionQuality.TypeScriptWarnLOC, 100)
	applyDefaultInt(&cfg.FunctionQuality.TypeScriptMaxLOC, 160)
	applyDefaultInt(&cfg.FunctionQuality.MildCTXMin, 12)
	applyDefaultInt(&cfg.FunctionQuality.ElevatedCTXMin, 14)
	applyDefaultInt(&cfg.FunctionQuality.ImmediateRefactorCTXMin, 16)
	applyDefaultInt(&cfg.FunctionQuality.ErrorCTXMin, 18)
	applyDefaultInt(&cfg.FunctionQuality.ErrorCTXAndLOCCTX, 10)
	applyDefaultSlice(&cfg.SecretLogging.BenignHints, []string{"example", "sample", "placeholder", "dummy", "fake", "fixture", "redacted", "masked"})
	applyDefaultSlice(&cfg.SecretLogging.PlaceholderStrings, []string{"<token>", "<password>", "<secret>", "<api-key>", "changeme", "change_me", "replace_me", "your_token_here"})
	applyDefaultSlice(&cfg.AICompatibility.RequiredFlags, []string{"--ai", "--user"})

	applyDefaultSlice(&cfg.RouterImports.BusinessRoots, []string{"internal/policycheck", "internal/cliwrapper", "internal/ports"})
	applyDefaultSlice(&cfg.RouterImports.AdapterRoots, []string{"internal/adapters"})
	applyDefaultSlice(&cfg.RouterImports.RouterCoreRoots, []string{"internal/router"})
	applyDefaultSlice(&cfg.RouterImports.RouterBootRoots, []string{"internal/app", "internal/router/ext"})
	applyDefaultSlice(&cfg.RouterImports.AllowedBusinessImports, []string{
		"policycheck/internal/ports",
		"policycheck/internal/router",
		"policycheck/internal/router/capabilities",
	})
	applyDefaultSlice(&cfg.RouterImports.ForbiddenBusinessImportPrefixes, []string{
		"policycheck/internal/adapters/",
		"policycheck/internal/router/ext/",
	})
	if !cfg.RouterImports.Enabled {
		cfg.RouterImports.ForbiddenAdapterToAdapter = true
	}

	if cfg.ScopeGuard.Mode == "" {
		cfg.ScopeGuard.Mode = ScopeGuardModeRestrict
	}

	if cfg.Documentation.Level == "" {
		cfg.Documentation.Level = "loose"
	}
	applyDefaultSlice(&cfg.Documentation.ScanRoots, []string{"internal", "cmd", "scripts"})
	if cfg.Documentation.GoStyle == "" {
		cfg.Documentation.GoStyle = "google"
	}
	if cfg.Documentation.PythonStyle == "" {
		cfg.Documentation.PythonStyle = "numpy"
	}
	if cfg.Documentation.TypeScriptStyle == "" {
		cfg.Documentation.TypeScriptStyle = "tsdoc"
	}
	applyDefaultSlice(&cfg.Documentation.PythonShebangRoots, []string{"scripts"})

	applyDefaultSlice(&cfg.ScopeGuard.ForbiddenCalls, []string{
		"os.WriteFile",
		"os.Rename",
		"os.Remove",
		"os.RemoveAll",
		"os.Chmod",
		"os.Chown",
		"os.Mkdir",
		"os.MkdirAll",
	})

	return nil
}

// applyDefaultSlice sets a slice to its default value if it is currently empty.
func applyDefaultSlice(target *[]string, def []string) {
	if len(*target) == 0 {
		*target = def
	}
}

// applyDefaultInt sets an integer to its default value if it is currently zero.
func applyDefaultInt(target *int, def int) {
	if *target == 0 {
		*target = def
	}
}

// ValidatePolicyConfig validates fields and compiles regexes for the configuration.
func ValidatePolicyConfig(cfg *PolicyConfig) error {
	if err := validateFileSizeConfig(&cfg.FileSize); err != nil {
		return err
	}

	if err := compileSecretLoggingPatterns(&cfg.SecretLogging); err != nil {
		return err
	}

	if err := compileCustomRules(cfg.CustomRules); err != nil {
		return err
	}

	if err := validateScopeGuardConfig(&cfg.ScopeGuard); err != nil {
		return fmt.Errorf("scope_guard: %w", err)
	}

	if err := validateRouterImportsConfig(&cfg.RouterImports); err != nil {
		return fmt.Errorf("router_imports: %w", err)
	}

	if err := validateDocumentationConfig(&cfg.Documentation); err != nil {
		return fmt.Errorf("documentation: %w", err)
	}

	return nil
}

// validateFileSizeConfig ensures file size thresholds are numerically consistent.
func validateFileSizeConfig(cfg *PolicyFileSizeConfig) error {
	effectiveMax := cfg.MinMaxLOC
	effectiveWarn := cfg.MinWarnLOC
	gap := cfg.MinWarnToMaxGap
	if effectiveMax < effectiveWarn+gap {
		return fmt.Errorf("file_size.min_max_loc (%d) must be at least min_warn_loc (%d) + min_warn_to_max_gap (%d)", effectiveMax, effectiveWarn, gap)
	}
	return nil
}

// compileSecretLoggingPatterns compiles regexes for both direct and list-based secret allowpatterns.
func compileSecretLoggingPatterns(cfg *PolicySecretLoggingConfig) error {
	totalAllowedPatterns := len(cfg.AllowedLiteralPatterns) + len(cfg.Allowlist.LiteralPatterns)
	cfg.CompiledAllowedLiteralPatterns = make([]*regexp.Regexp, 0, totalAllowedPatterns)

	for i, pattern := range cfg.AllowedLiteralPatterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("secret_logging.allowed_literal_patterns[%d]: invalid pattern: %w", i, err)
		}
		cfg.CompiledAllowedLiteralPatterns = append(cfg.CompiledAllowedLiteralPatterns, compiled)
	}

	for i, pattern := range cfg.Allowlist.LiteralPatterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("secret_logging.allowlist.literal_patterns[%d]: invalid pattern: %w", i, err)
		}
		cfg.CompiledAllowedLiteralPatterns = append(cfg.CompiledAllowedLiteralPatterns, compiled)
	}

	return nil
}

// compileCustomRules compiles regexes for each enabled custom rule.
func compileCustomRules(rules []PolicyCustomRule) error {
	for i := range rules {
		rule := &rules[i]
		if !rule.Enabled {
			continue
		}

		if rule.Severity != "warn" && rule.Severity != "error" {
			return fmt.Errorf("custom_rules[%d] (%s): invalid severity '%s', must be 'warn' or 'error'", i, rule.ID, rule.Severity)
		}

		compiled, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return fmt.Errorf("custom_rules[%d] (%s): invalid pattern: %w", i, rule.ID, err)
		}
		rule.CompiledPattern = compiled
	}
	return nil
}

// validateDocumentationConfig ensures the documentation levels and styles are valid choices.
func validateDocumentationConfig(cfg *PolicyDocumentationConfig) error {
	if !cfg.Enabled {
		return nil
	}

	if cfg.Level != "loose" && cfg.Level != "strict" {
		return fmt.Errorf("invalid level %q, must be \"loose\" or \"strict\"", cfg.Level)
	}

	validGoStyles := map[string]bool{"google": true, "standard": true, "presence_only": true}
	if !validGoStyles[cfg.GoStyle] {
		return fmt.Errorf("invalid go_style %q, allowed choices: google, standard, presence_only", cfg.GoStyle)
	}

	validPythonStyles := map[string]bool{
		"google":           true,
		"numpy":            true,
		"restructuredtext": true,
		"standard":         true,
		"presence_only":    true,
	}
	if !validPythonStyles[cfg.PythonStyle] {
		return fmt.Errorf("invalid python_style %q, allowed choices: google, numpy, restructuredtext, standard, presence_only", cfg.PythonStyle)
	}

	validTypeScriptStyles := map[string]bool{"tsdoc": true, "jsdoc": true, "standard": true, "presence_only": true}
	if !validTypeScriptStyles[cfg.TypeScriptStyle] {
		return fmt.Errorf("invalid typescript_style %q, allowed choices: tsdoc, jsdoc, standard, presence_only", cfg.TypeScriptStyle)
	}

	normalizedShebangRoots := make([]string, 0, len(cfg.PythonShebangRoots))
	for _, root := range cfg.PythonShebangRoots {
		normalizedRoot, err := normalizeDocumentationRoot(root)
		if err != nil {
			return fmt.Errorf("python_shebang_roots: %w", err)
		}
		normalizedShebangRoots = append(normalizedShebangRoots, normalizedRoot)
	}
	cfg.PythonShebangRoots = normalizedShebangRoots

	return nil
}

// normalizeDocumentationRoot validates that a documentation scan root is repo-relative and safe.
func normalizeDocumentationRoot(root string) (string, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return "", fmt.Errorf("must not contain empty values")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("must be repo-relative: %q", root)
	}

	normalizedRoot := utils.NormalizePolicyPath(trimmed)
	if normalizedRoot == "" || normalizedRoot == "." {
		return "", fmt.Errorf("must not point to the repository root: %q", root)
	}
	if normalizedRoot == ".." || strings.HasPrefix(normalizedRoot, "../") {
		return "", fmt.Errorf("must stay within the repository: %q", root)
	}

	return normalizedRoot, nil
}

// validateRouterImportsConfig ensures required roots are non-empty when router enforcement is enabled.
func validateRouterImportsConfig(cfg *PolicyRouterImportsConfig) error {
	if !cfg.Enabled {
		return nil
	}

	if len(cfg.BusinessRoots) == 0 {
		return fmt.Errorf("business_roots must not be empty when enabled")
	}
	if len(cfg.AdapterRoots) == 0 {
		return fmt.Errorf("adapter_roots must not be empty when enabled")
	}
	if len(cfg.RouterCoreRoots) == 0 {
		return fmt.Errorf("router_core_roots must not be empty when enabled")
	}

	return nil
}

// validateScopeGuardConfig ensures the mode is a valid choice and normalizes allowed prefixes.
func validateScopeGuardConfig(cfg *PolicyScopeGuardConfig) error {
	switch cfg.Mode {
	case ScopeGuardModeAllow, ScopeGuardModeRestrict, ScopeGuardModeBan:
	default:
		return fmt.Errorf("invalid mode %q, must be %q, %q, or %q", cfg.Mode, ScopeGuardModeAllow, ScopeGuardModeRestrict, ScopeGuardModeBan)
	}

	normalizedPrefixes := make([]string, 0, len(cfg.AllowedPathPrefixes))
	for _, prefix := range cfg.AllowedPathPrefixes {
		normalizedPrefix, err := normalizeScopeGuardPrefix(prefix)
		if err != nil {
			return err
		}
		normalizedPrefixes = append(normalizedPrefixes, normalizedPrefix)
	}
	cfg.AllowedPathPrefixes = normalizedPrefixes

	return nil
}

// normalizeScopeGuardPrefix validates that a scope guard path prefix is repo-relative and safe.
func normalizeScopeGuardPrefix(prefix string) (string, error) {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return "", fmt.Errorf("allowed_path_prefixes must not contain empty values")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("allowed_path_prefixes must be repo-relative: %q", prefix)
	}

	normalizedPrefix := utils.NormalizePolicyPath(trimmed)
	if normalizedPrefix == "" || normalizedPrefix == "." {
		return "", fmt.Errorf("allowed_path_prefixes must not point to the repository root: %q", prefix)
	}
	if normalizedPrefix == ".." || strings.HasPrefix(normalizedPrefix, "../") {
		return "", fmt.Errorf("allowed_path_prefixes must stay within the repository: %q", prefix)
	}

	return normalizedPrefix, nil
}
