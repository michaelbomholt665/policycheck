
```go
// cmd/policycheck/secret_patterns.go
// Defines regular expression patterns for identifying common secrets and credentials.

package main

import (
	"regexp"
	"strings"
)

const (
	secretSeverityLow      = "LOW"
	secretSeverityMedium   = "MEDIUM"
	secretSeverityHigh     = "HIGH"
	secretSeverityCritical = "CRITICAL"
)

type secretPattern struct {
	id       string
	severity string
	pattern  *regexp.Regexp
}

type secretFinding struct {
	patternID string
	severity  string
}

type secretPatternCatalog struct {
	patterns               []secretPattern
	allowedLiteralPatterns []*regexp.Regexp
	allowlistedPatternIDs  map[string]struct{}
}

var builtInSecretPatterns = []secretPattern{
	{id: "generic_api_key_assignment", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9_\-]{20,}['"]?`)},
	{id: "generic_password_assignment", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)password\s*[:=]\s*['"]?[^\s'"]{8,}['"]?`)},
	{id: "generic_secret_assignment", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)secret\s*[:=]\s*['"]?[a-zA-Z0-9_\-]{20,}['"]?`)},
	{id: "generic_token_assignment", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)token\s*[:=]\s*['"]?[a-zA-Z0-9_\-]{20,}['"]?`)},
	{id: "standalone_password_candidate", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)\b(?:pass|pwd|secret)[0-9a-zA-Z_\-]{4,}\b`)},
	{id: "authorization_bearer_header", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)Authorization:\s*Bearer`)},
	{id: "bearer_literal", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)Bearer\s+[a-zA-Z0-9_\-]{20,}`)},
	{id: "aws_access_key_id", severity: secretSeverityHigh, pattern: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{id: "aws_session_access_key_id", severity: secretSeverityHigh, pattern: regexp.MustCompile(`ASIA[0-9A-Z]{16}`)},
	{id: "aws_secret_access_key_label", severity: secretSeverityLow, pattern: regexp.MustCompile(`(?i)(?:aws)?[_-]?secret[_-]?access[_-]?key`)},
	{id: "aws_secret_access_key_assignment", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)aws[_-]?secret[_-]?access[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9/+]{40}['"]?`)},
	{id: "aws_session_token_assignment", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)aws[_-]?session[_-]?token\s*[:=]\s*['"]?[a-zA-Z0-9/+=]{100,}['"]?`)},
	{id: "gcp_api_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`AIza[a-zA-Z0-9_\-]{35}`)},
	{id: "google_oauth_client_secret", severity: secretSeverityHigh, pattern: regexp.MustCompile(`GOCSPX-[a-zA-Z0-9_\-]{28}`)},
	{id: "gcp_service_account_type", severity: secretSeverityLow, pattern: regexp.MustCompile(`"type"\s*:\s*"service_account"`)},
	{id: "gcp_private_key_id", severity: secretSeverityHigh, pattern: regexp.MustCompile(`"private_key_id"\s*:\s*"[a-f0-9]{40}"`)},
	{id: "azure_storage_connection_string", severity: secretSeverityHigh, pattern: regexp.MustCompile(`DefaultEndpointsProtocol=https;AccountName=[^;]+;AccountKey=[a-zA-Z0-9/+=]{88}`)},
	{id: "azure_storage_account_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`AccountKey=[a-zA-Z0-9/+=]{88}`)},
	{id: "azure_sas_sig", severity: secretSeverityMedium, pattern: regexp.MustCompile(`[?&]sig=[a-zA-Z0-9%/+=]{43,}`)},
	{id: "azure_client_secret", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)azure[_-]?client[_-]?secret\s*[:=]\s*['"]?[a-zA-Z0-9_\-~.]{32,}['"]?`)},
	{id: "azure_subscription_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)(?:ocp|subscription)[_-]?apim[_-]?(?:key|subscription[_-]?key)\s*[:=]\s*['"]?[a-f0-9]{32}['"]?`)},
	{id: "github_pat_classic", severity: secretSeverityCritical, pattern: regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`)},
	{id: "github_oauth_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`)},
	{id: "github_user_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`ghu_[a-zA-Z0-9]{36}`)},
	{id: "github_refresh_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`ghr_[a-zA-Z0-9]{36}`)},
	{id: "github_pat_fine_grained", severity: secretSeverityCritical, pattern: regexp.MustCompile(`github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}`)},
	{id: "github_actions_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`ghs_[a-zA-Z0-9]{36}`)},
	{id: "gitlab_pat", severity: secretSeverityCritical, pattern: regexp.MustCompile(`glpat-[a-zA-Z0-9\-_]{20,}`)},
	{id: "gitlab_trigger_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`glptt-[a-f0-9]{40}`)},
	{id: "gitlab_runner_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`GR1348941[a-zA-Z0-9\-_]{20}`)},
	{id: "openai_legacy_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`sk-[a-zA-Z0-9]{48}`)},
	{id: "openai_project_key", severity: secretSeverityCritical, pattern: regexp.MustCompile(`sk-proj-[a-zA-Z0-9\-_]{48,}`)},
	{id: "anthropic_api_key", severity: secretSeverityCritical, pattern: regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-_]{95}`)},
	{id: "stripe_secret_key_live", severity: secretSeverityCritical, pattern: regexp.MustCompile(`sk_live_[a-zA-Z0-9]{24}`)},
	{id: "stripe_secret_key_test", severity: secretSeverityHigh, pattern: regexp.MustCompile(`sk_test_[a-zA-Z0-9]{24}`)},
	{id: "stripe_publishable_key_live", severity: secretSeverityMedium, pattern: regexp.MustCompile(`pk_live_[a-zA-Z0-9]{24}`)},
	{id: "stripe_publishable_key_test", severity: secretSeverityMedium, pattern: regexp.MustCompile(`pk_test_[a-zA-Z0-9]{24}`)},
	{id: "stripe_restricted_key_live", severity: secretSeverityHigh, pattern: regexp.MustCompile(`rk_live_[a-zA-Z0-9]{24}`)},
	{id: "stripe_webhook_secret", severity: secretSeverityHigh, pattern: regexp.MustCompile(`whsec_[a-zA-Z0-9]{32,}`)},
	{id: "twilio_account_sid", severity: secretSeverityMedium, pattern: regexp.MustCompile(`AC[a-f0-9]{32}`)},
	{id: "twilio_api_key_sid", severity: secretSeverityMedium, pattern: regexp.MustCompile(`SK[a-f0-9]{32}`)},
	{id: "twilio_auth_token", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)twilio[_-]?auth[_-]?token\s*[:=]\s*['"]?[a-f0-9]{32}['"]?`)},
	{id: "sendgrid_api_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`SG\.[a-zA-Z0-9_\-]{22}\.[a-zA-Z0-9_\-]{43}`)},
	{id: "mailgun_api_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`key-[a-f0-9]{32}`)},
	{id: "postmark_token", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)(?:x-postmark|postmark)[_-]?(?:server|account)[_-]?token\s*[:=]\s*['"]?[a-f0-9\-]{36}['"]?`)},
	{id: "resend_api_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`re_[a-zA-Z0-9_]{32,}`)},
	{id: "slack_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`xox[baprs]-[a-zA-Z0-9\-]{10,}`)},
	{id: "slack_webhook", severity: secretSeverityCritical, pattern: regexp.MustCompile(`https://hooks\.slack\.com/services/`)},
	{id: "slack_workflow_webhook", severity: secretSeverityCritical, pattern: regexp.MustCompile(`https://hooks\.slack\.com/workflows/`)},
	{id: "vault_service_token", severity: secretSeverityHigh, pattern: regexp.MustCompile(`hvs\.[a-zA-Z0-9_\-]{90,}`)},
	{id: "vault_batch_token", severity: secretSeverityHigh, pattern: regexp.MustCompile(`hvb\.[a-zA-Z0-9_\-]{90,}`)},
	{id: "vault_recovery_token", severity: secretSeverityHigh, pattern: regexp.MustCompile(`hvr\.[a-zA-Z0-9_\-]{90,}`)},
	{id: "npm_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`npm_[a-zA-Z0-9]{36}`)},
	{id: "pypi_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`pypi-[a-zA-Z0-9]{50,}`)},
	{id: "dockerhub_pat", severity: secretSeverityCritical, pattern: regexp.MustCompile(`dckr_pat_[a-zA-Z0-9_\-]{20,}`)},
	{id: "terraform_cloud_token", severity: secretSeverityCritical, pattern: regexp.MustCompile(`[a-zA-Z0-9]{14}\.atlasv1\.[a-zA-Z0-9_\-]{60,}`)},
	{id: "mongodb_srv_dsn", severity: secretSeverityHigh, pattern: regexp.MustCompile(`mongodb\+srv://[^:]+:[^@]+@`)},
	{id: "mongodb_dsn", severity: secretSeverityHigh, pattern: regexp.MustCompile(`mongodb://[^:]+:[^@]+@`)},
	{id: "postgres_dsn", severity: secretSeverityHigh, pattern: regexp.MustCompile(`postgres(?:ql)?://[^:]+:[^@]+@`)},
	{id: "mysql_dsn", severity: secretSeverityHigh, pattern: regexp.MustCompile(`mysql://[^:]+:[^@]+@`)},
	{id: "mysql2_dsn", severity: secretSeverityHigh, pattern: regexp.MustCompile(`mysql2://[^:]+:[^@]+@`)},
	{id: "redis_dsn", severity: secretSeverityHigh, pattern: regexp.MustCompile(`redis://:[^@]+@`)},
	{id: "rediss_dsn", severity: secretSeverityHigh, pattern: regexp.MustCompile(`rediss://:[^@]+@`)},
	{id: "amqp_dsn", severity: secretSeverityHigh, pattern: regexp.MustCompile(`amqps?://[^:]+:[^@]+@`)},
	{id: "elasticsearch_dsn", severity: secretSeverityHigh, pattern: regexp.MustCompile(`https?://[^:]+:[^@]{8,}@[^/]+:92[0-9]{2}`)},
	{id: "clickhouse_dsn", severity: secretSeverityHigh, pattern: regexp.MustCompile(`clickhouse://[^:]+:[^@]+@`)},
	{id: "rsa_private_key_pem", severity: secretSeverityCritical, pattern: regexp.MustCompile(`-----BEGIN\s+RSA\s+PRIVATE\s+KEY-----`)},
	{id: "pkcs8_private_key_pem", severity: secretSeverityCritical, pattern: regexp.MustCompile(`-----BEGIN\s+PRIVATE\s+KEY-----`)},
	{id: "openssh_private_key_pem", severity: secretSeverityCritical, pattern: regexp.MustCompile(`-----BEGIN\s+OPENSSH\s+PRIVATE\s+KEY-----`)},
	{id: "ec_private_key_pem", severity: secretSeverityCritical, pattern: regexp.MustCompile(`-----BEGIN\s+EC\s+PRIVATE\s+KEY-----`)},
	{id: "pgp_private_key_pem", severity: secretSeverityCritical, pattern: regexp.MustCompile(`-----BEGIN\s+PGP\s+PRIVATE\s+KEY\s+BLOCK-----`)},
	{id: "jwt_token", severity: secretSeverityMedium, pattern: regexp.MustCompile(`eyJ[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]*`)},
	{id: "go_secret_assignment", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)(?:password|secret|token|key|api[_-]?key)\s*=\s*"[a-zA-Z0-9_\-]{8,}"`)},
	{id: "go_struct_secret_literal", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)(?:Password|Secret(?:Key)?|APIKey|ApiKey|Token|AccessKey)\s*:\s*"[^"]{8,}"`)},
	{id: "python_env_secret_fallback", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)os\.environ\.get\(\s*['"][^'"]*(?:secret|key|token|password)[^'"]*['"]\s*,\s*['"][^'"]{8,}['"]\s*\)`)},
	{id: "python_env_secret_assignment", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)os\.environ\[['"][^'"]*(?:secret|key|token|password)[^'"]*['"]\]\s*=\s*['"][^'"]{8,}['"]`)},
	{id: "django_secret_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)SECRET_KEY\s*=\s*['"][^'"]{20,}['"]`)},
	{id: "flask_secret_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)app\.secret_key\s*=\s*['"][^'"]{8,}['"]`)},
	{id: "python_aws_access_key_id", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)aws_access_key_id\s*=\s*['"][^'"]{16,}['"]`)},
	{id: "python_aws_secret_access_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)aws_secret_access_key\s*=\s*['"][^'"]{30,}['"]`)},
	{id: "python_sqlalchemy_dsn", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)create_engine\s*\(\s*['"](?:postgresql|mysql|sqlite)[^'"]*:[^@'"]{4,}@`)},
	{id: "js_process_env_secret_fallback", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)process\.env\.[A-Z0-9_]*(?:SECRET|KEY|TOKEN|PASSWORD|API)[A-Z0-9_]*\s*(?:\?\?|\|\|)\s*['"][^'"]{8,}['"]`)},
	{id: "js_access_key_id", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)accessKeyId\s*:\s*['"][A-Z0-9]{16,}['"]`)},
	{id: "js_secret_access_key", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)secretAccessKey\s*:\s*['"][a-zA-Z0-9/+]{30,}['"]`)},
	{id: "js_authorization_header", severity: secretSeverityHigh, pattern: regexp.MustCompile(`(?i)['"]Authorization['"]\s*:\s*['"]Bearer\s+[a-zA-Z0-9_\-]{20,}['"]`)},
	{id: "js_authorization_template_literal", severity: secretSeverityHigh, pattern: regexp.MustCompile("(?i)`Authorization`\\s*:\\s*`Bearer\\s+[a-zA-Z0-9_\\-]{20,}`")},
	{id: "js_localstorage_secret", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)localStorage\.setItem\(\s*['"][^'"]*(?:token|password|secret|key)[^'"]*['"]\s*,\s*['"][^'"]{8,}['"]`)},
	{id: "js_object_secret_literal", severity: secretSeverityMedium, pattern: regexp.MustCompile(`(?i)(?:apiKey|api_key|authToken|auth_token|clientSecret|client_secret)\s*:\s*['"][^'"]{16,}['"]`)},
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

func buildSecretPatternCatalog(cfg policyConfig) secretPatternCatalog {
	overrides := cfg.SecretLogging.Overrides
	patterns := make([]secretPattern, 0, len(builtInSecretPatterns))
	for _, item := range builtInSecretPatterns {
		severity := item.severity
		if override, ok := overrides[item.id]; ok {
			normalized := strings.ToUpper(strings.TrimSpace(override))
			switch normalized {
			case "":
			case "DISABLED", "OFF":
				continue
			case secretSeverityLow, secretSeverityMedium, secretSeverityHigh, secretSeverityCritical:
				severity = normalized
			}
		}
		patterns = append(patterns, secretPattern{
			id:       item.id,
			severity: severity,
			pattern:  item.pattern,
		})
	}

	allowlistedPatternIDs := make(map[string]struct{}, len(cfg.SecretLogging.Allowlist.PatternIDs))
	for _, id := range cfg.SecretLogging.Allowlist.PatternIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		allowlistedPatternIDs[trimmed] = struct{}{}
	}

	return secretPatternCatalog{
		patterns:               patterns,
		allowedLiteralPatterns: cfg.SecretLogging.CompiledAllowedLiteralPatterns,
		allowlistedPatternIDs:  allowlistedPatternIDs,
	}
}

func evaluateSecretLiteral(raw string, cfg policyConfig, catalog secretPatternCatalog) (secretFinding, bool) {
	lower := strings.ToLower(raw)
	if isAllowedSecretLogText(lower) || matchesSecretAllowPattern(raw, catalog.allowedLiteralPatterns) {
		return secretFinding{}, false
	}
	if isBenignSecretExample(lower) || isObviousPlaceholderSecret(raw) {
		return secretFinding{}, false
	}

	allMatches := make([]secretFinding, 0, 4)
	for _, item := range catalog.patterns {
		if item.pattern.MatchString(raw) {
			allMatches = append(allMatches, secretFinding{
				patternID: item.id,
				severity:  item.severity,
			})
		}
	}
	matches := filterAllowlistedSecretFindings(allMatches, catalog.allowlistedPatternIDs)
	if finding, ok := pickBestSecretFinding(matches); ok {
		return finding, true
	}
	if len(allMatches) > 0 {
		return secretFinding{}, false
	}

	if hasSecretKeyword(lower, cfg.SecretLogging.Keywords) {
		return secretFinding{
			patternID: "keyword_match",
			severity:  secretSeverityLow,
		}, true
	}

	return secretFinding{}, false
}

func pickBestSecretFinding(matches []secretFinding) (secretFinding, bool) {
	if len(matches) == 0 {
		return secretFinding{}, false
	}

	filtered := matches
	if hasSpecificSecretFinding(matches) {
		filtered = make([]secretFinding, 0, len(matches))
		for _, match := range matches {
			if isGenericSecretPattern(match.patternID) {
				continue
			}
			filtered = append(filtered, match)
		}
	}

	best := filtered[0]
	bestRank := secretSeverityRank(best.severity)
	for _, match := range filtered[1:] {
		matchRank := secretSeverityRank(match.severity)
		if matchRank > bestRank {
			best = match
			bestRank = matchRank
		}
	}

	return best, true
}

func filterAllowlistedSecretFindings(matches []secretFinding, allowlistedPatternIDs map[string]struct{}) []secretFinding {
	filtered := matches
	if hasSpecificSecretFinding(matches) {
		filtered = make([]secretFinding, 0, len(matches))
		for _, match := range matches {
			if isGenericSecretPattern(match.patternID) {
				continue
			}
			filtered = append(filtered, match)
		}
	}

	result := make([]secretFinding, 0, len(filtered))
	for _, match := range filtered {
		if _, ok := allowlistedPatternIDs[match.patternID]; ok {
			continue
		}
		result = append(result, match)
	}

	return result
}

func hasSpecificSecretFinding(matches []secretFinding) bool {
	for _, match := range matches {
		if !isGenericSecretPattern(match.patternID) {
			return true
		}
	}

	return false
}

func isGenericSecretPattern(patternID string) bool {
	_, ok := genericSecretPatternIDs[patternID]
	return ok
}

func secretSeverityRank(severity string) int {
	switch severity {
	case secretSeverityCritical:
		return 4
	case secretSeverityHigh:
		return 3
	case secretSeverityMedium:
		return 2
	case secretSeverityLow:
		return 1
	default:
		return 0
	}
}

func isKnownLoggerIdentifier(name string, cfg policyConfig) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if _, ok := knownLoggerIdentifiers[lower]; ok {
		return true
	}
	for _, ident := range cfg.SecretLogging.LoggerIdentifiers {
		if strings.ToLower(ident) == lower {
			return true
		}
	}
	return false
}

func isBenignSecretExample(lower string) bool {
	for _, hint := range []string{
		"example",
		"sample",
		"placeholder",
		"dummy",
		"fake",
		"fixture",
		"redacted",
		"masked",
		"your-",
		"your_",
		"test-",
		"test_",
		"my-",
		"my_",
	} {
		if strings.Contains(lower, hint) {
			return true
		}
	}

	return false
}

func isObviousPlaceholderSecret(raw string) bool {
	lower := strings.ToLower(raw)
	for _, placeholder := range []string{
		"<token>",
		"<password>",
		"<secret>",
		"<api-key>",
		"<access-key>",
		"your_token_here",
		"your-api-key",
		"your_access_key",
		"replace_me",
		"changeme",
		"change_me",
		"example-token",
		"example_password",
		"insert_key_here",
	} {
		if strings.Contains(lower, placeholder) {
			return true
		}
	}

	for _, pattern := range obviousPlaceholderSecretPatterns {
		if pattern.MatchString(raw) {
			return true
		}
	}
	return false
}
```