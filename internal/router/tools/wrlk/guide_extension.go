package main

import (
	"fmt"
	"io"
)

// RouterWriteExtensionGuide prints a detailed authoring guide for router extensions.
func RouterWriteExtensionGuide(writer io.Writer) error {
	lines := []string{
		"Router extension authoring guide:",
		"",
		"When to use this:",
		"  - Use an optional capability extension when the feature adds cross-cutting infrastructure and boot should continue if it fails.",
		"  - Wire optional capability extensions through `internal/router/ext/optional_extensions.go`.",
		"  - Place the extension package in `internal/router/ext/extensions/<name>/`.",
		"",
		"Working pattern:",
		"  1. Add a dedicated port with `wrlk add --name <PortName> --value <port-value>`.",
		"  2. Define the app-facing contract in `internal/ports/` so consumers cast to a stable interface, not a concrete extension type.",
		"  3. Scaffold the optional extension with `wrlk ext add --name <name>`.",
		"  4. Implement the provider inside `internal/router/ext/extensions/<name>/` and register it from `RouterProvideRegistration`.",
		"  5. Keep `Provides()` exactly aligned with the port you register.",
		"  6. Keep `Consumes()` truthful; if the extension needs another port at boot, declare it there.",
		"  7. Document the extension in `doc.go` so `wrlk guide current` can surface what it does and how to consume it.",
		"",
		"Worked example: go-pretty CLI styling capability",
		"  Goal: expose a reusable table-rendering capability to the app without hard-coding `go-pretty` into unrelated packages.",
		"",
		"  Step 1: add the port",
		"    `go run ./internal/router/tools/wrlk add --name PortCLIStyle --value cli-style`",
		"",
		"  Step 2: add the app-facing contract",
		"    Create `internal/ports/cli_style.go` with an interface such as:",
		"      type CLIStyleProvider interface {",
		"          RenderTable(headers []string, rows [][]string) (string, error)",
		"      }",
		"    The app resolves by `router.PortCLIStyle` and casts to `ports.CLIStyleProvider`.",
		"    Names such as `header`, `info`, `success`, `warning`, `error`, and `muted` are canonical semantic roles owned by the router contract.",
		"    If a library uses different style headers or option names, convert to them inside the extension instead of leaking those names to consumers.",
		"",
		"  Step 3: scaffold the optional extension",
		"    `go run ./internal/router/tools/wrlk ext add --name prettystyle`",
		"",
		"  Step 4: implement the provider in `internal/router/ext/extensions/prettystyle/`",
		"    - Add any renderer dependency to the extension only, never to the router core.",
		"    - Create a provider type that satisfies `ports.CLIStyleProvider`.",
		"    - Convert canonical router capability roles into the renderer's established patterns inside the extension.",
		"    - Build the table output inside the provider, not in the consumer.",
		"",
		"  Step 5: register it from the extension",
		"    Required() should return false.",
		"    Consumes() should return nil unless the formatter needs another router capability at boot.",
		"    Provides() should return `[]router.PortName{router.PortCLIStyle}`.",
		"    RouterProvideRegistration should instantiate the provider and call `reg.RouterRegisterProvider(router.PortCLIStyle, provider)`.",
		"",
		"  Step 6: consume it from the app",
		"    Resolve the provider with `router.RouterResolveProvider(router.PortCLIStyle)` and cast to `ports.CLIStyleProvider`.",
		"    If the cast fails, return `PortContractMismatch`.",
		"    If the port is missing, degrade gracefully because this is an optional capability extension.",
		"",
		"Rules that keep the extension usable by the app:",
		"  - The router port is the discovery key.",
		"  - The interface in `internal/ports/` is the app contract.",
		"  - The extension package owns the concrete implementation details.",
		"  - Canonical capability names stay in the router contract; the extension translates them to library-specific style names or options.",
		"  - Consumers must never import the concrete extension package just to use the capability.",
		"  - Optional extensions should return warnings on boot failure, not take down the app.",
		"",
		"Verification loop:",
		"  - `go run ./internal/router/tools/wrlk guide current`",
		"  - `go run ./internal/router/tools/wrlk lock verify`",
		"  - Boot the app and resolve the port from one real consumer path.",
		"",
		"Reference doc:",
		"  - `docs/documentation/extensions.md`",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write extension guide line: %w", err)
		}
	}

	return nil
}
