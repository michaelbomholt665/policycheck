// internal/tests/policycheck/host/ports_test.go
package host_test

import (
	"context"
	"io/fs"
	"testing"

	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/router"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConfig struct{}

func (m *mockConfig) SetPath(path string)           {}
func (m *mockConfig) GetRawSource() ([]byte, error) { return nil, nil }

type mockWalk struct{}

func (m *mockWalk) WalkDirectoryTree(root string, walkFn fs.WalkDirFunc) error { return nil }

type mockScanner struct{}

func (m *mockScanner) RunScanners(ctx context.Context, root string) ([]types.PolicyFact, error) {
	return nil, nil
}

type mockExt struct {
	port router.PortName
	prov router.Provider
}

func (m *mockExt) Required() bool              { return true }
func (m *mockExt) Consumes() []router.PortName { return nil }
func (m *mockExt) Provides() []router.PortName { return []router.PortName{m.port} }
func (m *mockExt) RouterProvideRegistration(reg *router.Registry) error {
	return reg.RouterRegisterProvider(m.port, m.prov)
}

func TestResolveProviders(t *testing.T) {
	router.RouterResetForTest()
	defer router.RouterResetForTest()

	// Boot the router with our mock extensions
	exts := []router.Extension{
		&mockExt{port: router.PortConfig, prov: &mockConfig{}},
		&mockExt{port: router.PortWalk, prov: &mockWalk{}},
		&mockExt{port: router.PortScanner, prov: &mockScanner{}},
	}
	warnings, err := router.RouterLoadExtensions(nil, exts, context.Background())
	require.NoError(t, err)
	assert.Empty(t, warnings)

	// Test Config
	cfg, err := host.ResolveConfigProvider()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Test Walk
	walk, err := host.ResolveWalkProvider()
	require.NoError(t, err)
	assert.NotNil(t, walk)

	// Test Scanner
	scanner, err := host.ResolveScannerProvider()
	require.NoError(t, err)
	assert.NotNil(t, scanner)
}

func TestResolveProviders_NotBooted(t *testing.T) {
	router.RouterResetForTest()

	_, err := host.ResolveConfigProvider()
	require.Error(t, err)

	_, err = host.ResolveWalkProvider()
	require.Error(t, err)

	_, err = host.ResolveScannerProvider()
	require.Error(t, err)
}
