// internal/tests/cliwrapper/detector/detector_test.go
package detector_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"policycheck/internal/cliwrapper"
)

// TestWrapperDetector_EmptyArgs_Passthrough verifies that no args produces passthrough.
func TestWrapperDetector_EmptyArgs_Passthrough(t *testing.T) {
	t.Parallel()

	d := cliwrapper.WrapperDetector{}
	assert.Equal(t, cliwrapper.ModePassthrough, d.Detect(nil, nil))
	assert.Equal(t, cliwrapper.ModePassthrough, d.Detect([]string{}, nil))
}

// TestWrapperDetector_PackageGate is a table-driven test for recognised install commands.
func TestWrapperDetector_PackageGate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"go get", []string{"go", "get", "github.com/foo/bar"}},
		{"pip install", []string{"pip", "install", "requests"}},
		{"npm install", []string{"npm", "install", "lodash"}},
		{"npm i shorthand", []string{"npm", "i", "lodash"}},
		{"uv add", []string{"uv", "add", "httpx"}},
	}

	d := cliwrapper.WrapperDetector{}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, cliwrapper.ModePackageGate, d.Detect(c.args, nil))
		})
	}
}

// TestWrapperDetector_ToolingGate confirms that known managed tools trigger the tooling gate.
func TestWrapperDetector_ToolingGate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"go build", []string{"go", "build", "./..."}},
		{"go test", []string{"go", "test", "./..."}},
		{"pip list", []string{"pip", "list"}},
		{"npm run", []string{"npm", "run", "build"}},
	}

	d := cliwrapper.WrapperDetector{}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// go test is a tooling gate regression guard — it must NOT become passthrough.
			assert.Equal(t, cliwrapper.ModeToolingGate, d.Detect(c.args, nil))
		})
	}
}

// TestWrapperDetector_MacroRun verifies that a registered macro name takes precedence.
func TestWrapperDetector_MacroRun(t *testing.T) {
	t.Parallel()

	macros := []string{"ci", "deploy"}
	d := cliwrapper.WrapperDetector{}

	assert.Equal(t, cliwrapper.ModeMacroRun, d.Detect([]string{"ci", "--dry-run"}, macros))
	assert.Equal(t, cliwrapper.ModeMacroRun, d.Detect([]string{"deploy"}, macros))
}

// TestWrapperDetector_MacroRun_UnknownName_Passthrough verifies that an unknown
// name does not match a macro.
func TestWrapperDetector_MacroRun_UnknownName_Passthrough(t *testing.T) {
	t.Parallel()

	macros := []string{"ci"}
	d := cliwrapper.WrapperDetector{}

	// "release" is not registered.
	assert.NotEqual(t, cliwrapper.ModeMacroRun, d.Detect([]string{"release"}, macros))
}

// TestWrapperDetector_Passthrough_Unknown confirms that an unrecognised command is
// treated as passthrough by default.
func TestWrapperDetector_Passthrough_Unknown(t *testing.T) {
	t.Parallel()

	d := cliwrapper.WrapperDetector{}
	assert.Equal(t, cliwrapper.ModePassthrough, d.Detect([]string{"rustc", "--edition", "2021"}, nil))
}

// TestWrapperDetector_MacroPrecedence_OverPackageGate proves that a macro name
// that looks like a package manager still resolves to MacroRun.
func TestWrapperDetector_MacroPrecedence_OverPackageGate(t *testing.T) {
	t.Parallel()

	// "go" is registered as a macro — macro must win.
	macros := []string{"go"}
	d := cliwrapper.WrapperDetector{}

	assert.Equal(t, cliwrapper.ModeMacroRun, d.Detect([]string{"go", "get", "something"}, macros))
}
