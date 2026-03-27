package fmt_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/cliwrapper"
)

func TestHeader_GoFile_StaleHeader_ReplacesNotDuplicates(t *testing.T) {
	t.Parallel()

	got, err := cliwrapper.InjectHeader("// internal/old/main.go\npackage main\n", "go", "internal/main.go")

	require.NoError(t, err)
	assert.Equal(t, "// internal/main.go\npackage main\n", got)
	assert.Equal(t, 2, strings.Count(got, "\n"))
}

func TestHeader_PythonFile_ExistingShebang_PreservesItAndInjectsPath(t *testing.T) {
	t.Parallel()

	got, err := cliwrapper.InjectHeader("#!/usr/bin/env python3\nimport os\n", "python", "scripts/run.py")

	require.NoError(t, err)
	assert.Equal(t, "#!/usr/bin/env python3\n# scripts/run.py\nimport os\n", got)
}

func TestHeader_TypeScriptFile_MissingHeader_InjectsOnLineOne(t *testing.T) {
	t.Parallel()

	got, err := cliwrapper.InjectHeader("export const x = 1\n", "typescript", "src/utils/x.ts")

	require.NoError(t, err)
	assert.Equal(t, "// src/utils/x.ts\nexport const x = 1\n", got)
}

func TestHeader_HasHeader_ReturnsFalseForStalePath(t *testing.T) {
	t.Parallel()

	assert.False(t, cliwrapper.HasHeader("// internal/old/main.go\npackage main\n", "go", "internal/main.go"))
}

func TestHeader_GoDocFile_PreservesPackageDocBlock(t *testing.T) {
	t.Parallel()

	input := "// Package config Provides the router-backed configuration adapter.\n//\n// Package Concerns:\n// - Loading raw TOML from the host filesystem.\npackage config\n"

	got, err := cliwrapper.InjectHeader(input, "go", "internal/adapters/config/doc.go")

	require.NoError(t, err)
	assert.Equal(
		t,
		"// internal/adapters/config/doc.go\n\n// Package config Provides the router-backed configuration adapter.\n//\n// Package Concerns:\n// - Loading raw TOML from the host filesystem.\npackage config\n",
		got,
	)
}

func TestHeader_GoBuildTagFile_PreservesBuildConstraintLead(t *testing.T) {
	t.Parallel()

	input := "//go:build !windows\n\npackage cliwrapper\n"

	got, err := cliwrapper.InjectHeader(input, "go", "internal/adapters/cliwrapper/exec_unix.go")

	require.NoError(t, err)
	assert.Equal(
		t,
		"//go:build !windows\n\n// internal/adapters/cliwrapper/exec_unix.go\npackage cliwrapper\n",
		got,
	)
}
