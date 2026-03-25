// internal/policycheck/cli/cli.go
// Implements the command-line interface for policycheck: flag parsing, config loading, result output.

package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core"
	"policycheck/internal/policycheck/types"
)

type mainOptions struct {
	root        string
	configPath  string
	concernName string
	format      string
	policyList  bool
	noCreate    bool
	dryRun      bool
	lenient     bool
}

// RunCLI parses args, loads config, runs all policy checks, and returns the process exit code.
func RunCLI(args []string, scannerBytes types.ScannerBytes) int {
	options, err := parseMainOptions(args)
	if err != nil {
		fmt.Printf("[ERROR] %s\n", err)
		return 1
	}

	cfg, effectiveRoot, exitCode, handled := loadMainConfig(options)
	if handled {
		return exitCode
	}
	if handleMainMode(options, cfg) {
		return 0
	}

	loadEnv(effectiveRoot)
	if os.Getenv("POLICY_CHECK_CONFIG") == "true" {
		fmt.Println("[INFO] POLICY_CHECK_CONFIG is true; getting policy checks from script-manager.toml (not yet implemented)")
		fmt.Println("policycheck: ok")
		return 0
	}

	results := core.RunPolicyChecks(effectiveRoot, cfg, scannerBytes)
	if err := printResults(options.format, results); err != nil {
		fmt.Printf("[ERROR] print results: %s\n", err)
		return 1
	}
	if len(results.Violations) > 0 || len(results.ScannerErrors) > 0 {
		return 1
	}
	return 0
}

// parseMainOptions parses command-line arguments into mainOptions and returns any error.
func parseMainOptions(args []string) (mainOptions, error) {
	fs := flag.NewFlagSet("policycheck", flag.ContinueOnError)
	root := fs.String("root", ".", "repository root")
	configPath := fs.String("config", config.DefaultPolicyConfigPath, "path to policy-gate.toml")
	policyList := fs.Bool("policy-list", false, "list all enforced policies")
	concernName := fs.String("concern", "", "print configured architecture concern locations")
	noCreate := fs.Bool("no-create", false, "do not create policy-gate.toml when missing")
	dryRun := fs.Bool("dry-run", false, "perform a scan without generating a default config file if missing")
	lenient := fs.Bool("lenient-config", false, "allow unknown fields in policy-gate.toml")
	format := fs.String("format", "text", "output format (text, json, ndjson)")

	if err := fs.Parse(args); err != nil {
		return mainOptions{}, fmt.Errorf("parse flags: %w", err)
	}
	return mainOptions{
		root:        *root,
		configPath:  *configPath,
		concernName: *concernName,
		format:      *format,
		policyList:  *policyList,
		noCreate:    *noCreate,
		dryRun:      *dryRun,
		lenient:     *lenient,
	}, nil
}

// loadMainConfig loads the policy configuration and determines the effective repository root.
func loadMainConfig(options mainOptions) (config.PolicyConfig, string, int, bool) {
	cfg, err := config.LoadPolicyConfig(options.root, options.configPath, !options.noCreate && !options.dryRun, options.lenient)
	if err != nil {
		fmt.Printf("[ERROR] %s\n", err)
		return config.PolicyConfig{}, "", 1, true
	}

	effectiveRoot := cfg.Runtime.RepoRoot
	if effectiveRoot == "" {
		effectiveRoot = options.root
	}
	if cfg.Runtime.WasCreated {
		printCreatedPolicyConfigMessage(cfg)
		return config.PolicyConfig{}, "", 1, true
	}
	return cfg, effectiveRoot, 0, false
}

// handleMainMode handles special modes like policy list or concern printing and returns true if handled.
func handleMainMode(options mainOptions, cfg config.PolicyConfig) bool {
	if options.policyList {
		printPolicyList(cfg)
		return true
	}
	if strings.TrimSpace(options.concernName) != "" {
		if err := core.PrintConcern(cfg, options.concernName); err != nil {
			fmt.Printf("[ERROR] %s\n", err)
			os.Exit(1)
		}
		return true
	}
	return false
}

// loadEnv reads and sets environment variables from a .env file in the repository root.
func loadEnv(root string) {
	path := filepath.Join(root, ".env")
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for i, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
			_ = os.Setenv(key, val)
		} else {
			fmt.Printf("[TRACE] %s:%d: invalid .env line (missing '=')\n", path, i+1)
		}
	}
}

// printResults outputs policy check results in the specified format (text, JSON, or NDJSON).
func printResults(format string, results types.PolicyCheckResults) error {
	type finding struct {
		Kind     string `json:"kind"`
		Severity string `json:"severity,omitempty"`
		Path     string `json:"path"`
		Message  string `json:"message"`
	}
	buildFindings := func() []finding {
		items := make([]finding, 0, len(results.ScannerErrors)+len(results.Warnings)+len(results.Violations))
		appendGroup := func(kind string, violations []types.Violation) {
			for _, v := range violations {
				items = append(items, finding{Kind: kind, Path: v.Path, Message: v.Message, Severity: v.Severity})
			}
		}
		appendGroup("scanner_error", results.ScannerErrors)
		appendGroup("warn", results.Warnings)
		appendGroup("error", results.Violations)
		return items
	}

	switch strings.ToLower(format) {
	case "json":
		findings := buildFindings()
		data, err := json.MarshalIndent(findings, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json findings: %w", err)
		}
		fmt.Println(string(data))
	case "ndjson":
		for _, entry := range buildFindings() {
			data, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("marshal ndjson finding: %w", err)
			}
			fmt.Println(string(data))
		}
	default:
		printTextResults(results)
	}
	return nil
}

