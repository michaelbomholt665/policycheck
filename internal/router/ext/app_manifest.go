// internal/router/ext/app_manifest.go
// Contains the statically generated ordered declarations for required
// application extensions, serving as the ground-truth for application boot.

package ext

// ApplicationExtensionManifestEntry declares one generated required application extension.
type ApplicationExtensionManifestEntry struct {
	Name string
}

// DeclaredApplicationExtensions defines the ordered required application extension declarations.
var DeclaredApplicationExtensions = []ApplicationExtensionManifestEntry{{Name: "config"}, {Name: "walk"}, {Name: "scanners"}, {Name: "cliwrappercore"}, {Name: "cliwrapper"}, {Name: "readfile"}}
