package classify

import (
	"strings"
	"testing"
)

func TestAWSAccessKey(t *testing.T) {
	hits := ClassifyValue("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	if !hasRule(hits, "aws-access-key-id") {
		t.Errorf("expected aws-access-key-id, got %+v", hits)
	}
}

func TestAWSSecretRequiresKeyHint(t *testing.T) {
	// 40-char base64-ish, but the key isn't named secret-y, so the rule
	// should not fire.
	hits := ClassifyValue("DESCRIPTION", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	if hasRule(hits, "aws-secret-access-key") {
		t.Errorf("aws-secret rule should require key hint")
	}

	hits = ClassifyValue("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	if !hasRule(hits, "aws-secret-access-key") {
		t.Errorf("aws-secret rule should fire with key hint, got %+v", hits)
	}
}

func TestGitHubPAT(t *testing.T) {
	hits := ClassifyValue("GITHUB_TOKEN", "ghp_"+strings.Repeat("a", 36))
	if !hasRule(hits, "github-pat") {
		t.Errorf("expected github-pat, got %+v", hits)
	}
}

func TestStripeLive(t *testing.T) {
	hits := ClassifyValue("STRIPE_KEY", "sk_live_abcdefghij1234567890abcdef")
	if !hasRule(hits, "stripe-live-secret") {
		t.Errorf("expected stripe-live-secret, got %+v", hits)
	}
}

func TestStripeTestSeparate(t *testing.T) {
	hits := ClassifyValue("STRIPE_KEY", "sk_test_abcdefghij1234567890abcdef")
	if !hasRule(hits, "stripe-test-secret") {
		t.Errorf("expected stripe-test-secret, got %+v", hits)
	}
	if hasRule(hits, "stripe-live-secret") {
		t.Errorf("test key should not match live rule")
	}
}

func TestOpenAIKey(t *testing.T) {
	hits := ClassifyValue("OPENAI_API_KEY", "sk-proj-AbCdEfGhIjKlMnOpQrStUvWxYz0123456789")
	if !hasRule(hits, "openai-api-key") {
		t.Errorf("expected openai-api-key, got %+v", hits)
	}
}

func TestSlackToken(t *testing.T) {
	hits := ClassifyValue("SLACK_TOKEN", "xoxb-1234567890123-abcdef-abcdef")
	if !hasRule(hits, "slack-token") {
		t.Errorf("expected slack-token, got %+v", hits)
	}
}

func TestSlackWebhook(t *testing.T) {
	hits := ClassifyValue("WEBHOOK", "https://hooks.slack.com/services/TABCDEFG/B1234567/abcdefghij1234")
	if !hasRule(hits, "slack-webhook") {
		t.Errorf("expected slack-webhook, got %+v", hits)
	}
}

func TestPrivateKeyHeader(t *testing.T) {
	hits := ClassifyValue("KEY", "-----BEGIN RSA PRIVATE KEY-----")
	if !hasRule(hits, "private-key-header") {
		t.Errorf("expected private-key-header, got %+v", hits)
	}
}

func TestJWT(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	hits := ClassifyValue("TOKEN", jwt)
	if !hasRule(hits, "jwt") {
		t.Errorf("expected jwt, got %+v", hits)
	}
}

func TestEmptyValueNoMatch(t *testing.T) {
	if hits := ClassifyValue("KEY", ""); len(hits) != 0 {
		t.Errorf("expected no hits, got %+v", hits)
	}
}

func TestNonSecretValueNoMatch(t *testing.T) {
	if hits := ClassifyValue("PORT", "3000"); len(hits) != 0 {
		t.Errorf("expected no hits, got %+v", hits)
	}
	if hits := ClassifyValue("DEBUG", "true"); len(hits) != 0 {
		t.Errorf("expected no hits, got %+v", hits)
	}
}

func TestClassifyTextSkipsKeyHinted(t *testing.T) {
	// AWS secret key pattern requires keyHint; ClassifyText should NOT
	// emit it (no key context).
	out := ClassifyText("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	for _, f := range out {
		if f.Rule.ID == "aws-secret-access-key" {
			t.Errorf("ClassifyText should skip keyHint-only rules")
		}
	}
}

func TestClassifyTextFindsAWSAccessKey(t *testing.T) {
	out := ClassifyText("some prefix AKIAIOSFODNN7EXAMPLE some suffix")
	if len(out) == 0 {
		t.Errorf("expected at least one finding")
	}
}

func TestClassifyTextMultipleFindings(t *testing.T) {
	text := "AKIAIOSFODNN7EXAMPLE\nghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	out := ClassifyText(text)
	if len(out) < 2 {
		t.Errorf("expected at least 2 findings, got %d", len(out))
	}
}

func TestSeverityString(t *testing.T) {
	if SeverityHigh.String() != "high" {
		t.Errorf("got %q", SeverityHigh.String())
	}
	if SeverityInfo.String() != "info" {
		t.Errorf("got %q", SeverityInfo.String())
	}
}

func TestRulesNotEmpty(t *testing.T) {
	if len(Rules()) < 15 {
		t.Errorf("rule set unexpectedly small: %d", len(Rules()))
	}
}

func hasRule(hits []Finding, id string) bool {
	for _, h := range hits {
		if h.Rule.ID == id {
			return true
		}
	}
	return false
}
