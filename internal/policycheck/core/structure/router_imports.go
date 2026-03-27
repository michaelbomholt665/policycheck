// internal/policycheck/core/structure/router_imports.go
// Package structure provides structural policy checks for the repository.
// It includes rules for router imports, package boundaries, and file locations.
package structure

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

// CheckRouterImports enforces the router import architecture.
//
// It detects illegal imports across business, adapter, and router core packages.
func CheckRouterImports(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	if !cfg.RouterImports.Enabled {
		return nil
	}

	var viols []types.Violation

	// Collect all roots to scan
	var allRoots []string
	allRoots = append(allRoots, cfg.RouterImports.BusinessRoots...)
	allRoots = append(allRoots, cfg.RouterImports.AdapterRoots...)
	allRoots = append(allRoots, cfg.RouterImports.RouterCoreRoots...)
	allRoots = append(allRoots, cfg.RouterImports.RouterBootRoots...)

	// Unique roots
	uniqueRoots := make(map[string]struct{})
	for _, r := range allRoots {
		uniqueRoots[r] = struct{}{}
	}

	for r := range uniqueRoots {
		absRoot := filepath.Join(root, r)
		_ = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}

			rel := utils.ToSlashRel(root, path)
			if isException(rel, cfg.RouterImports.Exceptions) {
				return nil
			}

			fileViols := checkFileRouterImports(root, path, cfg)
			viols = append(viols, fileViols...)
			return nil
		})
	}

	return viols
}

// isException returns true if the relative path matches any of the given exceptions.
func isException(rel string, exceptions []string) bool {
	for _, ex := range exceptions {
		if strings.HasPrefix(rel, ex) {
			return true
		}
	}
	return false
}

// checkFileRouterImports scans a single Go file for router import violations.
func checkFileRouterImports(root, path string, cfg config.PolicyConfig) []types.Violation {
	content, err := host.ReadFile(path)
	if err != nil {
		return nil
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, parser.ImportsOnly)
	if err != nil {
		return nil
	}

	rel := utils.ToSlashRel(root, path)
	var viols []types.Violation

	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		line := fset.Position(imp.Pos()).Line

		if v := validateImport(rel, importPath, line, cfg); v != nil {
			viols = append(viols, *v)
		}
	}

	return viols
}

// validateImport checks if a specific import path is allowed for the source file.
func validateImport(rel, importPath string, line int, cfg config.PolicyConfig) *types.Violation {
	// Classify the source file
	isRouterBoot := isUnder(rel, cfg.RouterImports.RouterBootRoots)
	isBusiness := isUnder(rel, cfg.RouterImports.BusinessRoots)
	isAdapter := isUnder(rel, cfg.RouterImports.AdapterRoots)
	isRouterCore := isUnder(rel, cfg.RouterImports.RouterCoreRoots) && !isRouterBoot

	if isBusiness && !isRouterBoot {
		if v := validateBusinessImport(rel, importPath, line, cfg); v != nil {
			return v
		}
	}

	if isAdapter && cfg.RouterImports.ForbiddenAdapterToAdapter {
		if v := validateAdapterImport(rel, importPath, line, cfg); v != nil {
			return v
		}
	}

	if isRouterCore {
		if v := validateRouterCoreImport(rel, importPath, line, cfg); v != nil {
			return v
		}
	}

	return nil
}

// validateBusinessImport ensures business packages do not import forbidden prefixes.
func validateBusinessImport(rel, importPath string, line int, cfg config.PolicyConfig) *types.Violation {
	for _, forbidden := range cfg.RouterImports.ForbiddenBusinessImportPrefixes {
		if matchesImportBoundary(importPath, normalizeImportPrefix(forbidden)) {
			return &types.Violation{
				RuleID:   "structure.router_imports",
				File:     rel,
				Line:     line,
				Message:  fmt.Sprintf("business package imports forbidden path %q; resolve through internal/ports + internal/router instead", importPath),
				Severity: "error",
			}
		}
	}

	if !isInternalModuleImport(importPath) {
		return nil
	}

	if isBusinessImport(importPath, cfg.RouterImports.BusinessRoots) {
		return nil
	}

	if isAllowedBusinessImport(importPath, cfg.RouterImports.AllowedBusinessImports) {
		return nil
	}

	return &types.Violation{
		RuleID:   "structure.router_imports",
		File:     rel,
		Line:     line,
		Message:  fmt.Sprintf("business package imports internal package %q outside allowed router boundaries; use internal/ports, internal/router, or configured business packages", importPath),
		Severity: "error",
	}
}

