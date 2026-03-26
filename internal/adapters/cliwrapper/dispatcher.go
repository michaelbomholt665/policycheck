// internal/adapters/cliwrapper/dispatcher.go
//
// Package cliwrapper provides the implementation of the CLI-wrapper subsystem.
// It detects and dispatches wrapper-specific commands such as package-gate
// scanning, macro execution, and header formatting.
package cliwrapper

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"policycheck/internal/ports"
	"policycheck/internal/router"
)

// WrapperDispatcher is the real implementation of ports.CLIWrapperDispatcher.
//
// WrapperDispatcher resolves its security gate dependency through the router at
// dispatch time — it never holds a direct reference to another adapter. The
// injected ExecFunc handles all subprocess execution so tests can verify
// orchestration without starting real processes.
type WrapperDispatcher struct {
	detector             WrapperDetector
	cfg                  WrapperConfig
	exec                 ExecFunc
	loadConfig           func() (WrapperConfig, error)
	securityGateResolver func() (ports.CLIWrapperSecurityGate, error)
	macroRunnerResolver  func() (ports.CLIWrapperMacroRunner, error)
	formatterResolver    func() (ports.CLIWrapperFormatter, error)
}

// WrapperResolvers groups injected router-provider resolvers for tests and
// alternate host seams.
type WrapperResolvers struct {
	SecurityGate func() (ports.CLIWrapperSecurityGate, error)
	MacroRunner  func() (ports.CLIWrapperMacroRunner, error)
	Formatter    func() (ports.CLIWrapperFormatter, error)
}

// NewWrapperDispatcher returns a WrapperDispatcher with the given config and exec function.
//
// The security gate is resolved from the router at each Dispatch call; callers
// must not pass it here. Tests pass an ExecFunc recorder; production callers
// pass OsExec.
func NewWrapperDispatcher(cfg WrapperConfig, exec ExecFunc) *WrapperDispatcher {
	return &WrapperDispatcher{
		detector:             WrapperDetector{},
		cfg:                  cfg,
		exec:                 exec,
		loadConfig:           staticWrapperConfigLoader(cfg),
		securityGateResolver: resolveSecurityGate,
		macroRunnerResolver:  resolveMacroRunner,
		formatterResolver:    resolveFormatter,
	}
}

// NewWrapperDispatcherWithResolver returns a WrapperDispatcher with an injected
// security-gate resolver for tests or alternate host seams.
func NewWrapperDispatcherWithResolver(
	cfg WrapperConfig,
	exec ExecFunc,
	resolver func() (ports.CLIWrapperSecurityGate, error),
) *WrapperDispatcher {
	dispatcher := NewWrapperDispatcher(cfg, exec)
	dispatcher.securityGateResolver = resolver

	return dispatcher
}

// NewWrapperDispatcherWithResolvers returns a WrapperDispatcher with injected
// router-provider resolvers for tests or alternate boot seams.
func NewWrapperDispatcherWithResolvers(
	cfg WrapperConfig,
	exec ExecFunc,
	resolvers WrapperResolvers,
) *WrapperDispatcher {
	dispatcher := NewWrapperDispatcher(cfg, exec)
	if resolvers.SecurityGate != nil {
		dispatcher.securityGateResolver = resolvers.SecurityGate
	}
	if resolvers.MacroRunner != nil {
		dispatcher.macroRunnerResolver = resolvers.MacroRunner
	}
	if resolvers.Formatter != nil {
		dispatcher.formatterResolver = resolvers.Formatter
	}

	return dispatcher
}

// Dispatch interprets args and routes to the matched wrapper capability.
//
// Routing precedence (mirrors WrapperDetector.Detect):
//  1. ModePackageGate — pre-scan (via router-resolved gate) → exec → done.
//  2. ModeToolingGate — check for -then; run chain or plain exec.
//  3. ModePassthrough — forward args directly to exec.
//  4. Other modes — return ErrNotImplemented with wrapper context.
func (d *WrapperDispatcher) Dispatch(ctx context.Context, args []string) error {
	cfg, err := d.loadConfig()
	if err != nil {
		return fmt.Errorf("dispatcher: load config: %w", err)
	}

	macroNames := collectMacroNames(cfg)
	mode := d.detector.Detect(args, macroNames)

	switch mode {
	case ModePackageGate:
		return d.dispatchPackageGate(ctx, args)
	case ModeToolingGate:
		return d.dispatchToolingGate(ctx, args)
	case ModeMacroRun:
		return d.dispatchMacroRun(ctx, args)
	case ModeFormatHeaders:
		return d.dispatchFormatHeaders(ctx, args)
	case ModePassthrough:
		return d.dispatchPassthrough(ctx, args)
	default:
		return fmt.Errorf("dispatcher: mode %v not yet implemented: %w", mode, errNotImplemented)
	}
}

