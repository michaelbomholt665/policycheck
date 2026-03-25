// internal/tests/policycheck/config/loader_test.go
package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/policycheck/config"
)

func TestLoad(t *testing.T) {
	t.Run("Empty Config", func(t *testing.T) {
		_, err := config.Load("test.toml", []byte(""))
		require.NoError(t, err)
	})

	t.Run("Valid TOML", func(t *testing.T) {
		tomlData := []byte(`
[go_version]
allowed_prefixes = ["1.20"]

[scope_guard]
`)
		cfg, err := config.Load("test.toml", tomlData)
		require.NoError(t, err)
		assert.Equal(t, []string{"1.20"}, cfg.GoVersion.AllowedPrefixes)
	})

	t.Run("Invalid TOML", func(t *testing.T) {
		tomlData := []byte(`[invalid syntax`)
		_, err := config.Load("test.toml", tomlData)
		assert.ErrorContains(t, err, "test.toml: decode error")
	})

	t.Run("Validation Failure", func(t *testing.T) {
		tomlData := []byte(`
[[custom_rules]]
id = "bad"
pattern = "(unclosed"
severity = "warn"
enabled = true
`)
		_, err := config.Load("test.toml", tomlData)
		assert.ErrorContains(t, err, "test.toml: validation error: custom_rules[0] (bad): invalid pattern")
	})
}
