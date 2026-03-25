// internal/policycheck/core/version.go
// Validates language versions (Go, Python, TypeScript) against configured allowed prefixes.

package core

const ScopeProjectRepo = true


import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
)

// CheckGoVersion validates that the Go version in go.mod matches an allowed prefix from the configuration.
func CheckGoVersion(root string, cfg config.PolicyVersionConfig) []types.Violation {
	content, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return []types.Violation{{Path: "go.mod", Message: fmt.Sprintf("unable to read go.mod: %v", err)}}
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "go ") {
			version := strings.TrimSpace(strings.TrimPrefix(line, "go"))
			if isVersionAllowed(version, cfg.AllowedPrefixes) {
				return nil
			}
			return []types.Violation{{Path: "go.mod", Message: fmt.Sprintf("go version must match one of [%s] (got %s)", strings.Join(cfg.AllowedPrefixes, ", "), version)}}
		}
	}
	return []types.Violation{{Path: "go.mod", Message: "missing go version declaration"}}
}

// CheckPythonVersion validates the Python version in pyproject.toml or .python-version against allowed prefixes.
func CheckPythonVersion(root string, cfg config.PolicyVersionConfig) []types.Violation {
	// Try pyproject.toml
	if violation, found := checkPyprojectPythonVersion(root, cfg); found {
		if violation != nil {
			return []types.Violation{*violation}
		}
		return nil // Found and valid
	}

	// Try .python-version
	if violation, found := checkDotPythonVersion(root, cfg); found {
		if violation != nil {
			return []types.Violation{*violation}
		}
		return nil // Found and valid
	}

	// If no version file/declaration found and "" is not allowed, it's a violation
	if !isVersionAllowed("", cfg.AllowedPrefixes) {
		return []types.Violation{{Path: ".", Message: "missing python version declaration (pyproject.toml or .python-version)"}}
	}

	return nil
}

// checkPyprojectPythonVersion orchestrates the validation of Python version from pyproject.toml.
func checkPyprojectPythonVersion(root string, cfg config.PolicyVersionConfig) (*types.Violation, bool) {
	content, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err != nil {
		return nil, false
	}

	var data map[string]any
	if err := toml.Unmarshal(content, &data); err != nil {
		return nil, false
	}

	// 1. Try [project] requires-python (PEP 621)
	if violation, found := checkPEP621Version(data, cfg); found {
		return violation, true
	}

	// 2. Try [tool.poetry.dependencies] python (Legacy/Poetry)
	if violation, found := checkPoetryVersion(data, cfg); found {
		return violation, true
	}

	return nil, false
}

// checkPEP621Version extracts and validates requires-python from the [project] table.
func checkPEP621Version(data map[string]any, cfg config.PolicyVersionConfig) (*types.Violation, bool) {
	project, ok := data["project"].(map[string]any)
	if !ok {
		return nil, false
	}
	req, ok := project["requires-python"].(string)
	if !ok {
		return nil, false
	}
	version := strings.TrimLeft(req, ">=<! ")
	if isVersionAllowed(version, cfg.AllowedPrefixes) {
		return nil, true
	}
	return &types.Violation{Path: "pyproject.toml", Message: fmt.Sprintf("python version must match one of [%s] (got %s)", strings.Join(cfg.AllowedPrefixes, ", "), req)}, true
}

// checkPoetryVersion extracts and validates python version from [tool.poetry.dependencies].
func checkPoetryVersion(data map[string]any, cfg config.PolicyVersionConfig) (*types.Violation, bool) {
	tool, ok := data["tool"].(map[string]any)
	if !ok {
		return nil, false
	}
	poetry, ok := tool["poetry"].(map[string]any)
	if !ok {
		return nil, false
	}
	deps, ok := poetry["dependencies"].(map[string]any)
	if !ok {
		return nil, false
	}
	req, ok := deps["python"].(string)
	if !ok {
		return nil, false
	}
	version := strings.TrimLeft(req, ">=<!^~ ")
	if isVersionAllowed(version, cfg.AllowedPrefixes) {
		return nil, true
	}
	return &types.Violation{Path: "pyproject.toml", Message: fmt.Sprintf("python version must match one of [%s] (got %s)", strings.Join(cfg.AllowedPrefixes, ", "), req)}, true
}

// checkDotPythonVersion validates the Python version from a .python-version file.
func checkDotPythonVersion(root string, cfg config.PolicyVersionConfig) (*types.Violation, bool) {
	content, err := os.ReadFile(filepath.Join(root, ".python-version"))
	if err != nil {
		return nil, false
	}
	version := strings.TrimSpace(string(content))
	if version == "" {
		return nil, false
	}
	if isVersionAllowed(version, cfg.AllowedPrefixes) {
		return nil, true
	}
	return &types.Violation{Path: ".python-version", Message: fmt.Sprintf("python version must match one of [%s] (got %s)", strings.Join(cfg.AllowedPrefixes, ", "), version)}, true
}

// CheckTypescriptVersion validates the engines.node or devDependencies.typescript in package.json against allowed prefixes.
func CheckTypescriptVersion(root string, cfg config.PolicyVersionConfig) []types.Violation {
	content, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		if !isVersionAllowed("", cfg.AllowedPrefixes) {
			return []types.Violation{{Path: "package.json", Message: "missing package.json for typescript version check"}}
		}
		return nil
	}

	var pkg struct {
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
		DevDependencies struct {
			Typescript string `json:"typescript"`
		} `json:"devDependencies"`
	}

	if err := json.Unmarshal(content, &pkg); err != nil {
		return []types.Violation{{Path: "package.json", Message: fmt.Sprintf("unable to parse package.json: %v", err)}}
	}

	// Check typescript version first
	if pkg.DevDependencies.Typescript != "" {
		version := strings.TrimLeft(pkg.DevDependencies.Typescript, "^~>=<! ")
		if isVersionAllowed(version, cfg.AllowedPrefixes) {
			return nil
		}
		return []types.Violation{{Path: "package.json", Message: fmt.Sprintf("typescript version must match one of [%s] (got %s)", strings.Join(cfg.AllowedPrefixes, ", "), pkg.DevDependencies.Typescript)}}
	}

	// Fallback to node engine if typescript not in devDeps
	if pkg.Engines.Node != "" {
		version := strings.TrimLeft(pkg.Engines.Node, "^~>=<! ")
		if isVersionAllowed(version, cfg.AllowedPrefixes) {
			return nil
		}
		return []types.Violation{{Path: "package.json", Message: fmt.Sprintf("node version must match one of [%s] (got %s)", strings.Join(cfg.AllowedPrefixes, ", "), pkg.Engines.Node)}}
	}

	if !isVersionAllowed("", cfg.AllowedPrefixes) {
		return []types.Violation{{Path: "package.json", Message: "missing typescript or node version declaration in package.json"}}
	}

	return nil
}

// isVersionAllowed checks if a version string starts with any of the allowed prefixes.
// isVersionAllowed checks if a version string starts with any of the allowed prefixes.
func isVersionAllowed(version string, allowedPrefixes []string) bool {
	for _, prefix := range allowedPrefixes {
		if prefix == "" {
			return true
		}
		if strings.HasPrefix(version, prefix) {
			return true
		}
	}
	return false
}
