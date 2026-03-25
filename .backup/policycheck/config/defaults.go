// internal/policycheck/config/defaults.go
// Provides the default policy configuration values used when no config file exists.

package config

const ScopeProjectRepo = true

// DefaultPolicyConfig returns the hardcoded default policy configuration.
func DefaultPolicyConfig() PolicyConfig {
	return PolicyConfig{
		Paths: PolicyPathsConfig{
			ProductionRoots:                []string{"internal", "cmd", "src", "pkg", "app", "lib"},
			SecretScanRoots:                []string{"internal", "cmd", "src", "pkg", "app", "lib"},
			TestScanRoots:                  []string{"cmd", "internal", "src", "pkg", "app", "lib", "tests", "test", "."},
			FileLOCRoots:                   []string{"internal", "cmd", "src", "pkg", "app", "lib", "test"},
			FunctionQualityRoots:           []string{"internal", "cmd", "src", "pkg", "app", "lib"},
			AllowedTestPrefixes:            []string{"internal/tests/", "tests/", "test/"},
			LOCIgnorePrefixes:              []string{},
			HardcodedRuntimeKnobScanRoots:  []string{"internal", "src", "pkg", "app", "lib"},
			HardcodedRuntimeKnobIgnorePath: []string{},
			ContractTargets:                map[string]string{},
		},
		FileSize: PolicyFileSizeConfig{
			WarnLOC:                   550,
			MaxLOC:                    900,
			WarnPenaltyPerCTXFunction: 10,
			MaxPenaltyPerCTXFunction:  15,
			MaxPenaltyCTXThreshold:    12,
			MinWarnLOC:                400,
			MinMaxLOC:                 700,
			MinWarnToMaxGap:           150,
		},
		FunctionQuality: PolicyFunctionQualityConfig{
			WarnLOC:                 80,
			MaxLOC:                  120,
			MildCTXMin:              10,
			ElevatedCTXMin:          12,
			ImmediateRefactorCTXMin: 14,
			ErrorCTXMin:             15,
			ErrorCTXAndLOCCTX:       8,
			ErrorCTXAndLOCLOC:       80,
			NilGuardRepeatWarnCount: 8,
		},
		Output: PolicyOutputConfig{
			MildCTXCompressSummary:        true,
			MildCTXSummaryMinFunctions:    2,
			MildCTXPerFileEscalationCount: 3,
			MildCTXPerFileSummaryMinCount: 2,
		},
		SecretLogging: PolicySecretLoggingConfig{
			Keywords: []string{"password", "passwd", "token", "api_key", "apikey", "dsn", "connection string", "connection_string"},
			LoggerIdentifiers: []string{
				"log", "logger", "zap", "zerolog", "fmt", "os", "ctxlog", "l",
				"logging", "logger", "self.logger", "cls.logger",
				"console", "logger", "winston", "pino", "log",
			},
			Allowlist: PolicySecretLoggingAllowlist{
				LiteralPatterns: []string{},
				PatternIDs:      []string{},
			},
			Overrides: map[string]string{},
		},
		CLIFormatter: PolicyCLIFormatterConfig{
			RequiredFiles: []string{},
		},
		HardcodedRuntimeKnob: PolicyHardcodedRuntimeKnobConfig{
			Identifiers: []string{"Timeout", "TimeoutSeconds", "MaxFile", "MaxSize", "Port", "Host", "AllowRemote", "Concurrent", "CacheTTL", "LogLevel"},
		},
		Architecture: PolicyArchitectureConfig{
			Enforce: false,
			Roots:   []PolicyArchitectureRoot{},
			Concerns: []PolicyArchitectureTopic{
				{
					Name:        "database",
					Tags:        []string{"database", "schema", "queries", "config"},
					Roots:       []string{"internal/db", "docs/database"},
					ConfigPaths: []string{"internal/config"},
				},
			},
		},
		GoVersion: PolicyVersionConfig{
			AllowedPrefixes: []string{"1.24", "1.25"},
		},
		PythonVersion: PolicyVersionConfig{
			AllowedPrefixes: []string{""},
		},
		TypescriptVersion: PolicyVersionConfig{
			AllowedPrefixes: []string{""},
		},
	}
}
