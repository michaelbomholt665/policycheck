package cli_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/router"
	"policycheck/internal/router/capabilities"

	policycli "policycheck/internal/policycheck/cli"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedOutput struct {
	data []byte
	err  error
}

type stubOutputStyler struct {
	lastKind    string
	lastHeaders []string
	lastRows    [][]string
}

func (s *stubOutputStyler) StyleText(kind, input string) (string, error) {
	return fmt.Sprintf("<%s>%s</%s>", kind, input, kind), nil
}

func (s *stubOutputStyler) StyleTable(kind string, headers []string, rows [][]string) (string, error) {
	s.lastKind = kind
	s.lastHeaders = append([]string(nil), headers...)
	s.lastRows = append([][]string(nil), rows...)
	return fmt.Sprintf("TABLE[%s]", kind), nil
}

func (s *stubOutputStyler) StyleLayout(kind, title string, content ...string) (string, error) {
	return fmt.Sprintf("LAYOUT[%s:%s]", kind, title), nil
}

type stubChromeStyler struct{}

func (s *stubChromeStyler) StyleText(kind, input string) (string, error) {
	if input == "" {
		switch kind {
		case capabilities.TextKindWarning:
			return "[WARN] ", nil
		case capabilities.TextKindError:
			return "[ERROR] ", nil
		default:
			return "[" + strings.ToUpper(kind) + "] ", nil
		}
	}

	return fmt.Sprintf("<%s>%s</%s>", kind, input, kind), nil
}

func (s *stubChromeStyler) StyleLayout(kind, title string, content ...string) (string, error) {
	return fmt.Sprintf("PANEL[%s]\n%s", title, strings.Join(content, "\n")), nil
}

type stubInteractor struct {
	responses []any
	callCount int
}

func (s *stubInteractor) StylePrompt(kind, title, description string, options []capabilities.Choice) (any, error) {
	if s.callCount >= len(s.responses) {
		return nil, fmt.Errorf("unexpected prompt %s", title)
	}

	response := s.responses[s.callCount]
	s.callCount++
	return response, nil
}

type mockCapabilityExtension struct {
	ports    []router.PortName
	provider router.Provider
}

func (m *mockCapabilityExtension) Required() bool              { return true }
func (m *mockCapabilityExtension) Consumes() []router.PortName { return nil }
func (m *mockCapabilityExtension) Provides() []router.PortName {
	return append([]router.PortName(nil), m.ports...)
}

func (m *mockCapabilityExtension) RouterProvideRegistration(reg *router.Registry) error {
	for _, port := range m.ports {
		if err := reg.RouterRegisterProvider(port, m.provider); err != nil {
			return fmt.Errorf("register mock capability %s: %w", port, err)
		}
	}

	return nil
}

func TestResolveRenderers_WithRegisteredCapabilities(t *testing.T) {
	router.RouterResetForTest()
	defer router.RouterResetForTest()

	outputProvider := &stubOutputStyler{}
	chromeProvider := &stubChromeStyler{}
	interactorProvider := &stubInteractor{}

	_, err := router.RouterLoadExtensions(nil, []router.Extension{
		&mockCapabilityExtension{
			ports:    []router.PortName{router.PortCLIStyle},
			provider: outputProvider,
		},
		&mockCapabilityExtension{
			ports: []router.PortName{router.PortCLIChrome, router.PortCLIInteraction},
			provider: struct {
				*stubChromeStyler
				*stubInteractor
			}{
				stubChromeStyler: chromeProvider,
				stubInteractor:   interactorProvider,
			},
		},
	}, context.Background())
	require.NoError(t, err)

	renderers, err := policycli.ResolveRenderers()
	require.NoError(t, err)
	assert.NotNil(t, renderers.Output)
	assert.NotNil(t, renderers.Chrome)
	assert.NotNil(t, renderers.Interactor)
}

