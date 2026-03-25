// internal/policycheck/config/validation.go
// Validates and applies defaults to a loaded PolicyConfig.

package config

const ScopeProjectRepo = true


import (
	"fmt"
	"regexp"
	"strings"

	"policycheck/internal/policycheck/utils"
)

// ApplyPolicyConfigDefaults fills in zero-valued config fields with their default values.
func ApplyPolicyConfigDefaults(cfg *PolicyConfig) {
	defaults := DefaultPolicyConfig()
	applyPathDefaults(cfg, defaults)
	applyFileSizeDefaults(cfg, defaults)
	applyFunctionQualityDefaults(cfg, defaults)
	applyOutputDefaults(cfg, defaults)
	applySecretLoggingDefaults(cfg, defaults)
	applyMiscDefaults(cfg, defaults)
}

// applyPathDefaults fills in zero-valued path config fields with their default values.
func applyPathDefaults(cfg *PolicyConfig, defaults PolicyConfig) {
	applyDefaultSlice(&cfg.Paths.ProductionRoots, defaults.Paths.ProductionRoots)
	applyDefaultSlice(&cfg.Paths.SecretScanRoots, defaults.Paths.SecretScanRoots)
	applyDefaultSlice(&cfg.Paths.TestScanRoots, defaults.Paths.TestScanRoots)
	applyDefaultSlice(&cfg.Paths.FileLOCRoots, defaults.Paths.FileLOCRoots)
	applyDefaultSlice(&cfg.Paths.FunctionQualityRoots, defaults.Paths.FunctionQualityRoots)
	applyDefaultSlice(&cfg.Paths.AllowedTestPrefixes, defaults.Paths.AllowedTestPrefixes)
	applyDefaultSlice(&cfg.Paths.LOCIgnorePrefixes, defaults.Paths.LOCIgnorePrefixes)
	applyDefaultSlice(&cfg.Paths.HardcodedRuntimeKnobScanRoots, defaults.Paths.HardcodedRuntimeKnobScanRoots)
	applyDefaultSlice(&cfg.Paths.HardcodedRuntimeKnobIgnorePath, defaults.Paths.HardcodedRuntimeKnobIgnorePath)
	if len(cfg.Paths.ContractTargets) == 0 {
		cfg.Paths.ContractTargets = defaults.Paths.ContractTargets
	}
}

// applyMiscDefaults fills in zero-valued miscellaneous config fields with their default values.
func applyMiscDefaults(cfg *PolicyConfig, defaults PolicyConfig) {
	applyDefaultSlice(&cfg.CLIFormatter.RequiredFiles, defaults.CLIFormatter.RequiredFiles)
	applyDefaultSlice(&cfg.HardcodedRuntimeKnob.Identifiers, defaults.HardcodedRuntimeKnob.Identifiers)
	applyDefaultSlice(&cfg.Architecture.Roots, defaults.Architecture.Roots)
	applyDefaultSlice(&cfg.Architecture.Concerns, defaults.Architecture.Concerns)
}

// applyFileSizeDefaults fills in zero-valued file size config fields with their default values.
func applyFileSizeDefaults(cfg *PolicyConfig, defaults PolicyConfig) {
	applyDefaultInt(&cfg.FileSize.WarnLOC, defaults.FileSize.WarnLOC)
	applyDefaultInt(&cfg.FileSize.MaxLOC, defaults.FileSize.MaxLOC)
	applyDefaultInt(&cfg.FileSize.WarnPenaltyPerCTXFunction, defaults.FileSize.WarnPenaltyPerCTXFunction)
	applyDefaultInt(&cfg.FileSize.MaxPenaltyPerCTXFunction, defaults.FileSize.MaxPenaltyPerCTXFunction)
	applyDefaultInt(&cfg.FileSize.MaxPenaltyCTXThreshold, defaults.FileSize.MaxPenaltyCTXThreshold)
	applyDefaultInt(&cfg.FileSize.MinWarnLOC, defaults.FileSize.MinWarnLOC)
	applyDefaultInt(&cfg.FileSize.MinMaxLOC, defaults.FileSize.MinMaxLOC)
	applyDefaultInt(&cfg.FileSize.MinWarnToMaxGap, defaults.FileSize.MinWarnToMaxGap)
}

