// internal/router/tools/wrlk/main.go
// Provides the CLI entrypoint, flag parsing, and command dispatch
// for the wrlk router scaffolding tool.

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

const (
	exitCodeSuccess     = 0
	exitCodeFailure     = 1
	exitCodeUsage       = 2
	exitCodeInternalBug = 3
	defaultRootPath     = "."
	defaultLiveTimeout  = "15s"
)

// main runs the Router CLI entrypoint.
func main() {
	os.Exit(RouterRunCLIProcess(os.Args[1:], os.Stdout, os.Stderr))
}

// routerWriteAndReturn writes a formatted message to w; if the write fails it
// returns exitCodeInternalBug, otherwise it returns successCode.
func routerWriteAndReturn(w io.Writer, successCode int, format string, args ...any) int {
	if err := RouterWriteCLIMessage(w, format, args...); err != nil {
		return exitCodeInternalBug
	}

	return successCode
}

// RouterRunCLIProcess executes the CLI and returns a process exit code.
func RouterRunCLIProcess(args []string, stdout io.Writer, stderr io.Writer) int {
	stdout, stderr = RouterNormalizeCLIWriters(stdout, stderr)

	if handledCode, handled := RouterHandleTopLevelUsage(args, stdout, stderr); handled {
		return handledCode
	}

	options, remainingArgs, parseCode, handled := RouterPrepareCLICommand(args, stdout, stderr)
	if handled {
		return parseCode
	}

	err := RouterDispatchCLICommand(options, remainingArgs, stdout, stderr)
	return RouterMapCommandResult(err, stderr)
}

// RouterNormalizeCLIWriters replaces nil CLI writers with io.Discard.
func RouterNormalizeCLIWriters(stdout io.Writer, stderr io.Writer) (io.Writer, io.Writer) {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	return stdout, stderr
}

// RouterPrepareCLICommand parses global flags and handles usage-only command paths.
func RouterPrepareCLICommand(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) (globalOptions, []string, int, bool) {
	options, remainingArgs, err := RouterParseGlobalOptions(args)
	if err != nil {
		return RouterHandleGlobalParseError(err, stdout, stderr)
	}

	commandCode, handled := RouterHandleRemainingCommandUsage(remainingArgs, stdout, stderr)
	if handled {
		return globalOptions{}, nil, commandCode, true
	}

	return options, remainingArgs, exitCodeSuccess, false
}

// RouterHandleGlobalParseError maps global flag parse failures to process exit behavior.
func RouterHandleGlobalParseError(
	err error,
	stdout io.Writer,
	stderr io.Writer,
) (globalOptions, []string, int, bool) {
	if errors.Is(err, flag.ErrHelp) {
		if usageErr := RouterWriteCLIUsage(stdout); usageErr != nil {
			return globalOptions{}, nil, exitCodeInternalBug, true
		}

		return globalOptions{}, nil, exitCodeSuccess, true
	}

	return globalOptions{}, nil, routerWriteAndReturn(stderr, exitCodeUsage, "Router usage error: %s\n", err), true
}

// RouterHandleRemainingCommandUsage handles help and missing-command cases after global parsing.
func RouterHandleRemainingCommandUsage(args []string, stdout io.Writer, stderr io.Writer) (int, bool) {
	if len(args) > 0 && RouterIsHelpToken(args[0]) {
		if usageErr := RouterWriteCLIUsage(stdout); usageErr != nil {
			return exitCodeInternalBug, true
		}

		return exitCodeSuccess, true
	}

	if len(args) == 0 {
		if usageErr := RouterWriteCLIUsage(stderr); usageErr != nil {
			return exitCodeInternalBug, true
		}

		return exitCodeUsage, true
	}

	return exitCodeSuccess, false
}

type globalOptions struct {
	root string
}

// RouterParseGlobalOptions parses flags shared by all command groups.
func RouterParseGlobalOptions(args []string) (globalOptions, []string, error) {
	options := globalOptions{}

	fs := flag.NewFlagSet("Router", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&options.root, "root", defaultRootPath, "repository root")

	if err := fs.Parse(args); err != nil {
		return globalOptions{}, nil, fmt.Errorf("parse global flags: %w", err)
	}

	return options, fs.Args(), nil
}

