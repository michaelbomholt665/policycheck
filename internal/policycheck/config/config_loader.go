// internal/policycheck/config/config_loader.go
package config

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

// Load decodes the raw TOML, applies defaults, and validates the configuration.
// The source parameter is used for error reporting (e.g. filename).
func Load(source string, raw []byte) (*PolicyConfig, error) {
	var cfg PolicyConfig
	if len(raw) > 0 {
		dec := toml.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&cfg); err != nil {
			var strictErr *toml.StrictMissingError
			if errors.As(err, &strictErr) {
				return nil, fmt.Errorf("%s: strict mode error: %s", source, strictErr.String())
			}
			var decodeErr *toml.DecodeError
			if errors.As(err, &decodeErr) {
				row, col := decodeErr.Position()
				return nil, fmt.Errorf("%s:%d:%d: %w", source, row, col, err)
			}
			return nil, fmt.Errorf("%s: decode error: %w", source, err)
		}
	}

	if err := ApplyPolicyConfigDefaults(&cfg); err != nil {
		return nil, fmt.Errorf("%s: apply defaults: %w", source, err)
	}

	if err := ValidatePolicyConfig(&cfg); err != nil {
		return nil, fmt.Errorf("%s: validation error: %w", source, err)
	}

	return &cfg, nil
}
