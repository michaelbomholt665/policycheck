// internal/policycheck/core/security_catalog.go
// Defines static secret-detection severities, patterns, and lookups.

package core

const ScopeProjectRepo = true


import "regexp"

const (
	secretSeverityLow      = "LOW"
	secretSeverityMedium   = "MEDIUM"
	secretSeverityHigh     = "HIGH"
	secretSeverityCritical = "CRITICAL"
)

var builtInSecretPatterns = []secretPattern{
	{id: "generic_api_key_assignment", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*['"']?[a-zA-Z0-9_\-]{20,}['"']?`)},
	{id: "generic_password_assignment", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)password\s*[:=]\s*['"']?[^\s'"]{8,}['"']?`)},
	{id: "generic_secret_assignment", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)secret\s*[:=]\s*['"']?[a-zA-Z0-9_\-]{20,}['"']?`)},
	{id: "generic_token_assignment", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)token\s*[:=]\s*['"']?[a-zA-Z0-9_\-]{20,}['"']?`)},
	{id: "standalone_password_candidate", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)\b(?:pass|pwd|secret)[0-9a-zA-Z_\-]{4,}\b`)},
	{id: "authorization_bearer_header", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)Authorization:\s*Bearer`)},
	{id: "bearer_literal", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)Bearer\s+[a-zA-Z0-9_\-]{20,}`)},
	{id: "aws_access_key_id", severity: secretSeverityHigh, pattern: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{id: "aws_session_access_key_id", severity: secretSeverityHigh, pattern: regexp.MustCompile(`ASIA[0-9A-Z]{16}`)},
	{id: "aws_secret_access_key_label", severity: secretSeverityLow, pattern: regexp.MustCompile(`(?i)(?:aws)?[_-]?secret[_-]?access[_-]?key`)},
	{id: "aws_secret_access_key_assignment", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)aws[_-]?secret[_-]?access[_-]?key\s*[:=]\s*['"']?[a-zA-Z0-9/+]{40}['"']?`)},
	{id: "aws_session_token_assignment", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)aws[_-]?session[_-]?token\s*[:=]\s*['"']?[a-zA-Z0-9/+=]{100,}['"']?`)},
	{id: "gcp_api_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`AIza[a-zA-Z0-9_\-]{35}`)},
	{id: "google_oauth_client_secret", severity: secretSeverityHigh, pattern: regexp.MustCompile(`GOCSPX-[a-zA-Z0-9_\-]{28}`)},
	{id: "gcp_service_account_type", severity: secretSeverityLow, pattern: regexp.MustCompile(`"type"\s*:\s*"service_account"`)},
	{id: "gcp_private_key_id", severity: secretSeverityHigh, pattern: regexp.MustCompile(`"private_key_id"\s*:\s*"[a-f0-9]{40}"`)},
	{id: "github_pat_classic", severity: secretSeverityCritical, pattern: regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`)},
	{id: "github_oauth_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`)},
	{id: "github_user_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`ghu_[a-zA-Z0-9]{36}`)},
	{id: "github_refresh_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`ghr_[a-zA-Z0-9]{36}`)},
	{id: "github_pat_fine_grained", severity: secretSeverityCritical, pattern: regexp.MustCompile(`github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}`)},
	{id: "github_actions_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`ghs_[a-zA-Z0-9]{36}`)},
	{id: "gitlab_pat", severity: secretSeverityCritical, pattern: regexp.MustCompile(`glpat-[a-zA-Z0-9\-_]{20,}`)},
	{id: "openai_legacy_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`sk-[a-zA-Z0-9]{48}`)},
	{id: "openai_project_key", severity: secretSeverityCritical, pattern: regexp.MustCompile(`sk-proj-[a-zA-Z0-9\-_]{48,}`)},
	{id: "anthropic_api_key", severity: secretSeverityCritical, pattern: regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-_]{95}`)},
	{id: "stripe_secret_key_live", severity: secretSeverityCritical, pattern: regexp.MustCompile(`sk_live_[a-zA-Z0-9]{24}`)},
	{id: "stripe_secret_key_test", severity: secretSeverityHigh, pattern: regexp.MustCompile(`sk_test_[a-zA-Z0-9]{24}`)},
	{id: "slack_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`xox[baprs]-[a-zA-Z0-9\-]{10,}`)},
	{id: "slack_webhook", severity: secretSeverityCritical, pattern: regexp.MustCompile(`https://hooks\.slack\.com/services/`)},
	{id: "npm_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`npm_[a-zA-Z0-9]{36}`)},
	{id: "go_secret_assignment", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)(?:password|secret|token|key|api[_-]?key)\s*=\s*"[a-zA-Z0-9_\-]{8,}"`)},
	{id: "go_struct_secret_literal", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)(?:Password|Secret(?:Key)?|APIKey|ApiKey|Token|AccessKey)\s*:\s*"[^"]{8,}"`)},
	{id: "rsa_private_key_pem", severity: secretSeverityCritical, pattern: regexp.MustCompile(`-----BEGIN\s+RSA\s+PRIVATE\s+KEY-----`)},
	{id: "openssh_private_key_pem", severity: secretSeverityCritical, pattern: regexp.MustCompile(`-----BEGIN\s+OPENSSH\s+PRIVATE\s+KEY-----`)},
	{id: "jwt_token", severity: secretSeverityMedium, pattern: regexp.MustCompile(`eyJ[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]*`)},
	{id: "hex_secret_64", severity: secretSeverityLow, pattern: regexp.MustCompile(`\b[a-f0-9]{64}\b`)},
	{id: "hex_secret_32", severity: secretSeverityLow, pattern: regexp.MustCompile(`\b[a-f0-9]{32}\b`)},
}

var knownLoggerIdentifiers = map[string]struct{}{
	"ctxlog":  {},
	"fmt":     {},
	"l":       {},
	"log":     {},
	"logger":  {},
	"os":      {},
	"zap":     {},
	"zerolog": {},
}

var genericSecretPatternIDs = map[string]struct{}{
	"authorization_bearer_header":   {},
	"aws_secret_access_key_label":   {},
	"gcp_service_account_type":      {},
	"generic_api_key_assignment":    {},
	"generic_password_assignment":   {},
	"generic_secret_assignment":     {},
	"generic_token_assignment":      {},
	"standalone_password_candidate": {},
	"hex_secret_32":                 {},
	"hex_secret_64":                 {},
}

var obviousPlaceholderSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:x|a|1){16,}\b`),
	regexp.MustCompile(`(?i)(?:ghp_|glpat-|sk-|sk-proj-)(?:x|a|1){16,}`),
	regexp.MustCompile(`(?i)\b(?:your|my|test)[_-]?(?:api|access|secret)[_-]?(?:key|token)\b`),
}
