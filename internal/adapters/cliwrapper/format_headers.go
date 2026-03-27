// internal/adapters/cliwrapper/format_headers.go
package cliwrapper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	core "policycheck/internal/cliwrapper"
	"policycheck/internal/ports"
	"policycheck/internal/router"
)

// HeaderFormatterAdapter formats path headers using the router walk capability.
type HeaderFormatterAdapter struct {
	resolveWalk func() (ports.WalkProvider, error)
	resolveRoot func() (string, error)
	readFile    func(string) ([]byte, error)
	fileMode    func(string) (fs.FileMode, error)
	writeFile   func(string, []byte, fs.FileMode) error
	output      io.Writer
}

// HeaderFormatterDeps holds the dependency seams for the header formatter adapter.
type HeaderFormatterDeps struct {
	ResolveWalk func() (ports.WalkProvider, error)
	ResolveRoot func() (string, error)
	ReadFile    func(string) ([]byte, error)
	FileMode    func(string) (fs.FileMode, error)
	WriteFile   func(string, []byte, fs.FileMode) error
	Output      io.Writer
}

// NewHeaderFormatterAdapter returns a formatter with production defaults.
func NewHeaderFormatterAdapter() *HeaderFormatterAdapter {
	return NewHeaderFormatterAdapterWithDeps(HeaderFormatterDeps{
		ResolveWalk: resolveWalkProvider,
		ResolveRoot: resolveHeaderRoot,
		ReadFile:    os.ReadFile,
		FileMode: func(path string) (fs.FileMode, error) {
			info, err := os.Stat(path)
			if err != nil {
				return 0, err
			}

			return info.Mode(), nil
		},
		WriteFile: func(path string, data []byte, mode fs.FileMode) error {
			return os.WriteFile(path, data, mode)
		},
		Output: os.Stdout,
	})
}

// NewHeaderFormatterAdapterWithDeps returns a formatter with injected seams for tests.
func NewHeaderFormatterAdapterWithDeps(deps HeaderFormatterDeps) *HeaderFormatterAdapter {
	return &HeaderFormatterAdapter{
		resolveWalk: deps.ResolveWalk,
		resolveRoot: deps.ResolveRoot,
		readFile:    deps.ReadFile,
		fileMode:    deps.FileMode,
		writeFile:   deps.WriteFile,
		output:      deps.Output,
	}
}

// FormatHeaders scans the repository and injects or corrects path-comment headers.
func (a *HeaderFormatterAdapter) FormatHeaders(ctx context.Context, dryRun bool, list bool, only []string) error {
	walkProvider, err := a.resolveWalk()
	if err != nil {
		return fmt.Errorf("format headers adapter: resolve walk provider: %w", err)
	}

	root, err := a.resolveRoot()
	if err != nil {
		return fmt.Errorf("format headers adapter: resolve repo root: %w", err)
	}

	report, err := core.HeaderWalker{
		Root:      root,
		Walk:      walkProvider.WalkDirectoryTree,
		ReadFile:  a.readFile,
		FileMode:  a.fileMode,
		WriteFile: a.writeFile,
	}.Run(ctx, dryRun, only)
	if list {
		if listErr := writeHeaderList(a.output, report.Changes); listErr != nil {
			return fmt.Errorf("format headers adapter: write list: %w", listErr)
		}
	}
	if err != nil {
		return fmt.Errorf("format headers adapter: %w", err)
	}

	return nil
}

func writeHeaderList(output io.Writer, changes []core.HeaderFileChange) error {
	if output == nil || len(changes) == 0 {
		return nil
	}

	for _, change := range changes {
		if _, err := fmt.Fprintln(output, change.Path); err != nil {
			return err
		}
	}

	return nil
}

func resolveWalkProvider() (ports.WalkProvider, error) {
	raw, err := router.RouterResolveProvider(router.PortWalk)
	if err != nil {
		return nil, fmt.Errorf("resolve walk provider: %w", err)
	}

	walkProvider, ok := raw.(ports.WalkProvider)
	if !ok {
		return nil, errors.New("provider does not implement WalkProvider")
	}

	return walkProvider, nil
}

func resolveHeaderRoot() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	return core.ResolveRepoRoot(workingDir), nil
}
