// internal/policycheck/utils/utils.go
// Package utils provides common helper functions used across the policycheck codebase.
// These utilities handle path normalization, pluralization, and AST metadata extraction.
package utils

import (
	"go/ast"
	"path"
	"path/filepath"
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
	cleaned := path.Clean(filepath.ToSlash(value))
	if cleaned == "." {
		return ""
	}

	return strings.TrimPrefix(cleaned, "./")
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
	if !filepath.IsAbs(target) {
		return NormalizePolicyPath(target)
	}

	rel, err := filepath.Rel(root, target)
	if err != nil {
		return NormalizePolicyPath(target)
	}

	return NormalizePolicyPath(rel)
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
