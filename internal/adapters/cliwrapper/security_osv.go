// internal/adapters/cliwrapper/security_osv.go
package cliwrapper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	osexec "os/exec"
	"strings"
)

const (
	osvQueryBatchURL = "https://api.osv.dev/v1/querybatch"
)

// osvPURL is a single package identifier in OSV batch-query format.
type osvPURL struct {
	Package struct {
		PURL string `json:"purl"`
	} `json:"package"`
}

// osvBatchRequest is the JSON body for the OSV querybatch endpoint.
type osvBatchRequest struct {
	Queries []osvPURL `json:"queries"`
}

// osvVuln is a single vulnerability in an OSV response entry.
type osvVuln struct {
	ID       string `json:"id"`
	Summary  string `json:"summary"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
}

// osvBatchResponse is the parsed OSV querybatch response body.
type osvBatchResponse struct {
	Results []struct {
		Vulns []osvVuln `json:"vulns"`
	} `json:"results"`
}

// OSVSecurityAdapter implements ports.CLIWrapperSecurityGate using the OSV API.
//
// OSVSecurityAdapter satisfies its port by injecting dependency on the OSV HTTP
// endpoint; it does not import or call any policycheck analysis-engine code.
type OSVSecurityAdapter struct {
	threshold  Severity
	httpClient *http.Client
}

// NewOSVSecurityAdapter returns a configured OSVSecurityAdapter.
// threshold is the minimum severity that triggers a block decision.
func NewOSVSecurityAdapter(threshold Severity) *OSVSecurityAdapter {
	return &OSVSecurityAdapter{
		threshold:  threshold,
		httpClient: &http.Client{},
	}
}

// CheckPackages queries the OSV API for each purl and blocks if any advisory
// meets or exceeds the configured threshold.
//
// A scan failure (HTTP error, parse error) is treated as a block to avoid
// silent downgrade of security decisions.
func (a *OSVSecurityAdapter) CheckPackages(ctx context.Context, _ string, purls []string) error {
	if len(purls) == 0 {
		return nil
	}

	advisories, err := a.scanPackages(ctx, purls)
	if err != nil {
		return fmt.Errorf("osv security adapter: scan failed: %w", err)
	}

	result := EvaluateSeverity(a.threshold, advisories)

	if result.Decision != DecisionAllow {
		return fmt.Errorf("osv security adapter: %s", result.BlockReason)
	}

	return nil
}

// CheckLockfile scans a package-manager lockfile after install to catch
// transitive vulnerabilities. This path requires the OSV CLI because the HTTP
// API cannot evaluate lockfile graphs directly.
func (a *OSVSecurityAdapter) CheckLockfile(ctx context.Context, _ string, lockfilePath string) error {
	if strings.TrimSpace(lockfilePath) == "" {
		return fmt.Errorf("lockfile path is empty")
	}

	if _, err := os.Stat(lockfilePath); err != nil {
		return fmt.Errorf("stat lockfile %q: %w", lockfilePath, err)
	}

	cliPath, ok := lookupOSVCLI()
	if !ok {
		return fmt.Errorf("osv-scanner CLI is required for lockfile scan %q", lockfilePath)
	}

	advisories, err := a.runOSVCLILockfileScan(ctx, cliPath, lockfilePath)
	if err != nil {
		return fmt.Errorf("lockfile scan %q: %w", lockfilePath, err)
	}

	result := EvaluateSeverity(a.threshold, advisories)
	if result.Decision != DecisionAllow {
		return fmt.Errorf("osv security adapter: %s", result.BlockReason)
	}

	return nil
}

// buildOSVQueries converts a purl list into OSV query objects.
func buildOSVQueries(purls []string) []osvPURL {
	queries := make([]osvPURL, len(purls))
	for i, p := range purls {
		queries[i].Package.PURL = p
	}

	return queries
}

func (a *OSVSecurityAdapter) scanPackages(ctx context.Context, purls []string) ([]Advisory, error) {
	if cliPath, ok := lookupOSVCLI(); ok {
		advisories, err := a.runOSVCLIPackageScan(ctx, cliPath, purls)
		if err == nil {
			return advisories, nil
		}
		return a.scanPackagesViaAPI(ctx, purls, err)
	}

	return a.scanPackagesViaAPI(ctx, purls, nil)
}

func (a *OSVSecurityAdapter) scanPackagesViaAPI(
	ctx context.Context,
	purls []string,
	cliErr error,
) ([]Advisory, error) {
	queries := buildOSVQueries(purls)
	resp, err := a.runOSVQuery(ctx, queries)
	if err != nil {
		if cliErr != nil {
			return nil, errors.Join(cliErr, err)
		}

		return nil, err
	}

	return collectAdvisories(resp), nil
}

// runOSVQuery posts the batch request and returns the parsed response.
func (a *OSVSecurityAdapter) runOSVQuery(ctx context.Context, queries []osvPURL) (osvBatchResponse, error) {
	body, err := json.Marshal(osvBatchRequest{Queries: queries})
	if err != nil {
		return osvBatchResponse{}, fmt.Errorf("marshal osv request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osvQueryBatchURL, bytes.NewReader(body))
	if err != nil {
		return osvBatchResponse{}, fmt.Errorf("build osv request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	httpResp, err := a.httpClient.Do(req)
	if err != nil {
		return osvBatchResponse{}, fmt.Errorf("osv http request: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK {
		return osvBatchResponse{}, fmt.Errorf("osv returned status %d", httpResp.StatusCode)
	}

	var parsed osvBatchResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&parsed); err != nil {
		return osvBatchResponse{}, fmt.Errorf("decode osv response: %w", err)
	}

	return parsed, nil
}

func (a *OSVSecurityAdapter) runOSVCLIPackageScan(
	ctx context.Context,
	cliPath string,
	purls []string,
) ([]Advisory, error) {
	args := []string{"scan", "--format", "json"}
	for _, purl := range purls {
		args = append(args, "--package", purl)
	}

	return a.runOSVCLI(ctx, cliPath, args)
}

func (a *OSVSecurityAdapter) runOSVCLILockfileScan(
	ctx context.Context,
	cliPath string,
	lockfilePath string,
) ([]Advisory, error) {
	return a.runOSVCLI(ctx, cliPath, []string{"--lockfile=" + lockfilePath, "--format", "json"})
}

func (a *OSVSecurityAdapter) runOSVCLI(
	ctx context.Context,
	cliPath string,
	args []string,
) ([]Advisory, error) {
	cmd := newManagedCommand(ctx, cliPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf(
			"run %s %s: %w",
			cliPath,
			strings.Join(args, " "),
			wrapCommandExitError(cliPath, err),
		)
	}

	advisories, parseErr := parseOSVCLIAdvisories(output)
	if parseErr != nil {
		rawOutput := summarizeCommandOutput(output)
		if err != nil {
			return nil, fmt.Errorf(
				"run %s %s: %w; parse output: %v; raw output: %s",
				cliPath,
				strings.Join(args, " "),
				err,
				parseErr,
				rawOutput,
			)
		}

		return nil, fmt.Errorf("parse %s output: %w; raw output: %s", cliPath, parseErr, rawOutput)
	}

	return advisories, nil
}

// collectAdvisories flattens all vulnerability entries from the batch response
// into a slice of Advisory values for EvaluateSeverity.
func collectAdvisories(resp osvBatchResponse) []Advisory {
	var out []Advisory
	for _, entry := range resp.Results {
		for _, vuln := range entry.Vulns {
			sev := extractSeverityLabel(vuln)
			out = append(out, Advisory{
				ID:       vuln.ID,
				Summary:  vuln.Summary,
				Severity: sev,
			})
		}
	}

	return out
}

// extractSeverityLabel picks the first severity label from a vuln entry.
// Returns "unknown" when none is present so EvaluateSeverity treats it as critical.
func extractSeverityLabel(vuln osvVuln) string {
	if len(vuln.Severity) > 0 {
		return vuln.Severity[0].Score
	}

	return "unknown"
}

func lookupOSVCLI() (string, bool) {
	for _, candidate := range []string{"osv-scanner", "osv-scanner.exe"} {
		path, err := osexec.LookPath(candidate)
		if err == nil {
			return path, true
		}
	}

	return "", false
}

func parseOSVCLIAdvisories(raw []byte) ([]Advisory, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("empty osv cli output")
	}

	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode osv cli json: %w", err)
	}

	found := make(map[string]Advisory)
	collectCLIAdvisories(payload, found)

	advisories := make([]Advisory, 0, len(found))
	for _, advisory := range found {
		advisories = append(advisories, advisory)
	}

	return advisories, nil
}

func collectCLIAdvisories(node any, found map[string]Advisory) {
	switch typed := node.(type) {
	case map[string]any:
		if rawVulns, ok := typed["vulnerabilities"]; ok {
			appendCLIAdvisories(rawVulns, found)
		}
		if rawVulns, ok := typed["vulns"]; ok {
			appendCLIAdvisories(rawVulns, found)
		}
		for _, value := range typed {
			collectCLIAdvisories(value, found)
		}
	case []any:
		for _, value := range typed {
			collectCLIAdvisories(value, found)
		}
	}
}

func appendCLIAdvisories(raw any, found map[string]Advisory) {
	items, ok := raw.([]any)
	if !ok {
		return
	}

	for _, item := range items {
		vuln, ok := item.(map[string]any)
		if !ok {
			continue
		}

		advisory := Advisory{
			ID:       stringValue(vuln["id"]),
			Summary:  stringValue(vuln["summary"]),
			Severity: extractCLISeverity(vuln),
		}
		if advisory.ID == "" {
			continue
		}

		found[advisory.ID] = advisory
	}
}

func extractCLISeverity(vuln map[string]any) string {
	if severity := strings.ToLower(strings.TrimSpace(stringValue(vuln["severity"]))); severity != "" {
		return severity
	}

	if severityList, ok := vuln["severity"].([]any); ok {
		for _, item := range severityList {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if score := strings.ToLower(strings.TrimSpace(stringValue(entry["score"]))); score != "" {
				return score
			}
		}
	}

	if dbSpecific, ok := vuln["database_specific"].(map[string]any); ok {
		if severity := strings.ToLower(strings.TrimSpace(stringValue(dbSpecific["severity"]))); severity != "" {
			return severity
		}
	}

	return "unknown"
}

func stringValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}

	return text
}

func summarizeCommandOutput(raw []byte) string {
	const maxOutputLength = 200

	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "<empty>"
	}

	if len(text) <= maxOutputLength {
		return text
	}

	return text[:maxOutputLength] + "..."
}