// RouterDispatchCLICommand routes the parsed command tree to a concrete handler.
func RouterDispatchCLICommand(
	options globalOptions,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	switch args[0] {
	case "lock":
		return RouterRunLockCommand(options, args[1:], stdout)
	case "live":
		return RouterRunLiveCommand(options, args[1:], stdout, stderr)
	case "module":
		return RouterRunModuleCommand(options, args[1:], stdout)
	case "register":
		return RouterRunRegisterCommand(options, args[1:], stdout)
	case "add":
		return RouterRunPortgenCommand(options, args[1:], stdout, stderr)
	case "ext":
		return RouterRunExtCommand(options, args[1:], stdout, stderr)
	case "guide":
		return RouterRunGuideCommand(options, args[1:], stdout)
	default:
		return &usageError{message: fmt.Sprintf("unknown command %q", args[0])}
	}
}

// RouterWriteCLIUsage prints the top-level CLI usage message.
func RouterWriteCLIUsage(writer io.Writer) error {
	return RouterWriteCLILines(writer, "write CLI usage line", []string{
		"usage: Router [--root PATH] <command> <subcommand> [flags]",
		"commands:",
		"  lock verify",
		"  lock update",
		"  lock restore",
		"  live run",
		"  module sync",
		"  register",
		"  add",
		"  ext add",
		"  ext install",
		"  ext remove",
		"  ext app add",
		"  ext app remove",
		"  guide",
		"  guide current",
	})
}

// RouterWriteGuide prints a concise operational guide for the router tooling.
func RouterWriteGuide(writer io.Writer) error {
	lines := []string{
		"Router guide:",
		"",
		"Workflow:",
		"  1. If this router bundle was copied into a new Go module, run `wrlk module sync` once.",
		"  2. Register router-owned ports with `wrlk register --port --router --name <PortName> --value <port-name>`.",
		"  3. Add an extension package under `internal/router/ext/extensions/<name>/` only for router-native optional capabilities.",
		"  4. Register optional router extensions with `wrlk register --ext --router --name <ExtensionName>`.",
		"  5. Register required app extensions with `wrlk register --ext --app --name <ExtensionName>`.",
		"  6. Run `wrlk lock verify` before and after changes to catch drift in protected router files.",
		"  7. Run `wrlk lock update` only after intentional router core changes are reviewed and accepted.",
		"  8. Run `wrlk lock restore` to put protected files back to the last local snapshot written by the supported manifest-backed workflow.",
		"",
		"Choose the command deliberately:",
		"  - `wrlk module sync` is a one-time bootstrap command for copied router bundles. It rewrites bundled imports from the source module to the current `go.mod` module path.",
		"  - `wrlk register --port --router` appends to `router_manifest.go` and regenerates `ports.go` plus `registry_imports.go`.",
		"  - `wrlk register --ext --router` appends to `router_manifest.go` and regenerates `optional_extensions.go`.",
		"  - `wrlk register --ext --app` appends to `app_manifest.go` and regenerates `extensions.go`.",
		"  - `wrlk guide current` prints the currently wired ports and extension inventory for the target root.",
		"  - `wrlk lock verify` checks checksum-tracked router core files for drift.",
		"  - `wrlk lock update` refreshes the lock file after accepted intentional core changes.",
		"  - `wrlk lock restore` restores the previous local snapshot; it is the rollback tool for CLI mutations, not for runtime boot.",
		"",
		"Structure the router expects:",
		"  - The router core is intentionally minimal. It is not a framework and should stay contract-blind.",
		"  - Providers must be registered only from extensions through `RouterProvideRegistration`; do not register providers ad hoc in app startup code.",
		"  - Consumers resolve by port from the published registry, then cast to the port contract they expect.",
		"  - `Provides()` declares the exact ports an extension will register during boot.",
		"  - `Provides()` must match what `RouterProvideRegistration` actually registers.",
		"  - Duplicate `Provides()` across extensions are invalid and fail dependency graph construction.",
		"  - `Consumes()` declares required boot-time port dependencies and drives boot ordering.",
		"  - If extension B consumes a port provided by extension A, A boots before B.",
		"  - If a consumed port is not registered when an extension boots, boot fails with a dependency-order error.",
		"",
		"Required vs optional:",
		"  - `internal/router/ext/extensions.go` is generated from `app_manifest.go` and may be empty when the app has no required adapters to boot there.",
		"  - `internal/router/ext/optional_extensions.go` is only for optional capability extensions such as telemetry or metrics.",
		"  - Optional extension failures produce warnings and boot continues.",
		"  - Required extension failures fail boot.",
		"  - Boot rollback is boot-only. `RouterRollbackBoot` undoes startup work for aborted boot attempts; it is not full runtime shutdown management.",
		"",
		"Short examples:",
		"  - Add a port: `wrlk register --port --router --name PortTelemetry --value telemetry`",
		"  - Wire an existing optional extension package: `wrlk register --ext --router --name telemetry`",
		"  - Wire an existing application adapter: `wrlk register --ext --app --name postgres`",
		"  - Simple provider extension:",
		"      func (e *Extension) Provides() []router.PortName { return []router.PortName{router.PortTelemetry} }",
		"      func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {",
		"          if err := reg.RouterRegisterProvider(router.PortTelemetry, provider); err != nil { return fmt.Errorf(\"telemetry extension: %w\", err) }",
		"          return nil",
		"      }",
		"  - Dependent extension:",
		"      func (e *Extension) Consumes() []router.PortName { return []router.PortName{router.PortTelemetry} }",
		"      func (e *Extension) Provides() []router.PortName { return []router.PortName{router.PortPostgres} }",
		"  - Bad example: `Provides()` returns `router.PortTelemetry` but registration writes `router.PortOptional`, or two extensions both provide the same port. That is wrong because ordering and duplicate detection rely on truthful `Provides()` declarations.",
		"",
		"Safe editing rules:",
		"  - Change router core files only when the contract surface itself must change.",
		"  - Most feature work should change an extension package plus one composition file, not the router core.",
		"  - `Any` is acceptable only in contract-blind infrastructure or explicit relaxed policy wiring, not business logic.",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write router guide line: %w", err)
		}
	}

	return nil
}

