// internal/app/config_command.go
// Implements the shared wrapper config inspection and scaffold commands.
// Keeps config rendering and file mutation out of the wrapper execution adapters.
package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"policycheck/internal/cliwrapper"
)

// RunConfigCommand handles the shared wrapper config inspection and init commands.
func RunConfigCommand(args []string) error {
	if len(args) > 0 && args[0] == "init" {
		return runConfigInit(args[1:])
	}

	return runConfigInspect(args)
}

// runConfigInspect renders the effective wrapper config for the current scope.
func runConfigInspect(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	globalOnly := fs.Bool("global", false, "inspect the global wrapper config")
	configPath := fs.String("config", cliwrapper.RepoConfigFilename, "repo config path")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("config inspect: parse flags: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("config inspect: get working directory: %w", err)
	}

	globalPath, err := cliwrapper.DefaultGlobalConfigPath()
	if err != nil {
		return fmt.Errorf("config inspect: resolve global config path: %w", err)
	}

	if *globalOnly {
		cfg, path, loadErr := loadGlobalConfig(globalPath)
		if loadErr != nil {
			return fmt.Errorf("config inspect: load global config: %w", loadErr)
		}

		return printConfig("global", cfg, path, "")
	}

	startDir, err := resolveRepoSearchDir(cwd, *configPath)
	if err != nil {
		return fmt.Errorf("config inspect: resolve repo config path: %w", err)
	}

	loader := cliwrapper.WrapperConfigLoader{
		GlobalConfigPath: globalPath,
		StartDir:         startDir,
	}
	result, err := loader.Load()
	if err != nil {
		return fmt.Errorf("config inspect: load merged config: %w", err)
	}

	return printConfig("merged", result.Merged, result.GlobalPath, result.RepoPath)
}

// runConfigInit renders or writes a wrapper config scaffold.
func runConfigInit(args []string) error {
	fs := flag.NewFlagSet("config init", flag.ContinueOnError)
	globalOnly := fs.Bool("global", false, "write the global wrapper config")
	dryRun := fs.Bool("dry-run", false, "print the scaffold without writing it")
	configPath := fs.String("config", cliwrapper.RepoConfigFilename, "repo config path")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("config init: parse flags: %w", err)
	}

	targetPath, err := resolveInitTargetPath(*globalOnly, *configPath)
	if err != nil {
		return fmt.Errorf("config init: resolve target path: %w", err)
	}

	content, err := cliwrapper.MarshalWrapperConfig(cliwrapper.DefaultWrapperConfig())
	if err != nil {
		return fmt.Errorf("config init: render scaffold: %w", err)
	}

	if *dryRun {
		_, err = fmt.Fprintf(os.Stdout, "target_path = %s\n%s", targetPath, content)
		if err != nil {
			return fmt.Errorf("config init: write dry-run output: %w", err)
		}

		return nil
	}

	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("config init: target already exists: %s", targetPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("config init: stat target %s: %w", targetPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("config init: create parent directory: %w", err)
	}

	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		return fmt.Errorf("config init: write scaffold: %w", err)
	}

	return nil
}

// loadGlobalConfig loads the global-only wrapper config view.
func loadGlobalConfig(globalPath string) (cliwrapper.WrapperConfig, string, error) {
	loader := cliwrapper.WrapperConfigLoader{
		GlobalConfigPath: globalPath,
		StartDir:         os.TempDir(),
	}
	result, err := loader.Load()
	if err != nil {
		return cliwrapper.WrapperConfig{}, "", err
	}

	return result.Merged, result.GlobalPath, nil
}

// printConfig writes the selected config view to stdout.
func printConfig(scope string, cfg cliwrapper.WrapperConfig, globalPath string, repoPath string) error {
	content, err := cliwrapper.MarshalWrapperConfig(cfg)
	if err != nil {
		return fmt.Errorf("print config: render: %w", err)
	}

	if _, err := fmt.Fprintf(os.Stdout, "scope = %q\n", scope); err != nil {
		return fmt.Errorf("print config: write scope: %w", err)
	}
	if _, err := fmt.Fprintf(os.Stdout, "global_path = %s\n", globalPath); err != nil {
		return fmt.Errorf("print config: write global path: %w", err)
	}
	if _, err := fmt.Fprintf(os.Stdout, "repo_path = %s\n", repoPath); err != nil {
		return fmt.Errorf("print config: write repo path: %w", err)
	}
	if _, err := fmt.Fprint(os.Stdout, string(content)); err != nil {
		return fmt.Errorf("print config: write body: %w", err)
	}

	return nil
}

// resolveRepoSearchDir resolves the repo-search root for config inspection.
func resolveRepoSearchDir(cwd string, configPath string) (string, error) {
	if strings.TrimSpace(configPath) == "" || configPath == cliwrapper.RepoConfigFilename {
		return cwd, nil
	}

	absolutePath, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("resolve absolute config path: %w", err)
	}
	if filepath.Base(absolutePath) != cliwrapper.RepoConfigFilename {
		return "", fmt.Errorf("config path must end with %s", cliwrapper.RepoConfigFilename)
	}

	return filepath.Dir(absolutePath), nil
}

// resolveInitTargetPath resolves the file path used by config init.
func resolveInitTargetPath(globalOnly bool, configPath string) (string, error) {
	if globalOnly {
		globalPath, err := cliwrapper.DefaultGlobalConfigPath()
		if err != nil {
			return "", err
		}

		return globalPath, nil
	}

	if strings.TrimSpace(configPath) == "" {
		configPath = cliwrapper.RepoConfigFilename
	}

	if filepath.IsAbs(configPath) {
		return configPath, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	return filepath.Join(cwd, configPath), nil
}
