// internal/policycheck/config/types.go
// Defines the configuration struct hierarchy for the policy-gate.toml file.

package config

const ScopeProjectRepo = true


import "regexp"

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
	GoVersion            PolicyVersionConfig              `toml:"go_version"`
	PythonVersion        PolicyVersionConfig              `toml:"python_version"`
	TypescriptVersion    PolicyVersionConfig              `toml:"typescript_version"`
	Runtime              PolicyConfigMetadata             `toml:"-"`
}

// PolicyVersionConfig defines allowed version prefixes for a language environment.
type PolicyVersionConfig struct {
	AllowedPrefixes []string `toml:"allowed_prefixes"`
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
	WarnLOC                 int `toml:"warn_loc"`
	MaxLOC                  int `toml:"max_loc"`
	MildCTXMin              int `toml:"mild_ctx_min"`
	ElevatedCTXMin          int `toml:"elevated_ctx_min"`
	ImmediateRefactorCTXMin int `toml:"immediate_refactor_ctx_min"`
	ErrorCTXMin             int `toml:"error_ctx_min"`
	ErrorCTXAndLOCCTX       int `toml:"error_ctx_and_loc_ctx"`
	ErrorCTXAndLOCLOC       int `toml:"error_ctx_and_loc_loc"`
	NilGuardRepeatWarnCount int `toml:"nil_guard_repeat_warn_count"`
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

// PolicyConfigMetadata holds runtime-only metadata populated after config loading.
type PolicyConfigMetadata struct {
	RepoRoot          string
	ConfigPath        string
	CreatedConfigPath string
	WasCreated        bool
}

// PolicyReviewTarget represents a configuration key that should be reviewed after first-run creation.
type PolicyReviewTarget struct {
	KeyPath string
	Line    int
}
