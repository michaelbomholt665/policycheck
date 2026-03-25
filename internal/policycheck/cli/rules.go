// internal/policycheck/cli/rules.go
package cli

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/router/capabilities"
	"policycheck/internal/router/ext"
)

// Run executes the policycheck CLI.
func Run(args []string) int {
	// 1. Parse flags
	fs := flag.NewFlagSet("policycheck", flag.ExitOnError)
	rootPtr := fs.String("root", ".", "Project root directory")
	configPtr := fs.String("config", "policy-gate.toml", "Path to policy-gate.toml")
	policyListPtr := fs.Bool("policy-list", false, "List all active policy rules")
	listRulesPtr := fs.Bool("list-rules", false, "List detailed descriptions of all enforced rules")

	if err := fs.Parse(args); err != nil {
		return HandleError(err)
	}

	// 2. Boot the router
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := ext.RouterBootExtensions(ctx); err != nil {
		return HandleError(fmt.Errorf("router boot: %w", err))
	}

	// 3. Resolve CLI styler
	styler, err := capabilities.ResolveCLIOutputStyler()
	if err != nil {
		return HandleError(fmt.Errorf("resolve styler: %w", err))
	}

	// 4. Handle --policy-list
	if *policyListPtr {
		printPolicyList(styler)
		return 0
	}

	// Handle --list-rules
	if *listRulesPtr {
		printRuleDescriptions(styler)
		return 0
	}

	// 5. Load config
	cfg, err := loadPolicyConfig(*configPtr)
	if err != nil {
		return HandleError(err)
	}

	// 6. Run checks
	absRoot, err := filepath.Abs(*rootPtr)
	if err != nil {
		return HandleError(err)
	}

	violations := core.RunPolicyChecks(ctx, absRoot, *cfg)

	// 7. Apply summary logic across all categories
	violations = SummarizeWarnings(*cfg, violations)

	// 8. Print results
	return processViolations(styler, violations)
}

func loadPolicyConfig(configPath string) (*config.PolicyConfig, error) {
	if err := host.SetInjectedPath(configPath); err != nil {
		return nil, err
	}

	configProvider, err := host.ResolveConfigProvider()
	if err != nil {
		return nil, err
	}

	rawConfig, err := configProvider.GetRawSource()
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	return config.Load(configPath, rawConfig)
}

func processViolations(styler capabilities.CLIOutputStyler, violations []types.Violation) int {
	if len(violations) == 0 {
		fmt.Println("policycheck: ok")
		return 0
	}

	// Sort violations by file and line for consistent output
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].File != violations[j].File {
			return violations[i].File < violations[j].File
		}
		return violations[i].Line < violations[j].Line
	})

	PrintViolations(styler, violations)

	// Check for errors (vs warnings) to determine exit code
	for _, v := range violations {
		if v.Severity == "error" {
			return 1
		}
	}

	return 0
}

func printPolicyList(styler capabilities.CLIOutputStyler) {
	header := "Active Policy Rules:"
	if styler != nil {
		if s, err := styler.StyleText(capabilities.TextKindHeader, header); err == nil {
			header = s
		}
	}
	fmt.Println(header)

	groups := []string{}
	for g := range core.PolicyRegistry {
		groups = append(groups, g)
	}
	sort.Strings(groups)

	for _, g := range groups {
		groupLabel := g
		if styler != nil {
			if s, err := styler.StyleText(capabilities.TextKindInfo, g); err == nil {
				groupLabel = s
			}
		} else {
			groupLabel = fmt.Sprintf("[%s]", g)
		}
		fmt.Printf("\n%s\n", groupLabel)
		// We cannot easily get function names from a function slice,
		// but we can list the group names at least.
		fmt.Printf("  (Contains %d checks)\n", len(core.PolicyRegistry[g]))
	}
}

func printRuleDescriptions(styler capabilities.CLIOutputStyler) {
	header := "Enforced Policy Rules:"
	if styler != nil {
		if s, err := styler.StyleText(capabilities.TextKindHeader, header); err == nil {
			header = s
		}
	}
	fmt.Println(header)
	fmt.Println("======================")

	rules := []struct {
		Name        string
		Description string
	}{
		{
			Name: "Scope Guard (scope-guard)",
			Description: `Security checks for lifecycle calls.
- Forbidden Calls: Prevents use of raw lifecycle calls like os.WriteFile or os.Rename outside of adapters.
- Exclusions: Automatically skips 'internal/router/' and all contents.`,
		},
		{
			Name: "Function Quality (function-quality)",
			Description: `Monitors function health across Go, Python, and TypeScript.
- Complexity (CTX): Measures cyclomatic complexity. Warn at 10-12, Error at 15+.
- Length (LOC): Warn at 80 (Go)/100 (Py/TS), Error at 120 (Go)/150 (Py)/160 (TS).
- Parameter Count: Warn at 5 parameters, Error at 7.`,
		},
		{
			Name: "Symbol Hygiene (hygiene.symbol_names)",
			Description: `Naming conventions for exported Go symbols.
- 2-Token Floor: Symbols used within a single backend must have 2+ tokens (e.g., 'RunQuery').
- 3-Token Rule: Cross-backend symbols require 3+ tokens for disambiguation (e.g., 'RunAgentQuery').
- Acronym Casing: Enforces standard Go casing (e.g., 'HTTPClient', not 'HttpClient').`,
		},
		{
			Name: "Documentation Style (hygiene.doc_style)",
			Description: `Google-style doc comments for Go.
- Prefix Rule: Comments must start with the symbol name.
- Substance: Minimum 5 words required to avoid tautological comments.
- Cleanup: Flags TODO/FIXME markers left in exported documentation.`,
		},
		{
			Name: "Secret Logging (secret-keyword)",
			Description: `Security scanner for sensitive data exposure.
- Keyword Detection: Flags 'password', 'token', 'api_key', etc. reaching logging sinks.
- Type-Aware: Suppresses false positives in struct tags, map keys, or config field names.
- Entropy Gating: Uses Shannon entropy for high-entropy strings (>32 chars).`,
		},
		{
			Name: "Architecture (architecture)",
			Description: `Enforces directory-level structure and dependency rules.
- Internal Allowlist: Only approved sub-packages allowed in 'internal/'.
- Directional Check: Packages in 'internal/' cannot import from 'cmd/'.`,
		},
		{
			Name: "Package Rules (structure.package_rules)",
			Description: `Structural health of Go packages.
- doc.go: Every package must have a valid doc.go file.
- File Limits: Hard error if a package exceeds 10 production files.`,
		},
		{
			Name: "Go Version (contracts.go_version)",
			Description: `Ensures project uses approved Go version (1.24/1.25).
- Pins: Verifies both 'go' and 'toolchain' directives in go.mod.`,
		},
	}

	for _, r := range rules {
		ruleHeader := r.Name
		if styler != nil {
			if s, err := styler.StyleText(capabilities.TextKindInfo, r.Name); err == nil {
				ruleHeader = s
			}
		} else {
			ruleHeader = fmt.Sprintf("\n--- %s ---", r.Name)
		}
		fmt.Printf("\n%s\n%s\n", ruleHeader, r.Description)
	}
}