// staticWrapperConfigLoader returns a config loader that always returns cfg.
func staticWrapperConfigLoader(cfg WrapperConfig) func() (WrapperConfig, error) {
	return func() (WrapperConfig, error) {
		return cfg, nil
	}
}

// loadActiveDispatcherConfig returns the wrapper configuration from the host.
func loadActiveDispatcherConfig() (WrapperConfig, error) {
	return loadActiveAdapterConfig()
}

// dispatchPackageGate runs parse → pre-scan → exec for package-install commands.
//
// The security gate is resolved fresh from the router on each call.
func (d *WrapperDispatcher) dispatchPackageGate(ctx context.Context, args []string) error {
	commandArgs, allowRisk, err := parseAllowRiskArgs(args)
	if err != nil {
		return fmt.Errorf("dispatcher: package gate: parse allow-risk: %w", err)
	}

	req, err := ParseInstallRequest(commandArgs)
	if err != nil {
		return fmt.Errorf("dispatcher: package gate: parse: %w", err)
	}

	gate, err := d.securityGateResolver()
	if err != nil {
		return fmt.Errorf("dispatcher: package gate: resolve security gate: %w", err)
	}

	if err := d.checkGateWithRiskOverride(ctx, allowRisk, gate.CheckPackages, string(req.Ecosystem), req.Packages); err != nil {
		return fmt.Errorf("dispatcher: package gate: pre-install scan: %w", err)
	}

	if err := d.exec(ctx, commandArgs); err != nil {
		return fmt.Errorf("dispatcher: package gate: exec: %w", err)
	}

	if err := d.checkLockfileWithRiskOverride(ctx, allowRisk, gate, string(req.Ecosystem), req.LockfileHint); err != nil {
		return fmt.Errorf("dispatcher: package gate: post-install scan: %w", err)
	}

	return nil
}

func (d *WrapperDispatcher) checkGateWithRiskOverride(
	ctx context.Context,
	allowRisk string,
	check func(context.Context, string, []string) error,
	ecosystem string,
	packages []string,
) error {
	err := check(ctx, ecosystem, packages)
	if err == nil {
		return nil
	}

	return resolveRiskOverride(allowRisk, err)
}

func (d *WrapperDispatcher) checkLockfileWithRiskOverride(
	ctx context.Context,
	allowRisk string,
	gate ports.CLIWrapperSecurityGate,
	ecosystem string,
	lockfilePath string,
) error {
	err := gate.CheckLockfile(ctx, ecosystem, lockfilePath)
	if err == nil {
		return nil
	}

	return resolveRiskOverride(allowRisk, err)
}

// dispatchToolingGate handles ModeToolingGate args, splitting on -then when
// present and running the resulting chain.
func (d *WrapperDispatcher) dispatchToolingGate(ctx context.Context, args []string) error {
	gate, main, chained := SplitChain(args)
	if !chained {
		return d.exec(ctx, args)
	}

	if err := RunChain(ctx, gate, main, d.exec); err != nil {
		return fmt.Errorf("dispatcher: tooling gate chain: %w", err)
	}

	return nil
}

// dispatchPassthrough forwards args directly to exec without modification.
func (d *WrapperDispatcher) dispatchPassthrough(ctx context.Context, args []string) error {
	if err := d.exec(ctx, args); err != nil {
		return fmt.Errorf("dispatcher: passthrough: %w", err)
	}

	return nil
}

// dispatchMacroRun resolves and executes a registered wrapper macro.
func (d *WrapperDispatcher) dispatchMacroRun(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("dispatcher: macro run: empty args")
	}

	macroName := args[0]
	if macroName == "run" {
		if len(args) < 2 {
			return fmt.Errorf("dispatcher: macro run: missing macro name")
		}
		macroName = args[1]
	}

	runner, err := d.macroRunnerResolver()
	if err != nil {
		return fmt.Errorf("dispatcher: macro run: resolve macro runner: %w", err)
	}

	if err := runner.RunMacro(ctx, macroName); err != nil {
		return fmt.Errorf("dispatcher: macro run: %w", err)
	}

	return nil
}

// dispatchFormatHeaders resolves and executes the wrapper header formatter.
func (d *WrapperDispatcher) dispatchFormatHeaders(ctx context.Context, args []string) error {
	formatter, err := d.formatterResolver()
	if err != nil {
		return fmt.Errorf("dispatcher: format headers: resolve formatter: %w", err)
	}

	dryRun, only, err := parseFormatHeadersArgs(args)
	if err != nil {
		return fmt.Errorf("dispatcher: format headers: parse args: %w", err)
	}

	if err := formatter.FormatHeaders(ctx, dryRun, only); err != nil {
		return fmt.Errorf("dispatcher: format headers: %w", err)
	}

	return nil
}