// printTextResults outputs policy check results in human-readable text format.
func printTextResults(results types.PolicyCheckResults) {
	printViolationList("ERROR", results.ScannerErrors)
	printViolationList("WARN", results.Warnings)
	if len(results.Violations) == 0 {
		fmt.Println("policycheck: ok")
		return
	}
	printViolationList("ERROR", results.Violations)
}

// printViolationList prints all violations of a given severity level.
func printViolationList(level string, items []types.Violation) {
	for _, item := range items {
		printViolation(level, item)
	}
}

// printViolation prints a single policy violation with its level, path, and message.
func printViolation(level string, item types.Violation) {
	if item.Path == "" {
		if item.Severity != "" {
			fmt.Printf("[%s][%s] %s\n", level, item.Severity, item.Message)
			return
		}
		fmt.Printf("[%s] %s\n", level, item.Message)
		return
	}
	if item.Severity != "" {
		fmt.Printf("[%s][%s] %s: %s\n", level, item.Severity, item.Path, item.Message)
		return
	}
	fmt.Printf("[%s] %s: %s\n", level, item.Path, item.Message)
}

// printPolicyList displays a list of all enforced codebase policies.
func printPolicyList(cfg config.PolicyConfig) {
	policies := []string{
		"1.  Go Version: version must be 1.24.* or 1.25.* in go.mod.",
		"2.  Secret Logging: No secret leakage (password, API key, etc.) in log literals in internal/ or cmd/.",
		"3.  Test Location: All *_test.go files must be in internal/tests/.",
		"4.  CLI Formatter: Commands in cmd/ must use the audience-aware formatter (no raw fmt.Print).",
		fmt.Sprintf("5.  File Size: Base max %d lines per .go file (warn at %d) in configured production roots, with stricter thresholds as files accumulate CTX-heavy functions.", cfg.FileSize.MaxLOC, cfg.FileSize.WarnLOC),
		fmt.Sprintf("6.  Function Quality: Go, Python, and TypeScript functions ERROR if ctx >= %d OR loc >= %d OR (ctx >= %d AND loc >= %d).", cfg.FunctionQuality.ErrorCTXMin, cfg.FunctionQuality.MaxLOC, cfg.FunctionQuality.ErrorCTXAndLOCCTX, cfg.FunctionQuality.ErrorCTXAndLOCLOC),
		fmt.Sprintf("    WARN bands: mild ctx %d-%d may be compressed, elevated ctx %d-%d are listed, ctx >= %d gets an immediate-refactor warning.", cfg.FunctionQuality.MildCTXMin, cfg.FunctionQuality.ElevatedCTXMin-1, cfg.FunctionQuality.ElevatedCTXMin, cfg.FunctionQuality.ImmediateRefactorCTXMin-1, cfg.FunctionQuality.ImmediateRefactorCTXMin),
		"7.  Symbol Names: Function names must have at least 2 tokens (e.g., ValidateSchema).",
		"8.  Doc Style: Functions and exported types must have Google-style comments starting with the symbol name.",
		"9.  AI Compatibility: Root command must support --ai flag.",
		"10. Scope Guard: Commands must default to ScopeProjectRepo for the --scope flag.",
		"11. Package Rules: Max 10 production files per package. Each package MUST have a doc.go with a Package Concerns: section.",
		formatArchitecturePolicyListItem(cfg),
	}
	fmt.Println("Enforced Policies:")
	for _, p := range policies {
		fmt.Printf("  - %s\n", p)
	}
}

// formatArchitecturePolicyListItem returns a formatted description of the architecture policy setting.
func formatArchitecturePolicyListItem(cfg config.PolicyConfig) string {
	if !cfg.Architecture.Enforce {
		return "12. Architecture Roots: disabled."
	}
	roots := make([]string, 0, len(cfg.Architecture.Roots))
	for _, root := range cfg.Architecture.Roots {
		if strings.TrimSpace(root.Path) != "" {
			roots = append(roots, filepath.ToSlash(root.Path))
		}
	}
	sort.Strings(roots)
	if len(roots) == 0 {
		return "12. Architecture Roots: enabled with no configured roots."
	}
	return fmt.Sprintf("12. Architecture Roots: enforced for %s.", strings.Join(roots, ", "))
}

// printCreatedPolicyConfigMessage informs the user about a newly created policy configuration file.
func printCreatedPolicyConfigMessage(cfg config.PolicyConfig) {
	if cfg.Runtime.CreatedConfigPath == "" {
		return
	}
	fmt.Printf("[INFO] policy-gate.toml created at %s\n", cfg.Runtime.CreatedConfigPath)
	fmt.Println("[INFO] Review required values before rerunning policycheck.")
	fmt.Println("[INFO] Suggested edits:")
	for _, line := range config.FormatPolicyReviewTargets(cfg.Runtime.CreatedConfigPath) {
		fmt.Println(line)
	}
}
