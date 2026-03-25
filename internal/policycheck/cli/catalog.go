package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"policycheck/internal/policycheck/core"
	"policycheck/internal/router/capabilities"
)

type policyGroupSummary struct {
	Name       string
	CheckCount int
}

type policyRule struct {
	ID          string
	Name        string
	Category    string
	Summary     string
	Description string
}

var policyRulesCatalog = []policyRule{
	{
		ID:       "scope-guard",
		Name:     "Scope Guard",
		Category: "contracts",
		Summary:  "Security checks for lifecycle calls.",
		Description: `Forbidden Calls: Prevents use of raw file-write or rename lifecycle calls outside of adapters.
Mode and Enablement: Uses scope_guard.enabled and scope_guard.mode to disable, restrict, or ban matches.
Exclusions: Automatically skips internal/router/ and all contents.`,
	},
	{
		ID:       "hardcoded-runtime-knob",
		Name:     "Hardcoded Runtime Knob",
		Category: "contracts",
		Summary:  "Flags config-like runtime values that are hardcoded in Go code.",
		Description: `Identifier Match: Uses hardcoded_runtime_knob.identifiers to find config-like names such as Timeout or Port.
Scope Control: Uses paths.hardcoded_runtime_knob_scan_roots and paths.hardcoded_runtime_knob_ignore_prefixes.
Literal Detection: Flags literal-backed assignments and struct fields that should be config-driven.`,
	},
	{
		ID:       "function-quality",
		Name:     "Function Quality",
		Category: "quality",
		Summary:  "Monitors function complexity, length, and parameter counts.",
		Description: `Complexity (CTX): Measures cyclomatic complexity. Warn at 10-12, Error at 15+.
Length (LOC): Warn at 80 (Go)/100 (Py/TS), Error at 120 (Go)/150 (Py)/160 (TS).
Parameter Count: Warn at 5 parameters, Error at 7.
Scope and Output: Uses paths.function_quality_roots plus output mild-CTX summary settings.`,
	},
	{
		ID:       "hygiene.symbol_names",
		Name:     "Symbol Hygiene",
		Category: "hygiene",
		Summary:  "Naming conventions for exported Go symbols.",
		Description: `2-Token Floor: Symbols used within a single backend must have 2+ tokens (for example, RunQuery).
3-Token Rule: Cross-backend symbols require 3+ tokens for disambiguation (for example, RunAgentQuery).
Acronym Casing: Enforces standard Go casing (for example, HTTPClient, not HttpClient).`,
	},
	{
		ID:       "hygiene.doc_style",
		Name:     "Documentation Style",
		Category: "hygiene",
		Summary:  "Google-style doc comments for exported Go symbols.",
		Description: `Prefix Rule: Comments must start with the symbol name.
Substance: Minimum 5 words required to avoid tautological comments.
Cleanup: Flags TODO/FIXME markers left in exported documentation.`,
	},
	{
		ID:       "secret-keyword",
		Name:     "Secret Logging",
		Category: "security",
		Summary:  "Security scanner for sensitive data exposure.",
		Description: `Keyword Detection: Flags password, token, api_key, and similar values reaching logging sinks.
Type-Aware: Suppresses false positives in struct tags, map keys, or config field names.
Entropy Gating: Uses Shannon entropy for high-entropy strings longer than 32 characters.
Allowlists: Honors secret_logging.allowed_literal_patterns and secret_logging.allowlist literal/pattern-ID entries.`,
	},
	{
		ID:       "structure.architecture",
		Name:     "Architecture",
		Category: "structure",
		Summary:  "Enforces directory-level structure and dependency rules.",
		Description: `Internal Allowlist: Only approved sub-packages allowed in internal/.
Directional Check: Packages in internal/ cannot import from cmd/.
Concern Coverage: Every architecture.concerns path entry must resolve to a real repo path.`,
	},
	{
		ID:       "structure.test_location",
		Name:     "Test Location",
		Category: "structure",
		Summary:  "Ensures tests live under approved test roots.",
		Description: `Scan Roots: Uses paths.test_scan_roots to find Go test files.
Allowed Prefixes: Uses paths.allowed_test_prefixes to enforce where tests may live.`,
	},
	{
		ID:       "file-size",
		Name:     "File Size",
		Category: "quality",
		Summary:  "Monitors file size with complexity-adjusted thresholds.",
		Description: `Scan Roots: Uses paths.file_loc_roots and paths.loc_ignore_prefixes.
Thresholds: Uses file_size warn/error floors plus CTX penalty settings.
Penalty Inputs: Reuses function-quality facts to tighten file-size limits for complex files.`,
	},
	{
		ID:       "ai-compatibility",
		Name:     "AI Compatibility",
		Category: "contracts",
		Summary:  "Checks required AI-related CLI flags in command surfaces.",
		Description: `Required Flags: Uses ai_compatibility.required_flags.
Coverage: Scans repository Go files for command implementations missing those flags.`,
	},
	{
		ID:       "cli-formatter",
		Name:     "CLI Formatter",
		Category: "contracts",
		Summary:  "Requires formatter-aware CLI output in configured files.",
		Description: `Required Files: Uses cli_formatter.required_files.
Output Guard: Flags direct fmt.Print* output in those command files.`,
	},
	{
		ID:       "go-version",
		Name:     "Go Version",
		Category: "contracts",
		Summary:  "Ensures the project uses an approved Go version.",
		Description: `Pins: Verifies both go and toolchain directives in go.mod.
Allowed Versions: Enforces go_version.allowed_prefixes from policy-gate.toml.`,
	},
	{
		ID:       "custom-rule",
		Name:     "Custom Rule",
		Category: "custom",
		Summary:  "Runs repo-configured regex rules from policy-gate.toml.",
		Description: `Rule Source: Uses [[custom_rules]] entries in policy-gate.toml.
Matching: Applies each enabled regex to matching files and emits custom.<id> violations.`,
	},
	{
		ID:       "structure.package_rules",
		Name:     "Package Rules",
		Category: "structure",
		Summary:  "Structural health checks for Go packages.",
		Description: `doc.go: Every package must have a valid doc.go file.
File Limits: Hard error if a package exceeds 10 production files.`,
	},
}

