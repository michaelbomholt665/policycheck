package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

const liveReportPath = "/report"

// RouterRunLiveCommand executes the explicit live verification command tree.
func RouterRunLiveCommand(
	_ globalOptions,
	args []string,
	stdout io.Writer,
	_ io.Writer,
) error {
	if len(args) == 0 || RouterIsHelpToken(args[0]) {
		return RouterWriteLiveUsage(stdout)
	}
	if args[0] != "run" {
		return &usageError{message: fmt.Sprintf("unknown live subcommand %q", args[0])}
	}

	if len(args) > 1 && RouterIsHelpToken(args[1]) {
		return RouterWriteLiveRunUsage(stdout)
	}

	return RouterRunLiveSession(args[1:], stdout)
}

type liveOptions struct {
	listenAddress string
	timeout       time.Duration
	expectedIDs   []string
}

// RouterRunLiveSession starts a bounded live verification session.
func RouterRunLiveSession(args []string, stdout io.Writer) error {
	options, err := RouterParseLiveOptions(args)
	if err != nil {
		return err
	}

	session := RouterNewLiveSession(options.expectedIDs)
	server := &http.Server{
		Handler: http.HandlerFunc(session.RouterHandleLiveReport),
	}

	listener, err := net.Listen("tcp", options.listenAddress)
	if err != nil {
		return fmt.Errorf("listen for live verification on %s: %w", options.listenAddress, err)
	}

	serverErrCh := make(chan error, 1)
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			serverErrCh <- fmt.Errorf("serve live verification: %w", serveErr)
		}
	}()

	if _, err := fmt.Fprintf(
		stdout,
		"Router live listening: http://%s%s\n",
		listener.Addr().String(),
		liveReportPath,
	); err != nil {
		return fmt.Errorf("write live listener status: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "Router live awaiting participants: %d\n", len(options.expectedIDs)); err != nil {
		return fmt.Errorf("write live participant status: %w", err)
	}

	resultErr := session.RouterWaitForSessionCompletion(options.timeout, serverErrCh)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown live verification server: %w", err)
	}

	if resultErr != nil {
		return resultErr
	}

	if _, err := fmt.Fprintf(
		stdout,
		"Router live check passed: %d/%d participants reported success\n",
		len(options.expectedIDs),
		len(options.expectedIDs),
	); err != nil {
		return fmt.Errorf("write live success status: %w", err)
	}

	return nil
}

// RouterParseLiveOptions parses live-run specific CLI flags.
func RouterParseLiveOptions(args []string) (liveOptions, error) {
	options := liveOptions{
		listenAddress: "127.0.0.1:0",
		expectedIDs:   make([]string, 0),
	}

	fs := flag.NewFlagSet("live run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&options.listenAddress, "listen", options.listenAddress, "listen address")
	fs.Func("expect", "expected participant id (repeatable)", func(value string) error {
		if value == "" {
			return fmt.Errorf("participant id cannot be empty")
		}

		options.expectedIDs = append(options.expectedIDs, value)
		return nil
	})

	timeoutRaw := fs.String("timeout", defaultLiveTimeout, "timeout after the first participant report")
	if err := fs.Parse(args); err != nil {
		return liveOptions{}, &usageError{message: fmt.Sprintf("parse live flags: %v", err)}
	}
	if len(fs.Args()) > 0 {
		return liveOptions{}, &usageError{message: fmt.Sprintf("unexpected live arguments: %v", fs.Args())}
	}
	if len(options.expectedIDs) == 0 {
		return liveOptions{}, &usageError{message: "live run requires at least one --expect participant id"}
	}

	timeout, err := time.ParseDuration(*timeoutRaw)
	if err != nil {
		return liveOptions{}, &usageError{message: fmt.Sprintf("parse timeout %q: %v", *timeoutRaw, err)}
	}
	options.timeout = timeout

	return options, nil
}

// RouterWriteLiveUsage prints the live command usage message.
func RouterWriteLiveUsage(writer io.Writer) error {
	lines := []string{
		"usage: Router [--root PATH] live run [flags]",
		"subcommands:",
		"  run   start a bounded live verification session",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write live usage line: %w", err)
		}
	}

	return nil
}

// RouterWriteLiveRunUsage prints the live run usage message.
func RouterWriteLiveRunUsage(writer io.Writer) error {
	lines := []string{
		"usage: Router [--root PATH] live run --expect <id> [--expect <id> ...] [flags]",
		"flags:",
		"  --expect <id>   expected participant id; repeat for each participant",
		"  --listen <addr> listen address (default 127.0.0.1:0)",
		"  --timeout <dur> timeout after the first participant report",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write live run usage line: %w", err)
		}
	}

	return nil
}

