// internal/tests/policycheck/core/contracts/cli_formatter_test.go
package contracts_test

import (
	"testing"

	"policycheck/internal/policycheck/core/contracts"

	"github.com/stretchr/testify/assert"
)

func TestValidateCLIFormatter(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectedViol bool
	}{
		{
			name: "forbidden direct stdout fails",
			content: `package main
import "fmt"
func main() {
	fmt.Println("hello")
}`,
			expectedViol: true,
		},
		{
			name: "audience-aware formatter passes",
			content: `package main
func main() {
	out.Info("hello")
}`,
			expectedViol: false,
		},
		{
			name: "fmt.Sprintf passes",
			content: `package main
import "fmt"
func main() {
	s := fmt.Sprintf("hello")
}`,
			expectedViol: false,
		},
		{
			name: "fmt.Print fails",
			content: `package main
import "fmt"
func main() {
	fmt.Print("hello")
}`,
			expectedViol: true,
		},
		{
			name: "fmt.Printf fails",
			content: `package main
import "fmt"
func main() {
	fmt.Printf("hello %s", "world")
}`,
			expectedViol: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viols, err := contracts.ValidateCLIFormatter("cmd/main.go", tt.content)
			assert.NoError(t, err)
			if tt.expectedViol {
				assert.NotEmpty(t, viols)
				assert.Equal(t, "cli-formatter", viols[0].RuleID)
			} else {
				assert.Empty(t, viols)
			}
		})
	}
}
