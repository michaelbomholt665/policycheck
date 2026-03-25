// internal/tests/policycheck/walk/walk_test.go
package walk_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"policycheck/internal/adapters/walk"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalkAdapter(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0o644))

	adapter := &walk.Adapter{}

	visited := []string{}
	err := adapter.WalkDirectoryTree(tempDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		visited = append(visited, path)
		return nil
	})

	require.NoError(t, err)
	assert.Contains(t, visited, tempDir)
	assert.Contains(t, visited, testFile)
}