func TestResolveRenderers_MissingOptionalCapabilities(t *testing.T) {
	router.RouterResetForTest()
	defer router.RouterResetForTest()

	_, err := router.RouterLoadExtensions(nil, nil, context.Background())
	require.NoError(t, err)

	renderers, err := policycli.ResolveRenderers()
	require.NoError(t, err)
	assert.Nil(t, renderers.Output)
	assert.Nil(t, renderers.Chrome)
	assert.Nil(t, renderers.Interactor)
}

func TestPrintViolations_UsesChromeStyling(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		err := policycli.PrintViolations(&stubChromeStyler{}, []types.Violation{{
			RuleID:   "function-quality",
			File:     "internal/policycheck/cli/rules.go",
			Function: "RunPolicy",
			Line:     42,
			Message:  "function RunPolicy has elevated complexity",
			Severity: "warn",
		}})
		require.NoError(t, err)
	})

	assert.Contains(t, stdout, "[WARN] ")
	assert.Contains(t, stdout, "<muted>internal/policycheck/cli/rules.go:RunPolicy:42:</muted>")
	assert.Contains(t, stdout, "function RunPolicy has elevated complexity [function-quality]")
}

func TestArrangeViolationsForCLI_GroupsSeverityAndRule(t *testing.T) {
	arranged := policycli.ArrangeViolationsForCLI([]types.Violation{
		{RuleID: "function-quality", Severity: "warn", File: "z.go", Line: 9},
		{RuleID: "go-version", Severity: "error", File: "go.mod", Line: 1},
		{RuleID: "function-quality", Severity: "error", File: "a.go", Line: 3},
		{RuleID: "architecture", Severity: "warn", File: "b.go", Line: 7},
	})

	require.Len(t, arranged, 4)
	assert.Equal(t, "function-quality", arranged[0].RuleID)
	assert.Equal(t, "error", arranged[0].Severity)
	assert.Equal(t, "go-version", arranged[1].RuleID)
	assert.Equal(t, "error", arranged[1].Severity)
	assert.Equal(t, "architecture", arranged[2].RuleID)
	assert.Equal(t, "warn", arranged[2].Severity)
	assert.Equal(t, "function-quality", arranged[3].RuleID)
	assert.Equal(t, "warn", arranged[3].Severity)
}

func TestPrintViolations_InsertsSeparatorBetweenGroups(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		err := policycli.PrintViolations(&stubChromeStyler{}, []types.Violation{
			{RuleID: "go-version", File: "go.mod", Line: 1, Message: "missing toolchain", Severity: "error"},
			{RuleID: "function-quality", File: "a.go", Line: 2, Message: "too complex", Severity: "warn"},
		})
		require.NoError(t, err)
	})

	assert.Contains(t, stdout, "missing toolchain [go-version]\n\n[WARN] ")
}

func TestSummarizeWarnings_CompressesMildContextWarnings(t *testing.T) {
	cfg := config.PolicyConfig{}
	cfg.Output.MildCTXCompressSummary = true
	cfg.Output.MildCTXSummaryMinFunctions = 2
	cfg.Output.MildCTXPerFileSummaryMinCount = 0
	cfg.FunctionQuality.MildCTXMin = 10
	cfg.FunctionQuality.ElevatedCTXMin = 13

	summarized := policycli.SummarizeWarnings(cfg, []types.Violation{
		{RuleID: "function-quality.mild-ctx", Message: "first", Severity: "warn"},
		{RuleID: "function-quality.mild-ctx", Message: "second", Severity: "warn"},
	})

	require.Len(t, summarized, 1)
	assert.Equal(t, "function-quality", summarized[0].RuleID)
	assert.Contains(t, summarized[0].Message, "2 functions have low CTX violations")
}