// RouterHandleTopLevelUsage handles top-level help and empty-argument usage flows.
func RouterHandleTopLevelUsage(args []string, stdout io.Writer, stderr io.Writer) (int, bool) {
	return RouterHandleRemainingCommandUsage(args, stdout, stderr)
}

// RouterMapCommandResult converts a command error into the correct process exit code.
func RouterMapCommandResult(err error, stderr io.Writer) int {
	if err == nil {
		return exitCodeSuccess
	}

	var bugErr *verificationBugError
	if errors.As(err, &bugErr) {
		return routerWriteAndReturn(stderr, exitCodeInternalBug, "Router internal failure: %s\n", err)
	}

	var usageErr *usageError
	if errors.As(err, &usageErr) {
		return routerWriteAndReturn(stderr, exitCodeUsage, "Router usage error: %s\n", err)
	}

	return routerWriteAndReturn(stderr, exitCodeFailure, "%s\n", err)
}

// RouterWriteCLIMessage writes one formatted CLI message.
func RouterWriteCLIMessage(writer io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(writer, format, args...); err != nil {
		return fmt.Errorf("write CLI message: %w", err)
	}

	return nil
}

// RouterWriteCLILines writes a sequence of CLI lines using one shared error context.
func RouterWriteCLILines(writer io.Writer, context string, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("%s: %w", context, err)
		}
	}

	return nil
}

type usageError struct {
	message string
}

// Error returns the usage error message.
func (e *usageError) Error() string {
	return e.message
}

// RouterIsHelpToken reports whether the provided argument is a conventional help token.
func RouterIsHelpToken(value string) bool {
	switch value {
	case "--help", "-h", "help":
		return true
	default:
		return false
	}
}
