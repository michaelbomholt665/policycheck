// internal/policycheck/embedded/scanners.go
// Materializes embedded scanner byte slices to temporary files for subprocess execution.

package embedded

const ScopeProjectRepo = true


import (
	"fmt"
	"os"
	"path/filepath"

	"policycheck/internal/policycheck/types"
)

// MaterializeScannerAssets writes embedded scanner bytes to a temporary directory.
// It returns the asset paths, a cleanup function, and any error encountered.
func MaterializeScannerAssets(bytes types.ScannerBytes) (types.PolicyScannerAssets, func(), error) {
	rootDir, err := os.MkdirTemp("", "policycheck-scanners-*")
	if err != nil {
		return types.PolicyScannerAssets{}, noopCleanup, fmt.Errorf("create temp script dir: %w", err)
	}

	pythonName := "policy_scanner.py"
	tsName := "policy_scanner.cjs"

	if err := writeScannerAsset(rootDir, pythonName, bytes.Python); err != nil {
		_ = os.RemoveAll(rootDir)
		return types.PolicyScannerAssets{}, noopCleanup, fmt.Errorf("write python scanner asset: %w", err)
	}
	if err := writeScannerAsset(rootDir, tsName, bytes.JS); err != nil {
		_ = os.RemoveAll(rootDir)
		return types.PolicyScannerAssets{}, noopCleanup, fmt.Errorf("write typescript scanner asset: %w", err)
	}

	assets := types.PolicyScannerAssets{
		RootDir: rootDir,
		Python:  filepath.Join(rootDir, pythonName),
		TS:      filepath.Join(rootDir, tsName),
	}

	cleanup := func() {
		_ = os.RemoveAll(rootDir)
	}
	return assets, cleanup, nil
}

// writeScannerAsset writes a single script to the target directory with executable permissions.
func writeScannerAsset(rootDir, name string, content []byte) error {
	destPath := filepath.Join(rootDir, name)
	if err := os.WriteFile(destPath, content, 0o700); err != nil {
		return fmt.Errorf("write embedded script %s: %w", name, err)
	}
	return nil
}

// noopCleanup is a placeholder cleanup function for error paths where no temp dir was created.
func noopCleanup() {
	// Intentionally empty: error paths that return this cleanup function never created temp assets.
}
