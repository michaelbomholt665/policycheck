// internal/tests/policycheck/core/structure/router_imports_test.go
package structure_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/app"
	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core/structure"
	"policycheck/internal/router"
)

func TestCheckRouterImports(t *testing.T) {
	router.RouterResetForTest()
	require.NoError(t, app.BootPolicycheckApp(context.Background()))

	tmp := t.TempDir()

	// Business code importing adapter (ILLEGAL)
	bizPath := filepath.Join(tmp, "internal/policycheck/logic.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(bizPath), 0o755))
	require.NoError(t, os.WriteFile(bizPath, []byte(`
package policycheck
import "policycheck/internal/adapters/scanners"
func Run() {}
`), 0o644))

	// Adapter importing another adapter (ILLEGAL)
	adapterPath := filepath.Join(tmp, "internal/adapters/config/adapter.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(adapterPath), 0o755))
	require.NoError(t, os.WriteFile(adapterPath, []byte(`
package config
import "policycheck/internal/adapters/scanners"
func Init() {}
`), 0o644))

	// Adapter importing business package (ILLEGAL)
	adapterBusinessPath := filepath.Join(tmp, "internal/adapters/corebridge/provider.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(adapterBusinessPath), 0o755))
	require.NoError(t, os.WriteFile(adapterBusinessPath, []byte(`
package corebridge
import "policycheck/internal/cliwrapper"
func Provide() cliwrapper.InstallRequest { return cliwrapper.InstallRequest{} }
`), 0o644))

	// Router core importing adapter (ILLEGAL)
	routerCorePath := filepath.Join(tmp, "internal/router/registry.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(routerCorePath), 0o755))
	require.NoError(t, os.WriteFile(routerCorePath, []byte(`
package router
import "policycheck/internal/adapters/config"
func Resolve() {}
`), 0o644))

	// Router core importing business logic (ILLEGAL)
	routerCorePath2 := filepath.Join(tmp, "internal/router/extension.go")
	require.NoError(t, os.WriteFile(routerCorePath2, []byte(`
package router
import "policycheck/internal/policycheck"
func Boot() {}
`), 0o644))

	// App (boot seam) importing router/ext (ALLOWED)
	appPath := filepath.Join(tmp, "internal/app/bootstrap.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(appPath), 0o755))
	require.NoError(t, os.WriteFile(appPath, []byte(`
package app
import "policycheck/internal/router/ext"
func Main() {}
`), 0o644))

	// Business code importing router/ext exact package (ILLEGAL)
	bizPath3 := filepath.Join(tmp, "internal/policycheck/ext_exact.go")
	require.NoError(t, os.WriteFile(bizPath3, []byte(`
package policycheck
import "policycheck/internal/router/ext"
func BadExtImport() {}
`), 0o644))

	// Business code importing a non-allowed internal package (ILLEGAL)
	bizPath4 := filepath.Join(tmp, "internal/policycheck/router_tool.go")
	require.NoError(t, os.WriteFile(bizPath4, []byte(`
package policycheck
import "policycheck/internal/router/tools/wrlk"
func BadToolImport() {}
`), 0o644))

	// Business code importing allowed boundaries (ALLOWED)
	bizPath2 := filepath.Join(tmp, "internal/policycheck/ok.go")
	require.NoError(t, os.WriteFile(bizPath2, []byte(`
package policycheck
import (
    "policycheck/internal/ports"
    "policycheck/internal/router"
    "policycheck/internal/router/capabilities"
)
func OK() {}
`), 0o644))

	cfg := config.PolicyConfig{
		RouterImports: config.PolicyRouterImportsConfig{
			Enabled:         true,
			BusinessRoots:   []string{"internal/policycheck", "internal/cliwrapper", "internal/ports"},
			AdapterRoots:    []string{"internal/adapters"},
			RouterCoreRoots: []string{"internal/router"},
			RouterBootRoots: []string{"internal/app", "internal/router/ext"},
			AllowedBusinessImports: []string{
				"policycheck/internal/ports",
				"policycheck/internal/router",
				"policycheck/internal/router/capabilities",
			},
			ForbiddenBusinessImportPrefixes: []string{
				"policycheck/internal/adapters/",
				"policycheck/internal/router/ext/",
			},
			ForbiddenAdapterToAdapter: true,
		},
	}

	violations := structure.CheckRouterImports(context.Background(), tmp, cfg)

	// Expected violations:
	// 1. internal/policycheck/logic.go -> policycheck/internal/adapters/scanners
	// 2. internal/adapters/config/adapter.go -> policycheck/internal/adapters/scanners
	// 3. internal/router/registry.go -> policycheck/internal/adapters/config
	// 4. internal/router/extension.go -> policycheck/internal/policycheck
	// 5. internal/policycheck/ext_exact.go -> policycheck/internal/router/ext
	// 6. internal/policycheck/router_tool.go -> policycheck/internal/router/tools/wrlk
	// 7. internal/adapters/corebridge/provider.go -> policycheck/internal/cliwrapper

	assert.Len(t, violations, 7)

	var bizViol, adapterViol, adapterBusinessViol, routerCoreAdapterViol, routerCoreBizViol, bizExtExactViol, bizDisallowedInternalViol bool
	for _, v := range violations {
		switch {
		case strings.Contains(v.File, "logic.go") && strings.Contains(v.Message, "adapters"):
			bizViol = true
		case strings.Contains(v.File, "adapter.go") && strings.Contains(v.Message, "adapter"):
			adapterViol = true
		case strings.Contains(v.File, "provider.go") && strings.Contains(v.Message, "business package"):
			adapterBusinessViol = true
		case strings.Contains(v.File, "registry.go") && strings.Contains(v.Message, "adapters"):
			routerCoreAdapterViol = true
		case strings.Contains(v.File, "extension.go") && strings.Contains(v.Message, "business"):
			routerCoreBizViol = true
		case strings.Contains(v.File, "ext_exact.go") && strings.Contains(v.Message, "forbidden path"):
			bizExtExactViol = true
		case strings.Contains(v.File, "router_tool.go") && strings.Contains(v.Message, "outside allowed router boundaries"):
			bizDisallowedInternalViol = true
		}
	}

	assert.True(t, bizViol, "Should have business -> adapter violation")
	assert.True(t, adapterViol, "Should have adapter -> adapter violation")
	assert.True(t, adapterBusinessViol, "Should have adapter -> business violation")
	assert.True(t, routerCoreAdapterViol, "Should have router core -> adapter violation")
	assert.True(t, routerCoreBizViol, "Should have router core -> business violation")
	assert.True(t, bizExtExactViol, "Should have business -> router/ext exact package violation")
	assert.True(t, bizDisallowedInternalViol, "Should have business -> non-allowed internal package violation")
}

func stringsContains(s []string, sub string) bool {
	for _, str := range s {
		if strings.Contains(str, sub) {
			return true
		}
	}
	return false
}