// isInternalModuleImport reports whether the import targets this repository's internal tree.
func isInternalModuleImport(importPath string) bool {
	return importPath == "policycheck/internal" || strings.HasPrefix(importPath, "policycheck/internal/")
}

// isAllowedBusinessImport reports whether the import is explicitly allowed for business packages.
func isAllowedBusinessImport(importPath string, allowedImports []string) bool {
	for _, allowed := range allowedImports {
		if importPath == allowed {
			return true
		}
	}

	return false
}

// validateAdapterImport prevents adapter-to-adapter imports if configured.
func validateAdapterImport(rel, importPath string, line int, cfg config.PolicyConfig) *types.Violation {
	if isBusinessImport(importPath, cfg.RouterImports.BusinessRoots) && !isAllowedAdapterBusinessImport(importPath) {
		return &types.Violation{
			RuleID:   "structure.router_imports",
			File:     rel,
			Line:     line,
			Message:  fmt.Sprintf("adapter package imports business package %q; adapters must depend on router ports/contracts instead", importPath),
			Severity: "error",
		}
	}

	if !isAdapterImport(importPath, cfg.RouterImports.AdapterRoots) {
		return nil
	}

	// Extract current package path
	pkgPath := filepath.Dir(rel)
	// Convert pkgPath to full module path for comparison
	fullPkgPath := "policycheck/" + pkgPath

	if importPath != fullPkgPath && !strings.HasPrefix(fullPkgPath, importPath+"/") && !strings.HasPrefix(importPath, fullPkgPath+"/") {
		return &types.Violation{
			RuleID:   "structure.router_imports",
			File:     rel,
			Line:     line,
			Message:  fmt.Sprintf("adapter package imports another adapter %q; adapters must communicate through router ports", importPath),
			Severity: "error",
		}
	}
	return nil
}

// validateRouterCoreImport ensures router core stays blind to adapters and business logic.
func validateRouterCoreImport(rel, importPath string, line int, cfg config.PolicyConfig) *types.Violation {
	if isAdapterImport(importPath, cfg.RouterImports.AdapterRoots) {
		return &types.Violation{
			RuleID:   "structure.router_imports",
			File:     rel,
			Line:     line,
			Message:  fmt.Sprintf("router core imports adapter package %q; router core must stay blind to adapters", importPath),
			Severity: "error",
		}
	}
	if isBusinessImport(importPath, cfg.RouterImports.BusinessRoots) {
		return &types.Violation{
			RuleID:   "structure.router_imports",
			File:     rel,
			Line:     line,
			Message:  fmt.Sprintf("router core imports business package %q; router core must stay blind to business logic", importPath),
			Severity: "error",
		}
	}
	return nil
}

// isUnder returns true if the relative path is under any of the given roots.
func isUnder(rel string, roots []string) bool {
	return utils.HasPrefix(rel, roots)
}

// isAdapterImport returns true if the import path refers to an adapter package.
func isAdapterImport(importPath string, adapterRoots []string) bool {
	for _, root := range adapterRoots {
		// Module prefix + root
		prefix := "policycheck/" + root
		if matchesImportBoundary(importPath, prefix) {
			return true
		}
	}
	return false
}

// isAllowedAdapterBusinessImport reports whether an adapter may import a business-side contract package.
func isAllowedAdapterBusinessImport(importPath string) bool {
	return importPath == "policycheck/internal/ports"
}

// isBusinessImport returns true if the import path refers to a business package.
func isBusinessImport(importPath string, businessRoots []string) bool {
	for _, root := range businessRoots {
		prefix := "policycheck/" + root
		if matchesImportBoundary(importPath, prefix) {
			return true
		}
	}
	return false
}

// matchesImportBoundary reports whether importPath matches prefix exactly or as a child package.
func matchesImportBoundary(importPath string, prefix string) bool {
	return importPath == prefix || strings.HasPrefix(importPath, prefix+"/")
}

// normalizeImportPrefix trims a trailing slash so config prefixes match exact packages too.
func normalizeImportPrefix(prefix string) string {
	return strings.TrimSuffix(prefix, "/")
}