// PrintPolicyList renders the grouped policy list using chrome for headings and output styling for tables.
func PrintPolicyList(renderers Renderers) error {
	header, err := styleChromeText(renderers.Chrome, capabilities.TextKindHeader, "Active Policy Groups")
	if err != nil {
		return fmt.Errorf("render policy list header: %w", err)
	}

	if _, err := fmt.Fprintln(os.Stdout, header); err != nil {
		return fmt.Errorf("write policy list header: %w", err)
	}

	groups := collectPolicyGroups()
	if len(groups) == 0 {
		return nil
	}

	if renderers.Output != nil {
		headers := []string{"Category", "Checks"}
		rows := make([][]string, 0, len(groups))
		for _, group := range groups {
			rows = append(rows, []string{group.Name, fmt.Sprintf("%d", group.CheckCount)})
		}

		renderedTable, err := renderers.Output.StyleTable(capabilities.TableKindCompact, headers, rows)
		if err != nil {
			return fmt.Errorf("render policy list table: %w", err)
		}

		if _, err := fmt.Fprintln(os.Stdout, renderedTable); err != nil {
			return fmt.Errorf("write policy list table: %w", err)
		}

		return nil
	}

	for _, group := range groups {
		if _, err := fmt.Fprintf(os.Stdout, "%s (%d checks)\n", group.Name, group.CheckCount); err != nil {
			return fmt.Errorf("write policy list row: %w", err)
		}
	}

	return nil
}

// PrintRuleDescriptions renders the policy rule catalog using the resolved capabilities.
func PrintRuleDescriptions(renderers Renderers) error {
	header, err := styleChromeText(renderers.Chrome, capabilities.TextKindHeader, "Enforced Policy Rules")
	if err != nil {
		return fmt.Errorf("render rule description header: %w", err)
	}

	if _, err := fmt.Fprintln(os.Stdout, header); err != nil {
		return fmt.Errorf("write rule description header: %w", err)
	}

	if renderers.Output != nil {
		headers := []string{"Rule", "Category", "Summary"}
		rows := make([][]string, 0, len(policyRulesCatalog))
		for _, rule := range policyRulesCatalog {
			rows = append(rows, []string{rule.ID, rule.Category, rule.Summary})
		}

		renderedTable, err := renderers.Output.StyleTable(capabilities.TableKindNormal, headers, rows)
		if err != nil {
			return fmt.Errorf("render rule summary table: %w", err)
		}

		if _, err := fmt.Fprintln(os.Stdout, renderedTable); err != nil {
			return fmt.Errorf("write rule summary table: %w", err)
		}
	}

	for _, rule := range policyRulesCatalog {
		if err := printRuleDetail(renderers.Chrome, rule); err != nil {
			return err
		}
	}

	return nil
}