func TestSummarizeWarnings_CompressesPerFileMildContextWarnings(t *testing.T) {
	cfg := config.PolicyConfig{}
	cfg.Output.MildCTXCompressSummary = true
	cfg.Output.MildCTXSummaryMinFunctions = 10
	cfg.Output.MildCTXPerFileSummaryMinCount = 2
	cfg.Output.MildCTXPerFileEscalationCount = 3
	cfg.FunctionQuality.MildCTXMin = 10
	cfg.FunctionQuality.ElevatedCTXMin = 13

	summarized := policycli.SummarizeWarnings(cfg, []types.Violation{
		{RuleID: "function-quality.mild-ctx", File: "internal/app/a.go", Message: "first", Severity: "warn"},
		{RuleID: "function-quality.mild-ctx", File: "internal/app/a.go", Message: "second", Severity: "warn"},
		{RuleID: "function-quality.mild-ctx", File: "internal/app/a.go", Message: "third", Severity: "warn"},
		{RuleID: "function-quality.mild-ctx", File: "internal/app/b.go", Message: "fourth", Severity: "warn"},
	})

	require.Len(t, summarized, 2)
	assert.Equal(t, "function-quality", summarized[0].RuleID)
	assert.Equal(t, "internal/app/a.go", summarized[0].File)
	assert.Contains(t, summarized[0].Message, "hotspot threshold is 3")
	assert.Equal(t, "function-quality", summarized[1].RuleID)
	assert.Equal(t, "fourth", summarized[1].Message)
}

func TestPrintPolicyList_UsesOutputStylerTable(t *testing.T) {
	outputStyler := &stubOutputStyler{}

	stdout, _ := captureOutput(t, func() {
		err := policycli.PrintPolicyList(policycli.Renderers{
			Output: outputStyler,
			Chrome: &stubChromeStyler{},
		})
		require.NoError(t, err)
	})

	assert.Contains(t, stdout, "<header>Active Policy Groups</header>")
	assert.Contains(t, stdout, "TABLE[table.compact]")
	assert.Equal(t, capabilities.TableKindCompact, outputStyler.lastKind)
	assert.Equal(t, []string{"Category", "Checks"}, outputStyler.lastHeaders)
}

func TestPrintRuleDescriptions_FallsBackWithoutCapabilities(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		err := policycli.PrintRuleDescriptions(policycli.Renderers{})
		require.NoError(t, err)
	})

	assert.Contains(t, stdout, "Enforced Policy Rules")
	assert.Contains(t, stdout, "Function Quality")
	assert.Contains(t, stdout, "Rule ID: function-quality")
}

func TestRunInteractivePolicyCatalog_SelectsRule(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		err := policycli.RunInteractivePolicyCatalog(policycli.Renderers{
			Chrome: &stubChromeStyler{},
			Interactor: &stubInteractor{
				responses: []any{"rules", "function-quality"},
			},
		})
		require.NoError(t, err)
	})

	assert.Contains(t, stdout, "PANEL[Function Quality]")
	assert.Contains(t, stdout, "Rule ID: function-quality")
}

func TestRunInteractivePolicyCatalog_FallsBackWithoutInteractor(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		err := policycli.RunInteractivePolicyCatalog(policycli.Renderers{
			Chrome: &stubChromeStyler{},
		})
		require.NoError(t, err)
	})

	assert.Contains(t, stderr, "interactive CLI capability unavailable")
	assert.Contains(t, stdout, "Enforced Policy Rules")
}

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)

	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	stdoutDone := make(chan capturedOutput, 1)
	go func() {
		data, readErr := io.ReadAll(stdoutReader)
		stdoutDone <- capturedOutput{data: data, err: readErr}
	}()

	stderrDone := make(chan capturedOutput, 1)
	go func() {
		data, readErr := io.ReadAll(stderrReader)
		stderrDone <- capturedOutput{data: data, err: readErr}
	}()

	fn()

	require.NoError(t, stdoutWriter.Close())
	require.NoError(t, stderrWriter.Close())

	stdoutResult := <-stdoutDone
	require.NoError(t, stdoutResult.err)

	stderrResult := <-stderrDone
	require.NoError(t, stderrResult.err)

	return string(stdoutResult.data), string(stderrResult.data)
}
