package framework

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectNextJS(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "package.json"), `{"dependencies":{"next":"14.0.0"}}`)
	if got := Detect(dir); got != NextJS {
		t.Errorf("got %q, want %q", got, NextJS)
	}
}

func TestDetectVite(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "package.json"), `{"devDependencies":{"vite":"5.0.0"}}`)
	if got := Detect(dir); got != Vite {
		t.Errorf("got %q, want %q", got, Vite)
	}
}

func TestDetectCRA(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "package.json"), `{"dependencies":{"react-scripts":"5.0.0"}}`)
	if got := Detect(dir); got != CRA {
		t.Errorf("got %q, want %q", got, CRA)
	}
}

func TestDetectExpress(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "package.json"), `{"dependencies":{"express":"4.0.0"}}`)
	if got := Detect(dir); got != Express {
		t.Errorf("got %q, want %q", got, Express)
	}
}

func TestDetectDjangoFromManagePy(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "manage.py"), "#!/usr/bin/env python\n")
	if got := Detect(dir); got != Django {
		t.Errorf("got %q, want %q", got, Django)
	}
}

func TestDetectFastAPI(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "requirements.txt"), "fastapi==0.110.0\n")
	if got := Detect(dir); got != FastAPI {
		t.Errorf("got %q, want %q", got, FastAPI)
	}
}

func TestDetectUnknown(t *testing.T) {
	dir := t.TempDir()
	if got := Detect(dir); got != Unknown {
		t.Errorf("got %q, want unknown", got)
	}
}

func TestPublicPrefixes(t *testing.T) {
	if !contains(PublicPrefixes(NextJS), "NEXT_PUBLIC_") {
		t.Errorf("Next.js should have NEXT_PUBLIC_")
	}
	if !contains(PublicPrefixes(Vite), "VITE_") {
		t.Errorf("Vite should have VITE_")
	}
	if !contains(PublicPrefixes(CRA), "REACT_APP_") {
		t.Errorf("CRA should have REACT_APP_")
	}
	if len(PublicPrefixes(Django)) != 0 {
		t.Errorf("Django shouldn't have public prefixes")
	}
}

func TestIsPublic(t *testing.T) {
	if !IsPublic(NextJS, "NEXT_PUBLIC_API_URL") {
		t.Errorf("NEXT_PUBLIC_ should be public for Next.js")
	}
	if IsPublic(NextJS, "API_KEY") {
		t.Errorf("API_KEY without prefix should not be public")
	}
	if IsPublic(Express, "ANYTHING") {
		t.Errorf("Express has no public prefixes")
	}
}

func TestLooksSecret(t *testing.T) {
	cases := map[string]bool{
		"API_SECRET":         true,
		"DB_PASSWORD":        true,
		"PRIVATE_KEY":        true,
		"AUTH_TOKEN":         true,
		"STRIPE_API_KEY":     true,
		"AWS_ACCESS_KEY":     true,
		"OAUTH_CREDENTIALS":  true,
		"PORT":               false,
		"DEBUG":              false,
		"PUBLIC_URL":         false,
		"NEXT_PUBLIC_API_URL": false,
	}
	for k, want := range cases {
		if got := LooksSecret(k); got != want {
			t.Errorf("LooksSecret(%q) = %v, want %v", k, got, want)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
