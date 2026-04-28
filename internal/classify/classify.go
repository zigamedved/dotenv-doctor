// Package classify identifies likely secret values by pattern.
//
// We deliberately keep the rule set small and high-confidence: dotenv-doctor
// is a developer dashboard, not gitleaks. False positives erode trust faster
// than missed long-tail patterns hurt us. Each rule has a stable ID so we can
// surface "this looks like an AWS secret access key" in the UI.
package classify

import "regexp"

// Severity ranks how worried we should be about a finding.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
)

func (s Severity) String() string {
	switch s {
	case SeverityHigh:
		return "high"
	case SeverityMedium:
		return "medium"
	case SeverityLow:
		return "low"
	default:
		return "info"
	}
}

// Rule is a curated secret-pattern matcher.
type Rule struct {
	ID          string
	Name        string
	Description string
	Severity    Severity
	pattern     *regexp.Regexp
	// keyHint, if set, requires the env key to contain this substring (case-insensitive).
	// Used to tighten patterns that are otherwise prone to false positives.
	keyHint string
}

// Match reports whether v looks like an instance of this rule.
func (r Rule) Match(key, v string) bool {
	if r.keyHint != "" {
		if !containsFold(key, r.keyHint) {
			return false
		}
	}
	return r.pattern.MatchString(v)
}

// Finding is a single classifier hit on a value.
type Finding struct {
	Rule  Rule
	Match string
}

// Rules returns the curated rule set. Order matters only for tie-breaking
// in tests; runtime callers should treat it as unordered.
func Rules() []Rule {
	return rules
}

// ClassifyValue returns all rules that match a single value. Most values
// will return zero or one finding; we return a slice for the rare case
// where a value contains multiple distinct kinds of secrets.
func ClassifyValue(key, v string) []Finding {
	if v == "" {
		return nil
	}
	var out []Finding
	for _, r := range rules {
		if r.Match(key, v) {
			out = append(out, Finding{Rule: r, Match: r.pattern.FindString(v)})
		}
	}
	return out
}

// ClassifyText scans an arbitrary blob of text (e.g. a .env file pulled from
// git history). Used by the leaks scanner.
func ClassifyText(text string) []Finding {
	var out []Finding
	for _, r := range rules {
		// Skip key-hinted rules in raw-text mode; we don't have a key.
		if r.keyHint != "" {
			continue
		}
		matches := r.pattern.FindAllString(text, -1)
		for _, m := range matches {
			out = append(out, Finding{Rule: r, Match: m})
		}
	}
	return out
}

func mustCompile(p string) *regexp.Regexp { return regexp.MustCompile(p) }

