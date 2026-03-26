// internal/adapters/walk/extension.go
package walk

import (
	"io/fs"
	"path/filepath"
	"strings"

	"policycheck/internal/router"
)

// Extension implements router.Extension for the walk adapter.
type Extension struct{}

// Required returns true - walk capability is mandatory.
func (e *Extension) Required() bool { return true }

// Consumes returns nil - no boot-time dependencies.
func (e *Extension) Consumes() []router.PortName { return nil }

// Provides returns the ports this extension registers.
func (e *Extension) Provides() []router.PortName { return []router.PortName{router.PortWalk} }

// RouterProvideRegistration registers the walk provider.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	return reg.RouterRegisterProvider(router.PortWalk, &Adapter{})
}

// ExtensionInstance returns the extension instance.
func ExtensionInstance() router.Extension {
	return &Extension{}
}

// Adapter implements the ports.WalkProvider interface.
type Adapter struct{}

// WalkDirectoryTree implements a standard file system traversal,
// skipping internal/router, tests, backups, scripts, and various cache/metadata directories.
func (a *Adapter) WalkDirectoryTree(root string, walkFn fs.WalkDirFunc) error {
	skipDirs := map[string]bool{
		"router":       false, // handled specially with path check
		"test":         true,
		"tests":        true,
		".backup":      true,
		".agents":      true,
		"script":       true,
		"scripts":      true,
		".claude":      true,
		".gemini":      true,
		".codex":       true,
		".kilocode":    true,
		".antigravity": true,
		".mypy_cache":  true,
		".qodo":        true,
		"__pycache__":  true,
		".ruff_cache":  true,
		".git":         true,
		"node_modules": true,
		".vscode":      true,
		".idea":        true,
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return walkFn(path, d, err)
		}
		if d.IsDir() {
			name := d.Name()
			// Special case for internal/router
			if name == "router" && strings.HasSuffix(filepath.ToSlash(path), "internal/router") {
				return fs.SkipDir
			}
			if skipDirs[name] && filepath.Clean(path) != filepath.Clean(root) {
				return fs.SkipDir
			}
		}
		if !d.IsDir() && strings.HasSuffix(path, ".toml") {
			return nil
		}
		return walkFn(path, d, err)
	})
}
