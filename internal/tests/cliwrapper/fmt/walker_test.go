package fmt_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/cliwrapper"
)

func TestHeaderWalker_DryRun_ReturnsErrorWhenFilesWouldChange(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n")

	walker := newHeaderWalker(root)

	report, err := walker.Run(context.Background(), true, []string{"go"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "would be modified")
	assert.Equal(t, 1, report.Checked)
	assert.Equal(t, 1, report.Modified)
}

func TestHeaderWalker_WriteRun_IsIdempotentAndSkipsVendor(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "cmd", "main.go")
	skipped := filepath.Join(root, "vendor", "dep.go")
	writeTestFile(t, target, "package main\n")
	writeTestFile(t, skipped, "package vendor\n")

	walker := newHeaderWalker(root)

	first, err := walker.Run(context.Background(), false, []string{"go"})
	require.NoError(t, err)
	assert.Equal(t, 1, first.Modified)

	second, err := walker.Run(context.Background(), false, []string{"go"})
	require.NoError(t, err)
	assert.Equal(t, 0, second.Modified)
	assert.Equal(t, 1, second.Skipped)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "// cmd/main.go\npackage main\n", string(content))

	skippedContent, err := os.ReadFile(skipped)
	require.NoError(t, err)
	assert.Equal(t, "package vendor\n", string(skippedContent))
}

func newHeaderWalker(root string) cliwrapper.HeaderWalker {
	return cliwrapper.HeaderWalker{
		Root: root,
		Walk: func(root string, walkFn fs.WalkDirFunc) error {
			return filepath.WalkDir(root, walkFn)
		},
		ReadFile: os.ReadFile,
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
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
}
