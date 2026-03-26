// internal/cliwrapper/walker.go
// Formats repository file headers for supported source languages in wrapper flows.
// Applies path-comment normalization while preserving language-specific file handling.
package cliwrapper

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

const defaultPythonShebang = "#!/usr/bin/env python3"

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

// HeaderWalkFunc traverses a repository tree.
type HeaderWalkFunc func(root string, walkFn fs.WalkDirFunc) error

// HeaderReadFunc reads one file from disk.
type HeaderReadFunc func(path string) ([]byte, error)

// HeaderModeFunc returns the current mode bits for one file.
type HeaderModeFunc func(path string) (fs.FileMode, error)

// HeaderWriteFunc writes one file back to disk.
type HeaderWriteFunc func(path string, data []byte, mode fs.FileMode) error

// HeaderFileChange records the outcome for one scanned file.
type HeaderFileChange struct {
	Path         string
	Action       string
	PreviousPath string
}

// HeaderRunReport summarizes one header-formatting run.
type HeaderRunReport struct {
	Checked  int
	Modified int
	Skipped  int
	Changes  []HeaderFileChange
}

// HeaderStatus describes the current path-header state for one file.
type HeaderStatus struct {
	Found        bool
	Matches      bool
	ExistingPath string
}

// HeaderWalker applies path-comment headers to repository files.
type HeaderWalker struct {
	Root      string
	Walk      HeaderWalkFunc
	ReadFile  HeaderReadFunc
	FileMode  HeaderModeFunc
	WriteFile HeaderWriteFunc
}

// Run walks the repository and injects or corrects supported path headers.
func (w HeaderWalker) Run(ctx context.Context, dryRun bool, only []string) (HeaderRunReport, error) {
	if w.Walk == nil || w.ReadFile == nil || w.FileMode == nil || w.WriteFile == nil {
		return HeaderRunReport{}, fmt.Errorf("header walker: dependencies are incomplete")
	}

	filter, err := normalizeHeaderFilter(only)
	if err != nil {
		return HeaderRunReport{}, fmt.Errorf("header walker: %w", err)
	}

	report := HeaderRunReport{}
	err = w.Walk(w.Root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", path, walkErr)
		}

		if err := ctx.Err(); err != nil {
			return fmt.Errorf("walk cancelled: %w", err)
		}

		if d.IsDir() {
			return nil
		}

		return w.processHeaderFile(path, filter, dryRun, &report)
	})
	if err != nil {
		return HeaderRunReport{}, err
	}

	if dryRun && report.Modified > 0 {
		return report, fmt.Errorf("fmt headers: %d files would be modified", report.Modified)
	}

	return report, nil
}

