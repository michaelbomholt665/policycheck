package cliwrapper

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
}

// NewHeaderFormatterAdapter returns a formatter with production defaults.
func NewHeaderFormatterAdapter() *HeaderFormatterAdapter {
	return NewHeaderFormatterAdapterWithDeps(
		resolveWalkProvider,
		resolveHeaderRoot,
		os.ReadFile,
		func(path string) (fs.FileMode, error) {
			info, err := os.Stat(path)
			if err != nil {
				return 0, err
			}

			return info.Mode(), nil
		},
		func(path string, data []byte, mode fs.FileMode) error {
			return os.WriteFile(path, data, mode)
		},
	)
}

// NewHeaderFormatterAdapterWithDeps returns a formatter with injected seams for tests.
func NewHeaderFormatterAdapterWithDeps(
	resolveWalk func() (ports.WalkProvider, error),
	resolveRoot func() (string, error),
	readFile func(string) ([]byte, error),
	fileMode func(string) (fs.FileMode, error),
	writeFile func(string, []byte, fs.FileMode) error,
) *HeaderFormatterAdapter {
	return &HeaderFormatterAdapter{
		resolveWalk: resolveWalk,
		resolveRoot: resolveRoot,
		readFile:    readFile,
		fileMode:    fileMode,
		writeFile:   writeFile,
	}
}

// FormatHeaders scans the repository and injects or corrects path-comment headers.
func (a *HeaderFormatterAdapter) FormatHeaders(ctx context.Context, dryRun bool, only []string) error {
	walkProvider, err := a.resolveWalk()
	if err != nil {
		return fmt.Errorf("format headers adapter: resolve walk provider: %w", err)
	}

	root, err := a.resolveRoot()
	if err != nil {
		return fmt.Errorf("format headers adapter: resolve repo root: %w", err)
	}

	if err := runHeaderFormatting(ctx, headerFormattingDeps{
		root:      root,
		walk:      walkProvider.WalkDirectoryTree,
		readFile:  a.readFile,
		fileMode:  a.fileMode,
		writeFile: a.writeFile,
		dryRun:    dryRun,
		only:      only,
	}); err != nil {
		return fmt.Errorf("format headers adapter: %w", err)
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

	return resolveAdapterRepoRoot(workingDir), nil
}

type headerFormattingDeps struct {
	root      string
	walk      func(string, fs.WalkDirFunc) error
	readFile  func(string) ([]byte, error)
	fileMode  func(string) (fs.FileMode, error)
	writeFile func(string, []byte, fs.FileMode) error
	dryRun    bool
	only      []string
}

var supportedHeaderLanguages = map[string]string{
	".go":  "go",
	".py":  "python",
	".ts":  "typescript",
	".tsx": "typescript",
}

var skippedHeaderDirs = map[string]struct{}{
	".git":         {},
	".venv":        {},
	"venv":         {},
	"node_modules": {},
	"__pycache__":  {},
	".mypy_cache":  {},
	".ruff_cache":  {},
	"dist":         {},
	"build":        {},
	"vendor":       {},
}

func runHeaderFormatting(ctx context.Context, deps headerFormattingDeps) error {
	filter, err := normalizeHeaderFilter(deps.only)
	if err != nil {
		return err
	}

	modified := 0
	err = deps.walk(deps.root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", path, walkErr)
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("walk cancelled: %w", err)
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(deps.root, path)
		if err != nil {
			return fmt.Errorf("rel path %s: %w", path, err)
		}
		relPath = filepath.ToSlash(relPath)

		language, ok := supportedHeaderLanguages[strings.ToLower(filepath.Ext(path))]
		if !ok || !filter[language] || shouldSkipHeaderPath(relPath) {
			return nil
		}

		content, err := deps.readFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		updated, changed, err := applyHeaderForPath(string(content), language, relPath)
		if err != nil {
			return fmt.Errorf("apply header %s: %w", relPath, err)
		}
		if !changed {
			return nil
		}

		modified++
		if deps.dryRun {
			return nil
		}

		mode, err := deps.fileMode(path)
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if err := deps.writeFile(path, []byte(updated), mode); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	if deps.dryRun && modified > 0 {
		return fmt.Errorf("fmt headers: %d files would be modified", modified)
	}

	return nil
}

func normalizeHeaderFilter(only []string) (map[string]bool, error) {
	filter := map[string]bool{
		"go":         true,
		"python":     true,
		"typescript": true,
	}
	if len(only) == 0 {
		return filter, nil
	}

	for key := range filter {
		filter[key] = false
	}
	for _, item := range only {
		name := strings.ToLower(strings.TrimSpace(item))
		if _, ok := filter[name]; !ok {
			return nil, fmt.Errorf("unsupported language %q", item)
		}
		filter[name] = true
	}

	return filter, nil
}

func shouldSkipHeaderPath(relPath string) bool {
	parts := strings.Split(relPath, "/")
	for _, part := range parts[:max(0, len(parts)-1)] {
		if _, ok := skippedHeaderDirs[part]; ok {
			return true
		}
	}

	return false
}

func applyHeaderForPath(content string, language string, relPath string) (string, bool, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	newline := "\n"
	if strings.Contains(content, "\r\n") {
		newline = "\r\n"
	}

	lines := splitContentLines(normalized)
	switch language {
	case "go", "typescript":
		return applySlashHeader(content, lines, relPath, newline)
	case "python":
		return applyPythonHeader(content, lines, relPath, newline)
	default:
		return "", false, fmt.Errorf("unsupported language %q", language)
	}
}

func applySlashHeader(content string, lines []string, relPath string, newline string) (string, bool, error) {
	expected := "// " + relPath
	if len(lines) > 0 && lines[0] == expected {
		return content, false, nil
	}
	if len(lines) > 0 && strings.HasPrefix(lines[0], "// ") {
		lines = lines[1:]
	}

	return joinContentLines(append([]string{expected}, lines...), newline), true, nil
}

func applyPythonHeader(content string, lines []string, relPath string, newline string) (string, bool, error) {
	const shebang = "#!/usr/bin/env python3"

	body := append([]string(nil), lines...)
	header := "# " + relPath
	currentShebang := shebang
	if len(body) > 0 && strings.HasPrefix(body[0], "#!") {
		currentShebang = body[0]
		body = body[1:]
	}
	if len(body) > 0 && body[0] == header && currentShebang == firstShebang(lines, shebang) {
		return content, false, nil
	}
	if len(body) > 0 && strings.HasPrefix(body[0], "# ") && !strings.HasPrefix(body[0], "#!/") {
		body = body[1:]
	}

	return joinContentLines(append([]string{currentShebang, header}, body...), newline), true, nil
}

func splitContentLines(content string) []string {
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

func joinContentLines(lines []string, newline string) string {
	return strings.Join(lines, newline) + newline
}

func firstShebang(lines []string, fallback string) string {
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		return lines[0]
	}

	return fallback
}

func resolveAdapterRepoRoot(startDir string) string {
	current := filepath.Clean(startDir)
	for {
		for _, marker := range []string{"policy-gate.toml", "wrapper-gate.toml", ".git"} {
			candidate := filepath.Join(current, marker)
			info, err := os.Stat(candidate)
			if err == nil {
				if info.IsDir() && marker == ".git" {
					return current
				}
				if !info.IsDir() {
					return current
				}
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			return startDir
		}

		current = parent
	}
}
