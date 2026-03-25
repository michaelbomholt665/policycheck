// internal/policycheck/utils/helpers.go
// General-purpose utilities for path handling and line counting.

package utils

const ScopeProjectRepo = true


import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// RelOrAbs returns a path relative to the root, or the absolute path if outside the root.
func RelOrAbs(root, filePath string) string {
	rel, err := filepath.Rel(root, filePath)
	if err != nil {
		return filepath.ToSlash(filePath)
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(filePath)
	}
	return rel
}

// HasPrefix returns true if the value starts with any of the provided prefixes, path-aware.
func HasPrefix(value string, prefixes []string) bool {
	normalizedValue := NormalizePolicyPath(value)
	for _, prefix := range prefixes {
		normalizedPrefix := NormalizePolicyPath(prefix)
		if normalizedPrefix == "" {
			continue
		}
		if normalizedValue == normalizedPrefix || strings.HasPrefix(normalizedValue, normalizedPrefix+"/") {
			return true
		}
	}
	return false
}

// NormalizePolicyPath cleans and normalizes a path for consistent prefix comparison.
func NormalizePolicyPath(value string) string {
	cleaned := path.Clean(filepath.ToSlash(value))
	if cleaned == "." {
		return ""
	}
	return strings.TrimPrefix(cleaned, "./")
}

// MaxInt returns the larger of two integers.
func MaxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

// CountLines counts the total number of lines in a byte slice.
func CountLines(content []byte) int {
	lineCount := bytes.Count(content, []byte{'\n'})
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lineCount++
	}
	return lineCount
}

// PathExists returns true if the given filesystem path exists.
func PathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