// resolveSecurityGate resolves the CLIWrapperSecurityGate from the router.
func resolveSecurityGate() (ports.CLIWrapperSecurityGate, error) {
	raw, err := router.RouterResolveProvider(router.PortCLIWrapperSecurityGate)
	if err != nil {
		return nil, fmt.Errorf("resolve CLIWrapperSecurityGate: %w", err)
	}

	gate, ok := raw.(ports.CLIWrapperSecurityGate)
	if !ok {
		return nil, errors.New("provider does not implement CLIWrapperSecurityGate")
	}

	return gate, nil
}

// resolveMacroRunner resolves the CLIWrapperMacroRunner from the router.
func resolveMacroRunner() (ports.CLIWrapperMacroRunner, error) {
	raw, err := router.RouterResolveProvider(router.PortCLIWrapperMacroRunner)
	if err != nil {
		return nil, fmt.Errorf("resolve CLIWrapperMacroRunner: %w", err)
	}

	runner, ok := raw.(ports.CLIWrapperMacroRunner)
	if !ok {
		return nil, errors.New("provider does not implement CLIWrapperMacroRunner")
	}

	return runner, nil
}

// resolveFormatter resolves the CLIWrapperFormatter from the router.
func resolveFormatter() (ports.CLIWrapperFormatter, error) {
	raw, err := router.RouterResolveProvider(router.PortCLIWrapperFormatter)
	if err != nil {
		return nil, fmt.Errorf("resolve CLIWrapperFormatter: %w", err)
	}

	formatter, ok := raw.(ports.CLIWrapperFormatter)
	if !ok {
		return nil, errors.New("provider does not implement CLIWrapperFormatter")
	}

	return formatter, nil
}

// collectMacroNames extracts registered macro names from the config.
func collectMacroNames(cfg WrapperConfig) []string {
	names := make([]string, len(cfg.Macros))
	for i, m := range cfg.Macros {
		names[i] = m.Name
	}

	return names
}

// parseFormatHeadersArgs parses CLI flags for the header-formatting command.
func parseFormatHeadersArgs(args []string) (bool, []string, error) {
	if len(args) < 2 {
		return false, nil, fmt.Errorf("expected 'fmt headers' or '<tool> fmt headers'")
	}

	startIndex := 3
	if args[0] == "fmt" && args[1] == "headers" {
		startIndex = 2
	} else if len(args) < 3 {
		return false, nil, fmt.Errorf("expected '<tool> fmt headers'")
	}

	dryRun := false
	only := make([]string, 0)

	for index := startIndex; index < len(args); {
		switch args[index] {
		case "--dry-run":
			dryRun = true
			index++
		case "--only":
			index++
			start := len(only)
			for index < len(args) && !strings.HasPrefix(args[index], "--") {
				only = append(only, args[index])
				index++
			}
			if len(only) == start {
				return false, nil, fmt.Errorf("--only requires at least one language")
			}
		default:
			return false, nil, fmt.Errorf("unknown format headers arg %q", args[index])
		}
	}

	return dryRun, only, nil
}

func parseAllowRiskArgs(args []string) ([]string, string, error) {
	filtered := make([]string, 0, len(args))
	allowRisk := ""

	for _, arg := range args {
		if !strings.HasPrefix(arg, "--allow-risk=") {
			filtered = append(filtered, arg)
			continue
		}

		if allowRisk != "" {
			return nil, "", fmt.Errorf("allow-risk may only be provided once")
		}

		allowRisk = strings.TrimPrefix(arg, "--allow-risk=")
		if _, err := ParseSeverity(allowRisk); err != nil {
			return nil, "", fmt.Errorf("invalid allow-risk %q: %w", allowRisk, err)
		}
	}

	return filtered, allowRisk, nil
}

func resolveRiskOverride(allowRisk string, err error) error {
	var blockErr *RiskBlockError
	if !errors.As(err, &blockErr) {
		return err
	}

	allowed, parseErr := IsRiskOverrideAllowed(allowRisk, blockErr.Severity)
	if parseErr != nil {
		return fmt.Errorf("resolve allow-risk override: %w", parseErr)
	}
	if allowed {
		return nil
	}

	if strings.TrimSpace(allowRisk) == "" {
		return fmt.Errorf("%w; use --allow-risk=%s to override", err, strings.ToLower(CanonicalSeverityLabel(blockErr.Severity)))
	}

	return fmt.Errorf(
		"%w; --allow-risk=%s is insufficient for %s",
		err,
		strings.ToLower(allowRisk),
		strings.ToLower(CanonicalSeverityLabel(blockErr.Severity)),
	)
}