// applyFunctionQualityDefaults fills in zero-valued function quality config fields with their default values.
func applyFunctionQualityDefaults(cfg *PolicyConfig, defaults PolicyConfig) {
	applyDefaultInt(&cfg.FunctionQuality.WarnLOC, defaults.FunctionQuality.WarnLOC)
	applyDefaultInt(&cfg.FunctionQuality.MaxLOC, defaults.FunctionQuality.MaxLOC)
	applyDefaultInt(&cfg.FunctionQuality.MildCTXMin, defaults.FunctionQuality.MildCTXMin)
	applyDefaultInt(&cfg.FunctionQuality.ElevatedCTXMin, defaults.FunctionQuality.ElevatedCTXMin)
	applyDefaultInt(&cfg.FunctionQuality.ImmediateRefactorCTXMin, defaults.FunctionQuality.ImmediateRefactorCTXMin)
	applyDefaultInt(&cfg.FunctionQuality.ErrorCTXMin, defaults.FunctionQuality.ErrorCTXMin)
	applyDefaultInt(&cfg.FunctionQuality.ErrorCTXAndLOCCTX, defaults.FunctionQuality.ErrorCTXAndLOCCTX)
	applyDefaultInt(&cfg.FunctionQuality.ErrorCTXAndLOCLOC, defaults.FunctionQuality.ErrorCTXAndLOCLOC)
	applyDefaultInt(&cfg.FunctionQuality.NilGuardRepeatWarnCount, defaults.FunctionQuality.NilGuardRepeatWarnCount)
}

// applyOutputDefaults fills in zero-valued output config fields with their default values.
func applyOutputDefaults(cfg *PolicyConfig, defaults PolicyConfig) {
	applyDefaultInt(&cfg.Output.MildCTXSummaryMinFunctions, defaults.Output.MildCTXSummaryMinFunctions)
	applyDefaultInt(&cfg.Output.MildCTXPerFileEscalationCount, defaults.Output.MildCTXPerFileEscalationCount)
	applyDefaultInt(&cfg.Output.MildCTXPerFileSummaryMinCount, defaults.Output.MildCTXPerFileSummaryMinCount)
}

// applySecretLoggingDefaults fills in zero-valued secret logging config fields with their default values.
func applySecretLoggingDefaults(cfg *PolicyConfig, defaults PolicyConfig) {
	applyDefaultSlice(&cfg.SecretLogging.Keywords, defaults.SecretLogging.Keywords)
	applyDefaultSlice(&cfg.SecretLogging.LoggerIdentifiers, defaults.SecretLogging.LoggerIdentifiers)
	applyDefaultSlice(&cfg.SecretLogging.IgnorePathPrefixes, defaults.SecretLogging.IgnorePathPrefixes)
	applyDefaultSlice(&cfg.SecretLogging.AllowedLiteralPatterns, defaults.SecretLogging.AllowedLiteralPatterns)
	applyDefaultSlice(&cfg.SecretLogging.Allowlist.LiteralPatterns, defaults.SecretLogging.Allowlist.LiteralPatterns)
	applyDefaultSlice(&cfg.SecretLogging.Allowlist.PatternIDs, defaults.SecretLogging.Allowlist.PatternIDs)
	if len(cfg.SecretLogging.Overrides) == 0 {
		cfg.SecretLogging.Overrides = defaults.SecretLogging.Overrides
	}
}

// ValidatePolicyConfig ensures all configuration values are within valid ranges.
func ValidatePolicyConfig(cfg *PolicyConfig) error {
	if err := validateQualityThresholds(cfg); err != nil {
		return err
	}
	if err := validatePathFields(cfg); err != nil {
		return err
	}
	return validateSecretLoggingPatterns(cfg)
}

// applyDefaultSlice copies default values into an empty slice field.
func applyDefaultSlice[T any](target *[]T, fallback []T) {
	if len(*target) == 0 {
		*target = fallback
	}
}

// applyDefaultInt copies a default value into an unset integer field.
func applyDefaultInt(target *int, fallback int) {
	if *target == 0 {
		*target = fallback
	}
}