// processHeaderFile evaluates and optionally applies headers to a single file.
func (w HeaderWalker) processHeaderFile(path string, filter map[string]bool, dryRun bool, report *HeaderRunReport) error {
	relPath, err := filepath.Rel(w.Root, path)
	if err != nil {
		return fmt.Errorf("rel path %s: %w", path, err)
	}

	relPath = filepath.ToSlash(relPath)
	language, ok := supportedHeaderLanguages[strings.ToLower(filepath.Ext(path))]
	if !ok || !filter[language] || shouldSkipHeaderPath(relPath) {
		return nil
	}

	report.Checked++

	raw, err := w.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	content := string(raw)
	status, err := InspectHeader(content, language, relPath)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", relPath, err)
	}

	if status.Matches {
		report.Skipped++
		return nil
	}

	updated, err := InjectHeader(content, language, relPath)
	if err != nil {
		return fmt.Errorf("inject %s: %w", relPath, err)
	}

	report.Modified++
	report.Changes = append(report.Changes, w.buildHeaderChange(relPath, status))

	if dryRun {
		return nil
	}

	mode, err := w.FileMode(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if err := w.WriteFile(path, []byte(updated), mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// buildHeaderChange constructs the change record for a modified header.
func (w HeaderWalker) buildHeaderChange(relPath string, status HeaderStatus) HeaderFileChange {
	change := HeaderFileChange{
		Path:         relPath,
		Action:       "ADDED",
		PreviousPath: status.ExistingPath,
	}
	if status.Found && status.ExistingPath != "" {
		change.Action = "STALE"
	}
	return change
}

// ResolveRepoRoot returns the nearest repository root marker or startDir when none is found.
func ResolveRepoRoot(startDir string) string {
	for _, marker := range []string{"policy-gate.toml", "wrapper-gate.toml", ".git"} {
		if path := walkUpForFile(startDir, marker); path != "" {
			if filepath.Base(path) == ".git" {
				return filepath.Dir(path)
			}

			return filepath.Dir(path)
		}
	}

	return startDir
}

// InspectHeader reports whether the file contains the expected header.
func InspectHeader(content string, language string, relPath string) (HeaderStatus, error) {
	lines := firstLines(normalizeNewlines(content), 3)
	expected, err := headerComment(language, relPath)
	if err != nil {
		return HeaderStatus{}, err
	}

	switch language {
	case "go", "typescript":
		if len(lines) == 0 {
			return HeaderStatus{}, nil
		}

		path, ok := parseHeaderPath(lines[0], language)
		if !ok {
			return HeaderStatus{}, nil
		}

		return HeaderStatus{
			Found:        true,
			Matches:      path == relPath,
			ExistingPath: path,
		}, nil
	case "python":
		for _, line := range lines {
			path, ok := parseHeaderPath(line, language)
			if !ok {
				continue
			}

			return HeaderStatus{
				Found:        true,
				Matches:      line == expected,
				ExistingPath: path,
			}, nil
		}

		return HeaderStatus{}, nil
	default:
		return HeaderStatus{}, fmt.Errorf("inspect header: unsupported language %q", language)
	}
}

// HasHeader reports whether the expected path header is already present.
func HasHeader(content string, language string, relPath string) bool {
	status, err := InspectHeader(content, language, relPath)
	return err == nil && status.Matches
}

// InjectHeader inserts or corrects the path header for one file.
func InjectHeader(content string, language string, relPath string) (string, error) {
	newline := detectNewline(content)
	lines := splitLines(normalizeNewlines(content))

	switch language {
	case "go", "typescript":
		return injectSlashCommentHeader(lines, newline, relPath), nil
	case "python":
		return injectPythonHeader(lines, newline, relPath), nil
	default:
		return "", fmt.Errorf("inject header: unsupported language %q", language)
	}
}

// normalizeHeaderFilter validates and normalizes the requested language filter set.
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

// shouldSkipHeaderPath reports whether relPath contains a directory excluded from header formatting.
func shouldSkipHeaderPath(relPath string) bool {
	parts := strings.Split(relPath, "/")
	return slices.ContainsFunc(parts[:max(0, len(parts)-1)], func(part string) bool {
		_, ok := skippedHeaderDirs[part]
		return ok
	})
}

// injectSlashCommentHeader inserts or replaces the leading path header for Go and TypeScript files.
func injectSlashCommentHeader(lines []string, newline string, relPath string) string {
	header := "// " + relPath
	body := append([]string(nil), lines...)
	if len(body) > 0 {
		if _, ok := parseHeaderPath(body[0], "go"); ok {
			body = body[1:]
		}
	}

	return joinLines(append([]string{header}, body...), newline)
}

// injectPythonHeader inserts or replaces the shebang-aware path header for Python files.
func injectPythonHeader(lines []string, newline string, relPath string) string {
	body := append([]string(nil), lines...)
	shebang := defaultPythonShebang
	if len(body) > 0 && strings.HasPrefix(body[0], "#!") {
		shebang = body[0]
		body = body[1:]
	}

	if len(body) > 0 {
		if _, ok := parseHeaderPath(body[0], "python"); ok {
			body = body[1:]
		}
	}

	return joinLines(append([]string{shebang, "# " + relPath}, body...), newline)
}

// headerComment renders the expected path header comment for one supported language.
func headerComment(language string, relPath string) (string, error) {
	switch language {
	case "go", "typescript":
		return "// " + relPath, nil
	case "python":
		return "# " + relPath, nil
	default:
		return "", fmt.Errorf("header comment: unsupported language %q", language)
	}
}

// parseHeaderPath extracts a repo-relative header path from one language-specific comment line.
func parseHeaderPath(line string, language string) (string, bool) {
	switch language {
	case "go", "typescript":
		if !strings.HasPrefix(line, "// ") {
			return "", false
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "// ")), true
	case "python":
		if !strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "#!/") {
			return "", false
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "# ")), true
	default:
		return "", false
	}
}

// firstLines returns up to count leading logical lines from content.
func firstLines(content string, count int) []string {
	lines := splitLines(content)
	if len(lines) > count {
		return lines[:count]
	}

	return lines
}

// splitLines returns content split on normalized newlines without a trailing empty record.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

// detectNewline reports the preferred newline sequence for rewritten output.
func detectNewline(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}

	return "\n"
}

// normalizeNewlines rewrites CRLF input to LF for stable header processing.
func normalizeNewlines(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

// joinLines reassembles lines using the selected newline sequence and a trailing newline.
func joinLines(lines []string, newline string) string {
	return strings.Join(lines, newline) + newline
}
