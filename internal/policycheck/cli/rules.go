// internal/policycheck/cli/rules.go
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
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
	fs := flag.NewFlagSet("policycheck", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	rootPtr := fs.String("root", ".", "Project root directory")
	configPtr := fs.String("config", "policy-gate.toml", "Path to policy-gate.toml")
	policyListPtr := fs.Bool("policy-list", false, "List all active policy groups")
	listRulesPtr := fs.Bool("list-rules", false, "List detailed descriptions of all enforced rules")
	interactivePtr := fs.Bool("interactive", false, "Browse the policy catalog interactively when the CLI interaction capability is available")

	if err := fs.Parse(args); err != nil {
		return HandleError(fmt.Errorf("parse policycheck flags: %w", err))
	}

	// 2. Boot the router
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	warnings, err := ext.RouterBootExtensions(ctx)
	if err != nil {
		return HandleError(fmt.Errorf("router boot: %w", err))
	}

	renderers, err := ResolveRenderers()
	if err != nil {
		return HandleError(err)
	}
	printRouterWarnings(renderers.Chrome, warnings)

	if *interactivePtr {
		interactiveErr := RunInteractivePolicyCatalog(renderers)
		if interactiveErr != nil {
			return HandleError(fmt.Errorf("run interactive policy catalog: %w", interactiveErr))
		}
		return 0
	}

	if *policyListPtr {
		policyListErr := PrintPolicyList(renderers)
		if policyListErr != nil {
			return HandleError(fmt.Errorf("print policy list: %w", policyListErr))
		}
		return 0
	}

	if *listRulesPtr {
		ruleDescriptionErr := PrintRuleDescriptions(renderers)
		if ruleDescriptionErr != nil {
			return HandleError(fmt.Errorf("print rule descriptions: %w", ruleDescriptionErr))
		}
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
	exitCode, processErr := processViolations(renderers, violations)
	if processErr != nil {
		return HandleError(processErr)
	}

	return exitCode
}

func loadPolicyConfig(configPath string) (*config.PolicyConfig, error) {
	if err := host.SetInjectedPath(configPath); err != nil {
		return nil, fmt.Errorf("set injected config path: %w", err)
	}

	configProvider, err := host.ResolveConfigProvider()
	if err != nil {
		return nil, fmt.Errorf("resolve config provider: %w", err)
	}

	rawConfig, err := configProvider.GetRawSource()
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	loadedConfig, err := config.Load(configPath, rawConfig)
	if err != nil {
		return nil, fmt.Errorf("load config %s: %w", configPath, err)
	}

	return loadedConfig, nil
}

func processViolations(renderers Renderers, violations []types.Violation) (int, error) {
	if len(violations) == 0 {
		success, err := styleChromeText(renderers.Chrome, capabilities.TextKindSuccess, "policycheck: ok")
		if err != nil {
			success = "policycheck: ok"
		}

		if _, err := fmt.Fprintln(os.Stdout, success); err != nil {
			return 1, fmt.Errorf("write success output: %w", err)
		}

		return 0, nil
	}

	violations = ArrangeViolationsForCLI(violations)

	if err := PrintViolations(renderers.Chrome, violations); err != nil {
		return 1, fmt.Errorf("print violations: %w", err)
	}

	// Check for errors (vs warnings) to determine exit code
	for _, v := range violations {
		if v.Severity == "error" {
			return 1, nil
		}
	}

	return 0, nil
}

// ArrangeViolationsForCLI returns a sorted copy of violations for stable CLI grouping.
func ArrangeViolationsForCLI(violations []types.Violation) []types.Violation {
	arranged := append([]types.Violation(nil), violations...)
	sort.Slice(arranged, func(i, j int) bool {
		leftSeverity := violationSeverityRank(arranged[i].Severity)
		rightSeverity := violationSeverityRank(arranged[j].Severity)
		if leftSeverity != rightSeverity {
			return leftSeverity < rightSeverity
		}

		if arranged[i].RuleID != arranged[j].RuleID {
			return arranged[i].RuleID < arranged[j].RuleID
		}

		if arranged[i].File != arranged[j].File {
			return arranged[i].File < arranged[j].File
		}

		if arranged[i].Function != arranged[j].Function {
			return arranged[i].Function < arranged[j].Function
		}

		if arranged[i].Line != arranged[j].Line {
			return arranged[i].Line < arranged[j].Line
		}

		return arranged[i].Message < arranged[j].Message
	})

	return arranged
}

func violationSeverityRank(severity string) int {
	switch severity {
	case "error":
		return 0
	case "warn", "warning":
		return 1
	default:
		return 2
	}
}