// rules is the curated set. Keep this conservative; gitleaks does the long tail.
var rules = []Rule{
	{
		ID:          "aws-access-key-id",
		Name:        "AWS Access Key ID",
		Description: "20-char AWS access key (AKIA/ASIA prefix).",
		Severity:    SeverityHigh,
		pattern:     mustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),
	},
	{
		ID:          "aws-secret-access-key",
		Name:        "AWS Secret Access Key",
		Description: "40-char AWS secret access key.",
		Severity:    SeverityHigh,
		pattern:     mustCompile(`\b[A-Za-z0-9/+=]{40}\b`),
		keyHint:     "secret",
	},
	{
		ID:          "github-pat",
		Name:        "GitHub Personal Access Token",
		Description: "Classic or fine-grained GitHub PAT.",
		Severity:    SeverityHigh,
		pattern:     mustCompile(`\bghp_[A-Za-z0-9]{36}\b`),
	},
	{
		ID:          "github-oauth",
		Name:        "GitHub OAuth Token",
		Severity:    SeverityHigh,
		pattern:     mustCompile(`\bgho_[A-Za-z0-9]{36}\b`),
	},
	{
		ID:       "github-server-token",
		Name:     "GitHub Server Token",
		Severity: SeverityHigh,
		pattern:  mustCompile(`\bghs_[A-Za-z0-9]{36}\b`),
	},
	{
		ID:       "github-refresh-token",
		Name:     "GitHub Refresh Token",
		Severity: SeverityHigh,
		pattern:  mustCompile(`\bghr_[A-Za-z0-9]{36}\b`),
	},
	{
		ID:       "github-fine-grained-pat",
		Name:     "GitHub Fine-Grained PAT",
		Severity: SeverityHigh,
		pattern:  mustCompile(`\bgithub_pat_[A-Za-z0-9_]{82}\b`),
	},
	{
		ID:          "stripe-live-secret",
		Name:        "Stripe Live Secret Key",
		Description: "Live Stripe secret key — production credentials.",
		Severity:    SeverityHigh,
		pattern:     mustCompile(`\bsk_live_[A-Za-z0-9]{20,}\b`),
	},
	{
		ID:       "stripe-test-secret",
		Name:     "Stripe Test Secret Key",
		Severity: SeverityMedium,
		pattern:  mustCompile(`\bsk_test_[A-Za-z0-9]{20,}\b`),
	},
	{
		ID:       "stripe-restricted",
		Name:     "Stripe Restricted Key",
		Severity: SeverityHigh,
		pattern:  mustCompile(`\brk_live_[A-Za-z0-9]{20,}\b`),
	},
	{
		ID:          "openai-api-key",
		Name:        "OpenAI API Key",
		Description: "OpenAI / ChatGPT-compatible API key.",
		Severity:    SeverityHigh,
		pattern:     mustCompile(`\bsk-(?:proj-)?[A-Za-z0-9_\-]{20,}\b`),
	},
	{
		ID:       "anthropic-api-key",
		Name:     "Anthropic API Key",
		Severity: SeverityHigh,
		pattern:  mustCompile(`\bsk-ant-(?:api03-)?[A-Za-z0-9_\-]{50,}\b`),
	},
	{
		ID:       "google-api-key",
		Name:     "Google API Key",
		Severity: SeverityHigh,
		pattern:  mustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`),
	},
	{
		ID:       "slack-token",
		Name:     "Slack Token",
		Severity: SeverityHigh,
		pattern:  mustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`),
	},
	{
		ID:       "slack-webhook",
		Name:     "Slack Webhook",
		Severity: SeverityMedium,
		pattern:  mustCompile(`https://hooks\.slack\.com/services/T[A-Z0-9]+/B[A-Z0-9]+/[A-Za-z0-9]+`),
	},
	{
		ID:       "discord-webhook",
		Name:     "Discord Webhook",
		Severity: SeverityMedium,
		pattern:  mustCompile(`https://(?:ptb\.|canary\.)?discord(?:app)?\.com/api/webhooks/[0-9]+/[A-Za-z0-9_\-]+`),
	},
	{
		ID:       "twilio-account-sid",
		Name:     "Twilio Account SID",
		Severity: SeverityMedium,
		pattern:  mustCompile(`\bAC[a-f0-9]{32}\b`),
	},
	{
		ID:          "private-key-header",
		Name:        "Private Key (PEM Header)",
		Description: "PEM-encoded private key — should never be in a .env file.",
		Severity:    SeverityHigh,
		pattern:     mustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA |ENCRYPTED |PGP |)PRIVATE KEY-----`),
	},
	{
		ID:          "jwt",
		Name:        "JSON Web Token",
		Description: "Looks like a JWT. Could be a session token; treat as sensitive.",
		Severity:    SeverityLow,
		pattern:     mustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b`),
	},
	{
		ID:          "generic-bearer",
		Name:        "Bearer Token",
		Description: "Long opaque token in a value whose key contains TOKEN/SECRET/KEY/PASSWORD.",
		Severity:    SeverityLow,
		pattern:     mustCompile(`^[A-Za-z0-9_\-+/=]{32,}$`),
		keyHint:     "token",
	},
}

func containsFold(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	// Avoid pulling strings.ContainsFold-equivalent dependency; small inline impl.
	sb := []byte(toLower(s))
	tb := []byte(toLower(sub))
	for i := 0; i+len(tb) <= len(sb); i++ {
		match := true
		for j := 0; j < len(tb); j++ {
			if sb[i+j] != tb[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
