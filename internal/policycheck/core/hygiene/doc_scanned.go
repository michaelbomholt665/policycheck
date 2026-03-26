// internal/policycheck/core/hygiene/doc_scanned.go
// Package hygiene provides documentation checks for non-Go scanned symbols.
// It validates Python (Google/Numpy/reST) and TypeScript (TSDoc) docstrings
// using facts produced by the scanner providers.
package hygiene

import (
	"fmt"
	"strings"

	"policycheck/internal/policycheck/types"
)

// validateScannedFunctionDocumentation validates the docstring of a scanned symbol.
func validateScannedFunctionDocumentation(
	fact types.PolicyFact,
	docCtx docContext,
) []types.Violation {
	if strings.TrimSpace(fact.Docstring) == "" {
		return []types.Violation{newDocumentationViolation(
			fact.LineNumber,
			documentationViolationSpec{
				subject:      fmt.Sprintf("%s %q is missing documentation", fact.SymbolKind, fact.SymbolName),
				expectation:  fmt.Sprintf("expected attached documentation immediately above the %s", fact.SymbolKind),
				functionName: fact.SymbolName,
			},
			docCtx,
		)}
	}

	if docCtx.cfg.Documentation.Level == "loose" {
		return nil
	}

	switch docCtx.lang.name {
	case "python":
		return validatePythonStrictDocumentation(fact, docCtx)
	case "typescript":
		return validateTypeScriptStrictDocumentation(fact, docCtx)
	default:
		return nil
	}
}

// validatePythonStrictDocumentation delegates to specific Python docstring style validators.
func validatePythonStrictDocumentation(
	fact types.PolicyFact,
	docCtx docContext,
) []types.Violation {
	style := docCtx.cfg.Documentation.PythonStyle
	if style == "presence_only" {
		return nil
	}

	summary := firstNonEmptyLine(fact.Docstring)
	if style == "standard" {
		return validateStrictSummaryFloor(fact, summary, docCtx)
	}

	if viols := validateStrictSummaryFloor(fact, summary, docCtx); len(viols) > 0 {
		return viols
	}

	switch style {
	case "google":
		return validatePythonGoogleDoc(fact, docCtx)
	case "numpy":
		return validatePythonNumpyDoc(fact, docCtx)
	case "restructuredtext":
		return validatePythonRESTDoc(fact, docCtx)
	default:
		return nil
	}
}

// validateTypeScriptStrictDocumentation validates TSDoc blocks in TypeScript files.
func validateTypeScriptStrictDocumentation(
	fact types.PolicyFact,
	docCtx docContext,
) []types.Violation {
	style := docCtx.cfg.Documentation.TypeScriptStyle
	if style == "presence_only" {
		return nil
	}

	summary := firstTypeScriptSummary(fact.Docstring)
	if style == "standard" {
		return validateStrictSummaryFloor(fact, summary, docCtx)
	}

	if !strings.HasPrefix(strings.TrimSpace(fact.Docstring), "/**") {
		return []types.Violation{newStyleViolation(
			fact.LineNumber,
			fact,
			"expected a /** ... */ documentation block immediately above the function",
			docCtx,
		)}
	}

	if viols := validateStrictSummaryFloor(fact, summary, docCtx); len(viols) > 0 {
		return viols
	}

	return validateTypeScriptTagSections(fact, docCtx)
}

// validatePythonGoogleDoc validates presence of Args: and Returns: in Google style.
func validatePythonGoogleDoc(
	fact types.PolicyFact,
	docCtx docContext,
) []types.Violation {
	var viols []types.Violation
	if len(fact.Params) > 0 && !googleArgsRegex.MatchString(fact.Docstring) {
		viols = append(viols, newStyleViolation(fact.LineNumber, fact, `missing required "Args:" section`, docCtx))
	}

	if !googleReturnsRegex.MatchString(fact.Docstring) {
		viols = append(viols, newStyleViolation(fact.LineNumber, fact, `missing required "Returns:" section`, docCtx))
	}

	return viols
}

// validatePythonNumpyDoc validates presence of Parameters and Returns in Numpy style.
func validatePythonNumpyDoc(
	fact types.PolicyFact,
	docCtx docContext,
) []types.Violation {
	var viols []types.Violation
	if len(fact.Params) > 0 && !numpyParametersRegex.MatchString(fact.Docstring) {
		viols = append(viols, newStyleViolation(fact.LineNumber, fact, `missing required "Parameters" section`, docCtx))
	}

	if !numpyReturnsRegex.MatchString(fact.Docstring) {
		viols = append(viols, newStyleViolation(fact.LineNumber, fact, `missing required "Returns" section`, docCtx))
	}

	return viols
}

// validatePythonRESTDoc validates presence of :param: and :returns: in reST style.
func validatePythonRESTDoc(
	fact types.PolicyFact,
	docCtx docContext,
) []types.Violation {
	var viols []types.Violation
	for _, param := range fact.Params {
		if !strings.Contains(fact.Docstring, ":param "+param+":") {
			viols = append(viols, newStyleViolation(fact.LineNumber, fact, fmt.Sprintf("missing :param field for argument %q", param), docCtx))
		}
	}

	if !strings.Contains(fact.Docstring, ":returns:") && !strings.Contains(fact.Docstring, ":return:") {
		viols = append(viols, newStyleViolation(fact.LineNumber, fact, "missing :returns: field", docCtx))
	}

	return viols
}

// firstTypeScriptSummary extracts the first meaningful summary line from a TSDoc block.
func firstTypeScriptSummary(docstring string) string {
	for _, line := range strings.Split(docstring, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "/**")
		trimmed = strings.TrimPrefix(trimmed, "*/")
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "*"))
		if trimmed == "" || strings.HasPrefix(trimmed, "@") {
			continue
		}
		return trimmed
	}

	return ""
}

// validateStrictSummaryFloor checks that a summary meets minimum length and quality rules.
func validateStrictSummaryFloor(
	fact types.PolicyFact,
	summary string,
	docCtx docContext,
) []types.Violation {
	floorReason := validateSummaryQuality(summary, fact.SymbolName, false)
	if floorReason == "" {
		return nil
	}

	return []types.Violation{newStyleViolation(
		fact.LineNumber,
		fact,
		floorReason,
		docCtx,
	)}
}

// validateTypeScriptTagSections ensures required TSDoc tags like @param are present.
func validateTypeScriptTagSections(
	fact types.PolicyFact,
	docCtx docContext,
) []types.Violation {
	var viols []types.Violation
	for _, param := range fact.Params {
		if strings.Contains(fact.Docstring, "@param "+param) {
			continue
		}

		viols = append(viols, newStyleViolation(
			fact.LineNumber,
			fact,
			fmt.Sprintf("missing @param tag for argument %q", param),
			docCtx,
		))
	}

	if strings.Contains(fact.Docstring, "@returns") || strings.Contains(fact.Docstring, "@return") {
		return viols
	}

	return append(viols, newStyleViolation(
		fact.LineNumber,
		fact,
		"missing required @returns tag",
		docCtx,
	))
}
