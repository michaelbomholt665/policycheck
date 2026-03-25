// internal/policycheck/core/security/secret_catalog.go
package security

import (
	"regexp"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
)

const (
	SecretSeverityLow      = "LOW"
	SecretSeverityMedium   = "MEDIUM"
	SecretSeverityHigh     = "HIGH"
	SecretSeverityCritical = "CRITICAL"
)

// SecretPattern defines a compiled regex and its associated metadata.
type SecretPattern struct {
	ID       string
	Pattern  *regexp.Regexp
	Severity string
}

// BuiltInPatterns returns the default set of high-confidence secret patterns.
func BuiltInPatterns() []SecretPattern {
	return []SecretPattern{
		{ID: "generic_api_key_assignment", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9_\-]{20,}['"]?`)},
		{ID: "generic_password_assignment", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)password\s*[:=]\s*['"]?[^\s'"]{8,}['"]?`)},
		{ID: "generic_secret_assignment", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)secret\s*[:=]\s*['"]?[a-zA-Z0-9_\-]{20,}['"]?`)},
		{ID: "generic_token_assignment", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)token\s*[:=]\s*['"]?[a-zA-Z0-9_\-]{20,}['"]?`)},
		{ID: "standalone_password_candidate", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)\b(?:pass|pwd|secret)(?:word)?[a-zA-Z0-9]*[0-9]{2,}[a-zA-Z0-9]*\b`)},
		{ID: "authorization_bearer_header", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)Authorization:\s*Bearer`)},
		{ID: "bearer_literal", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)Bearer\s+[a-zA-Z0-9_\-]{20,}`)},
		{ID: "aws_access_key_id", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
		{ID: "aws_session_access_key_id", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`ASIA[0-9A-Z]{16}`)},
		{ID: "aws_secret_access_key_label", Severity: SecretSeverityLow, Pattern: regexp.MustCompile(`(?i)(?:aws)?[_-]?secret[_-]?access[_-]?key`)},
		{ID: "aws_secret_access_key_assignment", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)aws[_-]?secret[_-]?access[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9/+]{40}['"]?`)},
		{ID: "aws_session_token_assignment", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)aws[_-]?session[_-]?token\s*[:=]\s*['"]?[a-zA-Z0-9/+=]{100,}['"]?`)},
		{ID: "gcp_api_key", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`AIza[a-zA-Z0-9_\-]{35}`)},
		{ID: "google_oauth_client_secret", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`GOCSPX-[a-zA-Z0-9_\-]{28}`)},
		{ID: "gcp_service_account_type", Severity: SecretSeverityLow, Pattern: regexp.MustCompile(`"type"\s*:\s*"service_account"`)},
		{ID: "gcp_private_key_id", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`"private_key_id"\s*:\s*"[a-f0-9]{40}"`)},
		{ID: "azure_storage_connection_string", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`DefaultEndpointsProtocol=https;AccountName=[^;]+;AccountKey=[a-zA-Z0-9/+=]{88}`)},
		{ID: "azure_storage_account_key", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`AccountKey=[a-zA-Z0-9/+=]{88}`)},
		{ID: "azure_sas_sig", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`[?&]sig=[a-zA-Z0-9%/+=]{43,}`)},
		{ID: "azure_client_secret", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)azure[_-]?client[_-]?secret\s*[:=]\s*['"]?[a-zA-Z0-9_\-~.]{32,}['"]?`)},
		{ID: "azure_subscription_key", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)(?:ocp|subscription)[_-]?apim[_-]?(?:key|subscription[_-]?key)\s*[:=]\s*['"]?[a-f0-9]{32}['"]?`)},
		{ID: "github_pat_classic", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`)},
		{ID: "github_oauth_token", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`)},
		{ID: "github_user_token", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`ghu_[a-zA-Z0-9]{36}`)},
		{ID: "github_refresh_token", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`ghr_[a-zA-Z0-9]{36}`)},
		{ID: "github_pat_fine_grained", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}`)},
		{ID: "github_actions_token", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`ghs_[a-zA-Z0-9]{36}`)},
		{ID: "gitlab_pat", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`glpat-[a-zA-Z0-9\-_]{20,}`)},
		{ID: "gitlab_trigger_token", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`glptt-[a-f0-9]{40}`)},
		{ID: "gitlab_runner_token", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`GR1348941[a-zA-Z0-9\-_]{20}`)},
		{ID: "openai_legacy_key", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`sk-[a-zA-Z0-9]{48}`)},
		{ID: "openai_project_key", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`sk-proj-[a-zA-Z0-9\-_]{48,}`)},
		{ID: "anthropic_api_key", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-_]{95}`)},
		{ID: "stripe_secret_key_live", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`sk_live_[a-zA-Z0-9]{24}`)},
		{ID: "stripe_secret_key_test", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`sk_test_[a-zA-Z0-9]{24}`)},
		{ID: "stripe_publishable_key_live", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`pk_live_[a-zA-Z0-9]{24}`)},
		{ID: "stripe_publishable_key_test", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`pk_test_[a-zA-Z0-9]{24}`)},
		{ID: "stripe_restricted_key_live", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`rk_live_[a-zA-Z0-9]{24}`)},
		{ID: "stripe_webhook_secret", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`whsec_[a-zA-Z0-9]{32,}`)},
		{ID: "twilio_account_sid", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`AC[a-f0-9]{32}`)},
		{ID: "twilio_api_key_sid", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`SK[a-f0-9]{32}`)},
		{ID: "twilio_auth_token", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)twilio[_-]?auth[_-]?token\s*[:=]\s*['"]?[a-f0-9]{32}['"]?`)},
		{ID: "sendgrid_api_key", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`SG\.[a-zA-Z0-9_\-]{22}\.[a-zA-Z0-9_\-]{43}`)},
		{ID: "mailgun_api_key", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`key-[a-f0-9]{32}`)},
		{ID: "postmark_token", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)(?:x-postmark|postmark)[_-]?(?:server|account)[_-]?token\s*[:=]\s*['"]?[a-f0-9\-]{36}['"]?`)},
		{ID: "resend_api_key", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`re_[a-zA-Z0-9_]{32,}`)},
		{ID: "slack_token", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`xox[baprs]-[a-zA-Z0-9\-]{10,}`)},
		{ID: "slack_webhook", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`https://hooks\.slack\.com/services/`)},
		{ID: "slack_workflow_webhook", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`https://hooks\.slack\.com/workflows/`)},
		{ID: "vault_service_token", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`hvs\.[a-zA-Z0-9_\-]{90,}`)},
		{ID: "vault_batch_token", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`hvb\.[a-zA-Z0-9_\-]{90,}`)},
		{ID: "vault_recovery_token", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`hvr\.[a-zA-Z0-9_\-]{90,}`)},
		{ID: "npm_token", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`npm_[a-zA-Z0-9]{36}`)},
		{ID: "pypi_token", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`pypi-[a-zA-Z0-9]{50,}`)},
		{ID: "dockerhub_pat", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`dckr_pat_[a-zA-Z0-9_\-]{20,}`)},
		{ID: "terraform_cloud_token", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`[a-zA-Z0-9]{14}\.atlasv1\.[a-zA-Z0-9_\-]{60,}`)},
		{ID: "mongodb_srv_dsn", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`mongodb\+srv://[^:]+:[^@]+@`)},
		{ID: "mongodb_dsn", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`mongodb://[^:]+:[^@]+@`)},
		{ID: "postgres_dsn", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`postgres(?:ql)?://[^:]+:[^@]+@`)},
		{ID: "mysql_dsn", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`mysql://[^:]+:[^@]+@`)},
		{ID: "mysql2_dsn", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`mysql2://[^:]+:[^@]+@`)},
		{ID: "redis_dsn", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`redis://:[^@]+@`)},
		{ID: "rediss_dsn", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`rediss://:[^@]+@`)},
		{ID: "amqp_dsn", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`amqps?://[^:]+:[^@]+@`)},
		{ID: "elasticsearch_dsn", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`https?://[^:]+:[^@]{8,}@[^/]+:92[0-9]{2}`)},
		{ID: "clickhouse_dsn", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`clickhouse://[^:]+:[^@]+@`)},
		{ID: "rsa_private_key_pem", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`-----BEGIN\s+RSA\s+PRIVATE\s+KEY-----`)},
		{ID: "pkcs8_private_key_pem", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`-----BEGIN\s+PRIVATE\s+KEY-----`)},
		{ID: "openssh_private_key_pem", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`-----BEGIN\s+OPENSSH\s+PRIVATE\s+KEY-----`)},
		{ID: "ec_private_key_pem", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`-----BEGIN\s+EC\s+PRIVATE\s+KEY-----`)},
		{ID: "pgp_private_key_pem", Severity: SecretSeverityCritical, Pattern: regexp.MustCompile(`-----BEGIN\s+PGP\s+PRIVATE\s+KEY\s+BLOCK-----`)},
		{ID: "jwt_token", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`eyJ[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]*`)},
		{ID: "go_secret_assignment", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)(?:password|secret|token|key|api[_-]?key)\s*=\s*"[a-zA-Z0-9_\-]{8,}"`)},
		{ID: "go_struct_secret_literal", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)(?:Password|Secret(?:Key)?|APIKey|ApiKey|Token|AccessKey)\s*:\s*"[^"]{8,}"`)},
		{ID: "python_env_secret_fallback", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)os\.environ\.get\(\s*['"][^'"]*(?:secret|key|token|password)[^'"]*['"]\s*,\s*['"][^'"]{8,}['"]\s*\)`)},
		{ID: "python_env_secret_assignment", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)os\.environ\[['"][^'"]*(?:secret|key|token|password)[^'"]*['"]\]\s*=\s*['"][^'"]{8,}['"]`)},
		{ID: "django_secret_key", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)SECRET_KEY\s*=\s*['"][^'"]{20,}['"]`)},
		{ID: "flask_secret_key", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)app\.secret_key\s*=\s*['"][^'"]{8,}['"]`)},
		{ID: "python_aws_access_key_id", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)aws_access_key_id\s*=\s*['"][^'"]{16,}['"]`)},
		{ID: "python_aws_secret_access_key", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)aws_secret_access_key\s*=\s*['"][^'"]{30,}['"]`)},
		{ID: "python_sqlalchemy_dsn", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)create_engine\s*\(\s*['"][^'"]*:[^@'"].{4,}@`)},
		{ID: "js_process_env_secret_fallback", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)process\.env\.[A-Z0-9_]*(?:SECRET|KEY|TOKEN|PASSWORD|API)[A-Z0-9_]*\s*(?:\?\?|\|\|)\s*['"][^'"]{8,}['"]`)},
		{ID: "js_access_key_id", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)accessKeyId\s*:\s*['"][A-Z0-9]{16,}['"]`)},
		{ID: "js_secret_access_key", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)secretAccessKey\s*:\s*['"][a-zA-Z0-9/+]{30,}['"]`)},
		{ID: "js_authorization_header", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile(`(?i)['"]Authorization['"]\s*:\s*['"]Bearer\s+[a-zA-Z0-9_\-]{20,}['"]`)},
		{ID: "js_authorization_template_literal", Severity: SecretSeverityHigh, Pattern: regexp.MustCompile("(?i)`Authorization`\\s*:\\s*`Bearer\\s+[a-zA-Z0-9_\\-]{20,}`")},
		{ID: "js_localstorage_secret", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)localStorage\.setItem\(\s*['"][^'"]*(?:token|password|secret|key)[^'"]*['"]\s*,\s*['"][^'"]{8,}['"]`)},
		{ID: "js_object_secret_literal", Severity: SecretSeverityMedium, Pattern: regexp.MustCompile(`(?i)(?:apiKey|api_key|authToken|auth_token|clientSecret|client_secret)\s*:\s*['"][^'"]{16,}['"]`)},
		{ID: "hex_secret_64", Severity: SecretSeverityLow, Pattern: regexp.MustCompile(`\b[a-f0-9]{64}\b`)},
		{ID: "hex_secret_32", Severity: SecretSeverityLow, Pattern: regexp.MustCompile(`\b[a-f0-9]{32}\b`)},
	}
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

// SecretSeverityRank returns a numeric rank for severity levels.
func SecretSeverityRank(severity string) int {
	switch strings.ToUpper(severity) {
	case SecretSeverityCritical:
		return 4
	case SecretSeverityHigh:
		return 3
	case SecretSeverityMedium:
		return 2
	case SecretSeverityLow:
		return 1
	case "OFF", "DISABLED":
		return -1
	default:
		return 0
	}
}

// PickBestSecretFinding selects the most severe finding from a list.
func PickBestSecretFinding(findings []types.Violation) types.Violation {
	if len(findings) == 0 {
		return types.Violation{}
	}

	filtered := filterSpecificSecretFindings(findings)

	if len(filtered) == 0 {
		return types.Violation{}
	}

	best := filtered[0]
	for i := 1; i < len(filtered); i++ {
		if SecretSeverityRank(filtered[i].Severity) > SecretSeverityRank(best.Severity) {
			best = filtered[i]
		}
	}
	return best
}

func filterSpecificSecretFindings(findings []types.Violation) []types.Violation {
	hasSpecific := false
	for _, f := range findings {
		if _, ok := genericSecretPatternIDs[f.RuleID]; !ok && f.RuleID != "secret-keyword" {
			hasSpecific = true
			break
		}
	}

	if !hasSpecific {
		return findings
	}

	var filtered []types.Violation
	for _, f := range findings {
		if _, ok := genericSecretPatternIDs[f.RuleID]; ok || f.RuleID == "secret-keyword" {
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered
}

// IsAllowedLiteral checks if a literal line matches any user-defined allowed pattern.
func IsAllowedLiteral(content string, allowed []*regexp.Regexp) bool {
	for _, p := range allowed {
		if p.MatchString(content) {
			return true
		}
	}
	return false
}

// IsBenignSecretExample checks if a literal contains hints that it is for testing/documentation.
func IsBenignSecretExample(content string, hints []string) bool {
	contentLower := strings.ToLower(content)
	for _, hint := range hints {
		if strings.Contains(contentLower, strings.ToLower(hint)) {
			return true
		}
	}
	return false
}

// IsObviousPlaceholderSecret checks if a literal is a known placeholder string.
func IsObviousPlaceholderSecret(content string, placeholders []string) bool {
	contentLower := strings.ToLower(content)
	for _, p := range placeholders {
		if strings.Contains(contentLower, strings.ToLower(p)) {
			return true
		}
	}
	for _, pattern := range obviousPlaceholderSecretPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// FilterAllowlistedSecretFindings removes findings that match allowlisted patterns or have been disabled via overrides.
func FilterAllowlistedSecretFindings(findings []types.Violation, cfg config.PolicySecretLoggingConfig) []types.Violation {
	var filtered []types.Violation

	allowlistedIDs := make(map[string]struct{})
	for _, id := range cfg.Allowlist.PatternIDs {
		allowlistedIDs[id] = struct{}{}
	}

	for _, f := range findings {
		// 1. Check pattern-ID allowlist
		if _, ok := allowlistedIDs[f.RuleID]; ok {
			continue
		}

		// 2. Check severity override
		if sev, ok := cfg.Overrides[f.RuleID]; ok {
			upperSev := strings.ToUpper(sev)
			if upperSev == "OFF" || upperSev == "DISABLED" {
				continue
			}
			f.Severity = upperSev
		}

		filtered = append(filtered, f)
	}

	return filtered
}
