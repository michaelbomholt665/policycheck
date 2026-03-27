// internal/router/tools/wrlk/module.go
// Implements module path discovery and automated import rewriting
// for synchronizing bundled router copies with the host project.

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const bundledRouterModulePath = "policycheck"

// RouterRunModuleCommand handles module-related maintenance commands.
func RouterRunModuleCommand(options globalOptions, args []string, stdout io.Writer) error {
	if len(args) == 0 || RouterIsHelpToken(args[0]) {
		return RouterWriteModuleUsage(stdout)
	}

	switch args[0] {
	case "sync":
		return RouterRunModuleSyncCommand(options, args[1:], stdout)
	default:
		return &usageError{message: fmt.Sprintf("unknown module command %q", args[0])}
	}
}

// RouterWriteModuleUsage prints the module command usage.
func RouterWriteModuleUsage(writer io.Writer) error {
	lines := []string{
		"usage: Router [--root PATH] module sync",
		"commands:",
		"  sync      rewrite bundled router imports from the source module to the current go.mod module",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write module usage line: %w", err)
		}
	}

	return nil
}

// RouterRunModuleSyncCommand rewrites bundled router imports to the current module path.
func RouterRunModuleSyncCommand(options globalOptions, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("wrlk module sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return RouterWriteModuleUsage(stdout)
		}

		return &usageError{message: fmt.Sprintf("parse module sync flags: %v", err)}
	}
	if len(fs.Args()) > 0 {
		return &usageError{message: fmt.Sprintf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))}
	}

	modulePath, err := RouterReadModulePath(options.root)
	if err != nil {
		return fmt.Errorf("module sync: read module path: %w", err)
	}

	updatedFiles, err := RouterRewriteBundledModulePath(options.root, bundledRouterModulePath, modulePath)
	if err != nil {
		return fmt.Errorf("module sync: %w", err)
	}

	if _, err := fmt.Fprintf(
		stdout,
		"module sync: rewrote %d file(s) from %q to %q\n",
		len(updatedFiles),
		bundledRouterModulePath,
		modulePath,
	); err != nil {
		return fmt.Errorf("write module sync result: %w", err)
	}

	return nil
}

// RouterRewriteBundledModulePath rewrites copied router bundle imports under internal/router.
func RouterRewriteBundledModulePath(root string, sourceModulePath string, targetModulePath string) ([]string, error) {
	if targetModulePath == "" {
		return nil, fmt.Errorf("target module path is empty")
	}
	if sourceModulePath == targetModulePath {
		return nil, nil
	}

	routerRoot := filepath.Join(root, filepath.FromSlash("internal/router"))
	updatedFiles := make([]string, 0)

	walkErr := filepath.WalkDir(routerRoot, func(path string, entry os.DirEntry, err error) error {
		updatedFile, updateErr := RouterRewriteBundledModuleFile(
			root,
			path,
			entry,
			err,
			sourceModulePath,
			targetModulePath,
		)
		if updateErr != nil {
			return updateErr
		}
		if updatedFile != "" {
			updatedFiles = append(updatedFiles, updatedFile)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("rewrite bundled module path: %w", walkErr)
	}

	return updatedFiles, nil
}

// RouterRewriteBundledModuleFile rewrites one bundled router Go file when it still imports the source module.
func RouterRewriteBundledModuleFile(
	root string,
	path string,
	entry os.DirEntry,
	walkErr error,
	sourceModulePath string,
	targetModulePath string,
) (string, error) {
	if walkErr != nil {
		return "", walkErr
	}
	if entry.IsDir() || filepath.Ext(path) != ".go" {
		return "", nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	updatedContent := strings.ReplaceAll(string(content), sourceModulePath, targetModulePath)
	if updatedContent == string(content) {
		return "", nil
	}

	if writeErr := os.WriteFile(path, []byte(updatedContent), 0o644); writeErr != nil {
		return "", fmt.Errorf("write %s: %w", path, writeErr)
	}

	relativePath, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("compute relative path for %s: %w", path, err)
	}

	return filepath.ToSlash(relativePath), nil
}
