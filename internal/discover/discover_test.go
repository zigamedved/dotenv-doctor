package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLooksLikeEnvFile(t *testing.T) {
	cases := map[string]bool{
		".env":             true,
		".env.local":       true,
		".env.production":  true,
		".env.example":     true,
		".env.dev.example": true,
		".envrc":           false, // direnv, intentionally excluded
		"env":              false,
		"foo.env":          false,
		"package.json":     false,
		"":                 false,
	}
	for name, want := range cases {
		if got := looksLikeEnvFile(name); got != want {
			t.Errorf("looksLikeEnvFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsExample(t *testing.T) {
	if !isExample(".env.example") {
		t.Errorf(".env.example should be example")
	}
	if !isExample(".env.sample") {
		t.Errorf(".env.sample should be example")
	}
	if !isExample(".env.template") {
		t.Errorf(".env.template should be example")
	}
	if isExample(".env.local") {
		t.Errorf(".env.local should NOT be example")
	}
	if isExample(".env") {
		t.Errorf(".env should NOT be example")
	}
}

func TestDiscoverFindsEnvFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "proj-a", ".env"), "KEY=value\n")
	mustWrite(t, filepath.Join(dir, "proj-a", ".env.example"), "KEY=\n")
	mustWrite(t, filepath.Join(dir, "proj-b", ".env.local"), "OTHER=1\n")
	mustWrite(t, filepath.Join(dir, "proj-b", "package.json"), "{}\n")

	projects, err := Discover(Options{ScanPaths: []string{dir}})
	if err != nil {
		t.Fatalf("Discover err: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("want 2 projects, got %d (%+v)", len(projects), projects)
	}
}

func TestDiscoverSkipsNodeModules(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "proj", ".env"), "K=v\n")
	mustWrite(t, filepath.Join(dir, "proj", "node_modules", "pkg", ".env"), "SHOULD_NOT_APPEAR=1\n")
	projects, _ := Discover(Options{ScanPaths: []string{dir}})
	if len(projects) != 1 {
		t.Fatalf("want 1 project, got %d", len(projects))
	}
	if len(projects[0].EnvFiles) != 1 {
		t.Errorf("expected node_modules .env to be skipped, got %v", projects[0].EnvFiles)
	}
}

func TestDiscoverMaxDepth(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "a", "b", "c", "d", "e", "f")
	mustWrite(t, filepath.Join(deep, ".env"), "K=v\n")
	projects, _ := Discover(Options{ScanPaths: []string{dir}, MaxDepth: 2})
	if len(projects) != 0 {
		t.Errorf("expected nothing past depth 2, got %+v", projects)
	}
}

func TestDiscoverDeduplicatesProjectNames(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "company-x", "frontend", ".env"), "K=v\n")
	mustWrite(t, filepath.Join(dir, "company-y", "frontend", ".env"), "K=v\n")
	projects, _ := Discover(Options{ScanPaths: []string{dir}})
	if len(projects) != 2 {
		t.Fatalf("want 2 projects, got %d", len(projects))
	}
	if projects[0].Name == projects[1].Name {
		t.Errorf("duplicate names not resolved: %v / %v", projects[0].Name, projects[1].Name)
	}
}

// TestDiscoverDeduplicatesDeepCollisions covers the real-world case where
// two projects share more than just their basename — e.g. multiple repos
// each have an infra/config/development directory. The naive "use parent
// dir as prefix" approach fails here; we must keep extending until unique.
func TestDiscoverDeduplicatesDeepCollisions(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "repo-a", "infra", "config", "development", ".env"), "K=v\n")
	mustWrite(t, filepath.Join(dir, "repo-b", "infra", "config", "development", ".env"), "K=v\n")
	mustWrite(t, filepath.Join(dir, "repo-a", "infra", "config", "production", ".env"), "K=v\n")
	mustWrite(t, filepath.Join(dir, "repo-b", "infra", "config", "production", ".env"), "K=v\n")

	projects, _ := Discover(Options{ScanPaths: []string{dir}, MaxDepth: 8})
	if len(projects) != 4 {
		t.Fatalf("want 4 projects, got %d", len(projects))
	}
	seen := map[string]int{}
	for _, p := range projects {
		seen[p.Name]++
	}
	for name, count := range seen {
		if count > 1 {
			t.Errorf("name %q used %d times — disambiguation failed", name, count)
		}
	}
	// Spot-check: at least one project name should mention its repo to make the
	// disambiguation actually meaningful to a human reader.
	hasRepoPrefix := false
	for _, p := range projects {
		if filepath.Base(p.Dir) == "development" && (containsString(p.Name, "repo-a") || containsString(p.Name, "repo-b")) {
			hasRepoPrefix = true
			break
		}
	}
	if !hasRepoPrefix {
		t.Errorf("disambiguated names should include the repo segment, got: %+v", projectNames(projects))
	}
}

func TestDiscoverIdentifiesExample(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "p", ".env"), "K=v\n")
	mustWrite(t, filepath.Join(dir, "p", ".env.example"), "K=\n")
	projects, _ := Discover(Options{ScanPaths: []string{dir}})
	if len(projects) != 1 {
		t.Fatalf("want 1, got %d", len(projects))
	}
	if projects[0].ExampleFile == "" {
		t.Errorf("expected ExampleFile to be set")
	}
}

func TestDiscoverIgnoresEnvrc(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "p", ".envrc"), "export FOO=bar\n")
	projects, _ := Discover(Options{ScanPaths: []string{dir}})
	if len(projects) != 0 {
		t.Errorf(".envrc should be ignored, got %+v", projects)
	}
}

func TestDiscoverHandlesMissingPath(t *testing.T) {
	projects, err := Discover(Options{ScanPaths: []string{"/path/that/does/not/exist/12345"}})
	if err != nil {
		t.Errorf("missing path should not error, got %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected no projects, got %+v", projects)
	}
}

func TestSmartWalkFallsBackToCwd(t *testing.T) {
	dir := t.TempDir() // not a git repo
	got := SmartWalk(dir)
	if len(got) != 1 || got[0] != dir {
		t.Errorf("SmartWalk(%q) = %v", dir, got)
	}
}

func projectNames(ps []Project) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

func containsString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
