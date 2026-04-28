package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestParseGitHubOriginURL(t *testing.T) {
	cases := []struct {
		url   string
		owner string
		repo  string
		ok    bool
	}{
		{"https://github.com/foo/bar", "foo", "bar", true},
		{"https://github.com/foo/bar.git", "foo", "bar", true},
		{"git@github.com:foo/bar.git", "foo", "bar", true},
		{"git@github.com:foo/bar", "foo", "bar", true},
		{"ssh://git@github.com/foo/bar.git", "foo", "bar", true},
		{"https://gitlab.com/foo/bar", "", "", false},
		{"random-string", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		owner, repo, ok := ParseGitHubOriginURL(c.url)
		if owner != c.owner || repo != c.repo || ok != c.ok {
			t.Errorf("ParseGitHubOriginURL(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.url, owner, repo, ok, c.owner, c.repo, c.ok)
		}
	}
}

func TestLooksLikeEnvFileGit(t *testing.T) {
	cases := map[string]bool{
		".env":            true,
		".env.production": true,
		".env.local":      true,
		".envrc":          false,
		"env":             false,
		"sub/.env":        true,
		"package.json":    false,
	}
	for name, want := range cases {
		if got := looksLikeEnvFile(name); got != want {
			t.Errorf("looksLikeEnvFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsExampleGit(t *testing.T) {
	if !isExample(".env.example") {
		t.Errorf(".env.example should be example")
	}
	if isExample(".env.local") {
		t.Errorf(".env.local should not be example")
	}
}

func TestScanRepoFindsCommittedEnv(t *testing.T) {
	repo, dir := initTestRepo(t)
	addCommit(t, repo, dir, ".env", "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE\n", "leak commit")

	leaks, err := ScanRepo(dir, 0)
	if err != nil {
		t.Fatalf("ScanRepo err: %v", err)
	}
	if len(leaks) != 1 {
		t.Fatalf("want 1 leak, got %d", len(leaks))
	}
	if leaks[0].Path != ".env" {
		t.Errorf("path: %q", leaks[0].Path)
	}
	if !leaks[0].StillExists {
		t.Errorf("expected StillExists=true since file is at HEAD")
	}
	if len(leaks[0].Findings) == 0 {
		t.Errorf("expected at least one classifier finding")
	}
}

func TestScanRepoSkipsExampleFiles(t *testing.T) {
	repo, dir := initTestRepo(t)
	addCommit(t, repo, dir, ".env.example", "API_KEY=\n", "example commit")

	leaks, err := ScanRepo(dir, 0)
	if err != nil {
		t.Fatalf("ScanRepo err: %v", err)
	}
	if len(leaks) != 0 {
		t.Errorf("expected example file to be skipped, got %+v", leaks)
	}
}

func TestScanRepoEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, err := gogit.PlainInit(dir, false); err != nil {
		t.Fatalf("init: %v", err)
	}
	leaks, err := ScanRepo(dir, 0)
	if err != nil {
		t.Errorf("empty repo should not error: %v", err)
	}
	if len(leaks) != 0 {
		t.Errorf("want 0 leaks, got %d", len(leaks))
	}
}

func TestRemediationHintMentionsRotation(t *testing.T) {
	hint := RemediationHint(Leak{Path: ".env", StillExists: true})
	if !strContains(hint, "rotate") && !strContains(hint, "Rotate") {
		t.Errorf("remediation should mention rotation, got: %s", hint)
	}
	if !strContains(hint, ".env") {
		t.Errorf("remediation should mention the path, got: %s", hint)
	}
}

func TestIsTracked(t *testing.T) {
	repo, dir := initTestRepo(t)
	envPath := filepath.Join(dir, ".env")
	addCommit(t, repo, dir, ".env", "K=v\n", "commit")
	if !IsTracked(dir, envPath) {
		t.Errorf("expected .env to be tracked")
	}
	if IsTracked(dir, filepath.Join(dir, "missing.txt")) {
		t.Errorf("missing file should not be tracked")
	}
}

func initTestRepo(t *testing.T) (*gogit.Repository, string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	return repo, dir
}

func addCommit(t *testing.T, repo *gogit.Repository, dir, name, content, msg string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add(name); err != nil {
		t.Fatal(err)
	}
	_, err = wt.Commit(msg, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func strContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
