// internal/policycheck/utils/utils.go
// Package utils provides common helper functions used across the policycheck codebase.
// These utilities handle path normalization, pluralization, and AST metadata extraction.
package utils

import (
	"go/ast"
	"path"
	"strings"
)

var irregularPluralVerbs = map[string]string{
	"does": "do",
	"has":  "have",
	"is":   "are",
	"was":  "were",
}

// NormalizePolicyPath normalizes a policy path for comparison.
func NormalizePolicyPath(value string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	cleaned := path.Clean(normalized)
	if cleaned == "." {
		return ""
	}

	return strings.TrimPrefix(cleaned, "./")
}

// IsPolicyAbsPath returns true for both native absolute paths and Windows-style
// absolute paths, even when the current host OS is not Windows.
func IsPolicyAbsPath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "/") {
		return true
	}
	if strings.HasPrefix(trimmed, `\\`) || strings.HasPrefix(trimmed, "//") {
		return true
	}
	if len(trimmed) < 3 {
		return false
	}
	drive := trimmed[0]
	if !((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) {
		return false
	}
	if trimmed[1] != ':' {
		return false
	}
	return trimmed[2] == '\\' || trimmed[2] == '/'
}

// HasPrefix returns true if the string starts with any of the provided prefixes,
// accounting for slash-delimited path boundaries.
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

// ToSlashRel returns the slash-normalized path of target relative to root.
func ToSlashRel(root, target string) string {
	if !IsPolicyAbsPath(target) {
		return NormalizePolicyPath(target)
	}
	if !IsPolicyAbsPath(root) {
		return NormalizePolicyPath(target)
	}

	normalizedRoot := NormalizePolicyPath(root)
	normalizedTarget := NormalizePolicyPath(target)

	rootVolume, rootPath, rootHasVolume := splitPolicyVolume(normalizedRoot)
	targetVolume, targetPath, targetHasVolume := splitPolicyVolume(normalizedTarget)
	if rootHasVolume || targetHasVolume {
		if !rootHasVolume || !targetHasVolume || !strings.EqualFold(rootVolume, targetVolume) {
			return normalizedTarget
		}
		rel, err := relativePolicyPath(rootPath, targetPath)
		if err != nil {
			return normalizedTarget
		}
		return NormalizePolicyPath(rel)
	}

	rel, err := relativePolicyPath(normalizedRoot, normalizedTarget)
	if err != nil {
		return normalizedTarget
	}

	return NormalizePolicyPath(rel)
}

// splitPolicyVolume extracts a Windows drive prefix from a normalized path.
func splitPolicyVolume(value string) (string, string, bool) {
	if len(value) < 3 {
		return "", value, false
	}
	drive := value[0]
	if !((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) || value[1] != ':' || value[2] != '/' {
		return "", value, false
	}
	return strings.ToUpper(value[:2]), value[2:], true
}

// relativePolicyPath computes a slash-normalized relative path without relying
// on host-specific filepath semantics.
func relativePolicyPath(root, target string) (string, error) {
	rootParts := splitPolicyPath(root)
	targetParts := splitPolicyPath(target)

	commonLength := 0
	for commonLength < len(rootParts) && commonLength < len(targetParts) && rootParts[commonLength] == targetParts[commonLength] {
		commonLength++
	}

	relativeParts := make([]string, 0, len(rootParts)-commonLength+len(targetParts)-commonLength)
	for idx := commonLength; idx < len(rootParts); idx++ {
		relativeParts = append(relativeParts, "..")
	}
	relativeParts = append(relativeParts, targetParts[commonLength:]...)
	if len(relativeParts) == 0 {
		return ".", nil
	}

	return strings.Join(relativeParts, "/"), nil
}

// splitPolicyPath splits a normalized slash path into clean path components.
func splitPolicyPath(value string) []string {
	trimmed := strings.Trim(path.Clean(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

// Pluralize returns the singular or plural form of a noun based on count.
func Pluralize(noun string, count int) string {
	if count == 1 {
		return noun
	}
	return noun + "s"
}

// PluralizeVerb returns the singular or plural form of a verb based on count.
func PluralizeVerb(singular string, count int) string {
	if count == 1 {
		return singular
	}

	if plural, ok := irregularPluralVerbs[singular]; ok {
		return plural
	}

	if strings.HasSuffix(singular, "es") {
		return strings.TrimSuffix(singular, "es")
	}

	if strings.HasSuffix(singular, "s") {
		return strings.TrimSuffix(singular, "s")
	}

	return singular
}

// IsGeneratedFile checks if a file contains standard "Code generated" headers.
//
// It only checks the first 512 bytes for efficiency.
func IsGeneratedFile(content []byte) bool {
	header := string(content)
	if len(header) > 512 {
		header = header[:512]
	}
	return strings.Contains(header, "Code generated") && strings.Contains(header, "DO NOT EDIT")
}

// SelectorBaseName extracts the base name from an AST expression (Ident or Selector).
func SelectorBaseName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	default:
		return ""
	}
}