type liveSession struct {
	mu             sync.Mutex
	startedAt      time.Time
	expectedByID   map[string]struct{}
	successByID    map[string]struct{}
	failureMessage string
	startedCh      chan struct{}
	doneCh         chan struct{}
	startOnce      sync.Once
	doneOnce       sync.Once
}

type liveReport struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// RouterNewLiveSession creates a live verification session for the expected participants.
func RouterNewLiveSession(expectedIDs []string) *liveSession {
	expectedByID := make(map[string]struct{}, len(expectedIDs))
	for _, id := range expectedIDs {
		expectedByID[id] = struct{}{}
	}

	return &liveSession{
		expectedByID: expectedByID,
		successByID:  make(map[string]struct{}, len(expectedIDs)),
		startedCh:    make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// RouterHandleLiveReport validates and records one participant report.
func (s *liveSession) RouterHandleLiveReport(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != liveReportPath {
		http.NotFound(writer, request)
		return
	}

	var report liveReport
	if err := json.NewDecoder(request.Body).Decode(&report); err != nil {
		http.Error(writer, fmt.Sprintf("decode report: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.RouterRecordLiveReport(report); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	writer.WriteHeader(http.StatusAccepted)
}

// RouterRecordLiveReport records one participant result into the session state.
func (s *liveSession) RouterRecordLiveReport(report liveReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.startedAt.IsZero() {
		s.startedAt = time.Now()
		s.startOnce.Do(func() {
			close(s.startedCh)
		})
	}

	if _, exists := s.expectedByID[report.ID]; !exists {
		s.RouterMarkSessionFailure(fmt.Sprintf("unknown participant %q", report.ID))
		return fmt.Errorf("unknown participant %q", report.ID)
	}
	if _, exists := s.successByID[report.ID]; exists {
		s.RouterMarkSessionFailure(fmt.Sprintf("duplicate participant report %q", report.ID))
		return fmt.Errorf("duplicate participant report %q", report.ID)
	}

	switch report.Status {
	case "success":
		s.successByID[report.ID] = struct{}{}
		if len(s.successByID) == len(s.expectedByID) {
			s.RouterCloseLiveSession()
		}
		return nil
	case "failure":
		message := report.Error
		if message == "" {
			message = "participant reported failure"
		}
		s.RouterMarkSessionFailure(fmt.Sprintf("%s reported failure: %s", report.ID, message))
		return fmt.Errorf("%s reported failure: %s", report.ID, message)
	default:
		s.RouterMarkSessionFailure(fmt.Sprintf("invalid participant status %q", report.Status))
		return fmt.Errorf("invalid participant status %q", report.Status)
	}
}

// RouterWaitForSessionCompletion blocks until the live session succeeds, fails, or times out.
func (s *liveSession) RouterWaitForSessionCompletion(timeout time.Duration, serverErrCh <-chan error) error {
	var timeoutTimer *time.Timer
	var timeoutCh <-chan time.Time
	startedCh := s.startedCh
	defer func() {
		if timeoutTimer != nil {
			timeoutTimer.Stop()
		}
	}()

	for {
		select {
		case err := <-serverErrCh:
			return err
		case <-s.doneCh:
			return s.RouterBuildCompletionError()
		case <-startedCh:
			startedCh = nil
			timeoutTimer = time.NewTimer(timeout)
			timeoutCh = timeoutTimer.C
		case <-timeoutCh:
			return &verificationBugError{
				message: fmt.Sprintf(
					"Router live check timed out after %s: verification session did not complete; this is a bug",
					timeout,
				),
			}
		}
	}
}

// RouterBuildCompletionError returns the session's terminal failure, if any.
func (s *liveSession) RouterBuildCompletionError() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.failureMessage == "" {
		return nil
	}

	return fmt.Errorf("Router live check failed: %s", s.failureMessage)
}

// RouterCloseLiveSession closes the session once a terminal state is reached.
func (s *liveSession) RouterCloseLiveSession() {
	s.doneOnce.Do(func() {
		close(s.doneCh)
	})
}

// RouterMarkSessionFailure records a failure and finalizes the session.
func (s *liveSession) RouterMarkSessionFailure(message string) {
	s.failureMessage = message
	s.RouterCloseLiveSession()
}

type verificationBugError struct {
	message string
}

// Error returns the verification bug message.
func (e *verificationBugError) Error() string {
	return e.message
}
