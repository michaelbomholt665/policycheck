package exec_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cliwrapperadapter "policycheck/internal/adapters/cliwrapper"
)

func TestHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	mode := helperMode()
	switch mode {
	case "sleep":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case "exit7":
		_, _ = fmt.Fprintln(os.Stderr, "child process failed")
		os.Exit(7)
	default:
		os.Exit(2)
	}
}

// TestOsExec_ContextCancelled verifies the managed subprocess is interrupted
// when the wrapper context is cancelled.
func TestOsExec_ContextCancelled(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := cliwrapperadapter.OsExec(ctx, helperCommandArgs("sleep"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

// TestOsExec_CommandExitError verifies child-process failures are surfaced as
// CommandExitError values rather than generic wrapper failures.
func TestOsExec_CommandExitError(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	err := cliwrapperadapter.OsExec(context.Background(), helperCommandArgs("exit7"))
	require.Error(t, err)

	var exitErr *cliwrapperadapter.CommandExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 7, exitErr.ExitCode)
}

func helperCommandArgs(mode string) []string {
	return []string{os.Args[0], "-test.run=TestHelperProcess", "--", mode}
}

func helperMode() string {
	for index, arg := range os.Args {
		if arg == "--" && index+1 < len(os.Args) {
			return strings.TrimSpace(os.Args[index+1])
		}
	}

	return ""
}