// RunInteractivePolicyCatalog browses policy groups and rules through the router-native interaction capability.
func RunInteractivePolicyCatalog(renderers Renderers) error {
	if renderers.Interactor == nil {
		printInteractiveFallbackNotice(renderers.Chrome)
		return PrintRuleDescriptions(renderers)
	}

	viewChoice, err := renderers.Interactor.StylePrompt(
		capabilities.PromptKindSelect,
		"Policy Explorer",
		"Choose what to inspect.",
		[]capabilities.Choice{
			{Key: "groups", Label: "Policy groups"},
			{Key: "rules", Label: "Rule details"},
		},
	)
	if err != nil {
		return fmt.Errorf("select policy explorer view: %w", err)
	}

	selectedView, ok := viewChoice.(string)
	if !ok {
		return fmt.Errorf("select policy explorer view: unexpected result %T", viewChoice)
	}

	switch selectedView {
	case "groups":
		return runInteractiveGroupView(renderers)
	case "rules":
		return runInteractiveRuleView(renderers)
	default:
		return fmt.Errorf("select policy explorer view: unsupported selection %q", selectedView)
	}
}

func runInteractiveGroupView(renderers Renderers) error {
	groups := collectPolicyGroups()
	options := make([]capabilities.Choice, 0, len(groups))
	for _, group := range groups {
		options = append(options, capabilities.Choice{
			Key:   group.Name,
			Label: fmt.Sprintf("%s (%d checks)", group.Name, group.CheckCount),
		})
	}

	choice, err := renderers.Interactor.StylePrompt(
		capabilities.PromptKindSelect,
		"Policy Groups",
		"Select a policy group to inspect.",
		options,
	)
	if err != nil {
		return fmt.Errorf("select policy group: %w", err)
	}

	selectedGroup, ok := choice.(string)
	if !ok {
		return fmt.Errorf("select policy group: unexpected result %T", choice)
	}

	for _, group := range groups {
		if group.Name != selectedGroup {
			continue
		}

		content := fmt.Sprintf("Checks: %d\nUse --list-rules for the full static catalog.", group.CheckCount)
		rendered, err := styleChromeLayout(renderers.Chrome, capabilities.LayoutKindPanel, group.Name, content)
		if err != nil {
			return fmt.Errorf("render policy group detail: %w", err)
		}

		if _, err := fmt.Fprintln(os.Stdout, rendered); err != nil {
			return fmt.Errorf("write policy group detail: %w", err)
		}

		return nil
	}

	return fmt.Errorf("select policy group: unknown group %q", selectedGroup)
}

func runInteractiveRuleView(renderers Renderers) error {
	options := make([]capabilities.Choice, 0, len(policyRulesCatalog))
	for _, rule := range policyRulesCatalog {
		options = append(options, capabilities.Choice{
			Key:   rule.ID,
			Label: fmt.Sprintf("%s (%s)", rule.Name, rule.ID),
		})
	}

	choice, err := renderers.Interactor.StylePrompt(
		capabilities.PromptKindSelect,
		"Policy Rules",
		"Select a rule to inspect.",
		options,
	)
	if err != nil {
		return fmt.Errorf("select policy rule: %w", err)
	}

	selectedRuleID, ok := choice.(string)
	if !ok {
		return fmt.Errorf("select policy rule: unexpected result %T", choice)
	}

	for _, rule := range policyRulesCatalog {
		if rule.ID != selectedRuleID {
			continue
		}

		return printRuleDetail(renderers.Chrome, rule)
	}

	return fmt.Errorf("select policy rule: unknown rule %q", selectedRuleID)
}

func printRuleDetail(chrome capabilities.CLIChromeStyler, rule policyRule) error {
	body := strings.Join([]string{
		fmt.Sprintf("Rule ID: %s", rule.ID),
		fmt.Sprintf("Category: %s", rule.Category),
		"",
		rule.Summary,
		"",
		rule.Description,
	}, "\n")

	rendered, err := styleChromeLayout(chrome, capabilities.LayoutKindPanel, rule.Name, body)
	if err != nil {
		return fmt.Errorf("render rule detail for %s: %w", rule.ID, err)
	}

	if _, err := fmt.Fprintln(os.Stdout, rendered); err != nil {
		return fmt.Errorf("write rule detail for %s: %w", rule.ID, err)
	}

	return nil
}

func collectPolicyGroups() []policyGroupSummary {
	groups := make([]policyGroupSummary, 0, len(core.PolicyRegistry))
	for name, checks := range core.PolicyRegistry {
		groups = append(groups, policyGroupSummary{
			Name:       name,
			CheckCount: len(checks),
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups
}

func renderPlainLayout(title string, content ...string) string {
	body := strings.Join(content, "\n")
	if title == "" {
		return body
	}

	if body == "" {
		return title
	}

	return fmt.Sprintf("%s\n%s", title, body)
}
