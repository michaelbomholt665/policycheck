package fmt_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cliwrapperadapter "policycheck/internal/adapters/cliwrapper"
	"policycheck/internal/ports"
)

type stubWalkProvider struct{}

func (stubWalkProvider) WalkDirectoryTree(root string, walkFn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, walkFn)
}

func TestHeaderFormatterAdapter_List_PrintsChangedPathsOnDryRun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "cmd", "main.go"), "package main\n")

	var output bytes.Buffer
	adapter := cliwrapperadapter.NewHeaderFormatterAdapterWithDeps(cliwrapperadapter.HeaderFormatterDeps{
		ResolveWalk: func() (ports.WalkProvider, error) {
			return stubWalkProvider{}, nil
		},
		ResolveRoot: func() (string, error) { return root, nil },
		ReadFile:    os.ReadFile,
		FileMode: func(path string) (os.FileMode, error) {
			info, err := os.Stat(path)
			if err != nil {
				return 0, err
			}

			return info.Mode(), nil
		},
		WriteFile: func(path string, data []byte, mode os.FileMode) error {
			return os.WriteFile(path, data, mode)
		},
		Output: &output,
	})

	err := adapter.FormatHeaders(context.Background(), true, true, []string{"go"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "would be modified")
	assert.Equal(t, "cmd/main.go\n", output.String())
}