// validateQualityThresholds validates cross-field quality threshold relationships.
func validateQualityThresholds(cfg *PolicyConfig) error {
	checks := []struct {
		valid   bool
		message string
	}{
		{valid: cfg.FunctionQuality.MildCTXMin > 0, message: "function_quality.mild_ctx_min must be > 0"},
		{valid: cfg.FunctionQuality.ElevatedCTXMin >= cfg.FunctionQuality.MildCTXMin, message: "function_quality.elevated_ctx_min must be >= mild_ctx_min"},
		{valid: cfg.FunctionQuality.ImmediateRefactorCTXMin >= cfg.FunctionQuality.ElevatedCTXMin, message: "function_quality.immediate_refactor_ctx_min must be >= elevated_ctx_min"},
		{valid: cfg.FunctionQuality.ErrorCTXMin >= cfg.FunctionQuality.ImmediateRefactorCTXMin, message: "function_quality.error_ctx_min must be >= immediate_refactor_ctx_min"},
		{valid: cfg.FileSize.MaxPenaltyCTXThreshold >= cfg.FunctionQuality.MildCTXMin, message: "file_size.max_penalty_ctx_threshold must be >= function_quality.mild_ctx_min"},
		{valid: cfg.Output.MildCTXPerFileSummaryMinCount <= cfg.Output.MildCTXPerFileEscalationCount, message: "output.mild_ctx_per_file_summary_min_count must be <= output.mild_ctx_per_file_escalation_count"},
	}
	for _, check := range checks {
		if !check.valid {
			return fmt.Errorf("%s", check.message)
		}
	}
	return nil
}

// validatePathFields ensures all path configuration fields contain valid unique values.
func validatePathFields(cfg *PolicyConfig) error {
	pathFields := map[string][]string{
		"paths.production_roots":                       cfg.Paths.ProductionRoots,
		"paths.secret_scan_roots":                      cfg.Paths.SecretScanRoots,
		"paths.test_scan_roots":                        cfg.Paths.TestScanRoots,
		"paths.file_loc_roots":                         cfg.Paths.FileLOCRoots,
		"paths.function_quality_roots":                 cfg.Paths.FunctionQualityRoots,
		"paths.allowed_test_prefixes":                  cfg.Paths.AllowedTestPrefixes,
		"paths.loc_ignore_prefixes":                    cfg.Paths.LOCIgnorePrefixes,
		"paths.hardcoded_runtime_knob_scan_roots":      cfg.Paths.HardcodedRuntimeKnobScanRoots,
		"paths.hardcoded_runtime_knob_ignore_prefixes": cfg.Paths.HardcodedRuntimeKnobIgnorePath,
	}
	for fieldName, values := range pathFields {
		if err := validateUniquePolicyPaths(fieldName, values); err != nil {
			return err
		}
	}

	architectureRoots := make([]string, 0, len(cfg.Architecture.Roots))
	for _, root := range cfg.Architecture.Roots {
		if strings.TrimSpace(root.Path) == "" {
			return fmt.Errorf("architecture.roots.path must not be empty")
		}
		architectureRoots = append(architectureRoots, root.Path)
	}
	if err := validateUniquePolicyPaths("architecture.roots.path", architectureRoots); err != nil {
		return err
	}
	for _, concern := range cfg.Architecture.Concerns {
		if strings.TrimSpace(concern.Name) == "" {
			return fmt.Errorf("architecture.concerns.name must not be empty")
		}
	}
	return nil
}

// validateSecretLoggingPatterns ensures all secret logging regex patterns are valid.
func validateSecretLoggingPatterns(cfg *PolicyConfig) error {
	for _, pattern := range cfg.SecretLogging.AllowedLiteralPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("secret_logging.allowed_literal_patterns contains invalid regexp %q: %w", pattern, err)
		}
	}
	for _, pattern := range cfg.SecretLogging.Allowlist.LiteralPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("secret_logging.allowlist.literal_patterns contains invalid regexp %q: %w", pattern, err)
		}
	}
	for patternID, severity := range cfg.SecretLogging.Overrides {
		normalizedSeverity := strings.ToUpper(strings.TrimSpace(severity))
		switch normalizedSeverity {
		case "OFF", "DISABLED", "LOW", "MEDIUM", "HIGH", "CRITICAL":
		default:
			return fmt.Errorf("secret_logging.overrides.%s has unsupported severity %q", patternID, severity)
		}
	}
	return nil
}

// validateUniquePolicyPaths checks that a list of paths contains no duplicates.
func validateUniquePolicyPaths(fieldName string, values []string) error {
	seen := make(map[string]string, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized := utils.NormalizePolicyPath(trimmed)
		if prior, ok := seen[normalized]; ok {
			return fmt.Errorf("%s contains duplicate path %q (also %q)", fieldName, trimmed, prior)
		}
		seen[normalized] = trimmed
	}
	return nil
}
