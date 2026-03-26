// internal/tests/app/run_test.go
package run_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"policycheck/internal/app"
)

func TestRun_Routing(t *testing.T) {
	ctx := context.Background()

	t.Run("wrapper command is routed to wrapper", func(t *testing.T) {
		var wrapperCalled bool
		var analysisCalled bool

		analysisHandler := func(args []string) int {
			analysisCalled = true
			return 0
		}
		wrapperHandler := func(ctx context.Context, args []string) error {
			wrapperCalled = true
			assert.Equal(t, []string{"uv", "add", "fastapi"}, args)
			return nil
		}

		exitCode := app.RunWithHandlers(ctx, []string{"uv", "add", "fastapi"}, analysisHandler, wrapperHandler)

		assert.Equal(t, 0, exitCode)
		assert.True(t, wrapperCalled)
		assert.False(t, analysisCalled)
	})

	t.Run("legacy analysis command is routed to analysis", func(t *testing.T) {
		var wrapperCalled bool
		var analysisCalled bool

		analysisHandler := func(args []string) int {
			analysisCalled = true
			assert.Equal(t, []string{"--policy-list"}, args)
			return 0
		}
		wrapperHandler := func(ctx context.Context, args []string) error {
			wrapperCalled = true
			return nil
		}

		exitCode := app.RunWithHandlers(ctx, []string{"--policy-list"}, analysisHandler, wrapperHandler)

		assert.Equal(t, 0, exitCode)
		assert.False(t, wrapperCalled)
		assert.True(t, analysisCalled)
	})

	t.Run("explicit check command is routed to analysis", func(t *testing.T) {
		var wrapperCalled bool
		var analysisCalled bool

		analysisHandler := func(args []string) int {
			analysisCalled = true
			assert.Equal(t, []string{"--policy-list"}, args)
			return 0
		}
		wrapperHandler := func(ctx context.Context, args []string) error {
			wrapperCalled = true
			return nil
		}

		exitCode := app.RunWithHandlers(ctx, []string{"check", "--policy-list"}, analysisHandler, wrapperHandler)

		assert.Equal(t, 0, exitCode)
		assert.False(t, wrapperCalled)
		assert.True(t, analysisCalled)
	})

	t.Run("fmt headers command is routed to wrapper", func(t *testing.T) {
		var wrapperCalled bool
		var analysisCalled bool

		analysisHandler := func(args []string) int {
			analysisCalled = true
			return 0
		}
		wrapperHandler := func(ctx context.Context, args []string) error {
			wrapperCalled = true
			assert.Equal(t, []string{"fmt", "headers", "--dry-run"}, args)
			return nil
		}

		exitCode := app.RunWithHandlers(ctx, []string{"fmt", "headers", "--dry-run"}, analysisHandler, wrapperHandler)

		assert.Equal(t, 0, exitCode)
		assert.True(t, wrapperCalled)
		assert.False(t, analysisCalled)
	})

	t.Run("run macro command is routed to wrapper", func(t *testing.T) {
		var wrapperCalled bool
		var analysisCalled bool

		analysisHandler := func(args []string) int {
			analysisCalled = true
			return 0
		}
		wrapperHandler := func(ctx context.Context, args []string) error {
			wrapperCalled = true
			assert.Equal(t, []string{"run", "fmt"}, args)
			return nil
		}

		exitCode := app.RunWithHandlers(ctx, []string{"run", "fmt"}, analysisHandler, wrapperHandler)

		assert.Equal(t, 0, exitCode)
		assert.True(t, wrapperCalled)
		assert.False(t, analysisCalled)
	})

	t.Run("-then chain command is routed to wrapper", func(t *testing.T) {
		var wrapperCalled bool
		var analysisCalled bool

		analysisHandler := func(args []string) int {
			analysisCalled = true
			return 0
		}
		wrapperHandler := func(ctx context.Context, args []string) error {
			wrapperCalled = true
			assert.Equal(t, []string{"gofumpt", "-l", ".", "-then", "go", "test", "./..."}, args)
			return nil
		}

		exitCode := app.RunWithHandlers(ctx, []string{"gofumpt", "-l", ".", "-then", "go", "test", "./..."}, analysisHandler, wrapperHandler)

		assert.Equal(t, 0, exitCode)
		assert.True(t, wrapperCalled)
		assert.False(t, analysisCalled)
	})
}
