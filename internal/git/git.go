// Package git provides repository-aware helpers for dotenv-doctor.
//
// The leak scan walks every commit reachable from HEAD and looks for blobs
// at .env-style paths. We rely on go-git so users do not need a system git
// binary, and so the tool stays a single static binary.
package git

import (
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/zigamedved/dotenv-doctor/internal/classify"
)

// Leak describes a secret found in git history.
type Leak struct {
	Repo       string             // git repo root
	Path       string             // path inside the repo of the offending file
	Commit     string             // full SHA
	ShortSHA   string             // 7-char SHA for display
	Author     string             // name <email>
	When       string             // RFC3339 timestamp
	Subject    string             // first line of commit message
	Findings   []classify.Finding // pattern matches
	StillExists bool              // file is still committed at HEAD
}

// ScanRepo walks every commit and returns leaks for any .env-style file ever committed.
// `limit` caps the number of commits walked (0 = no limit). For large repos this matters.
func ScanRepo(repoDir string, limit int) ([]Leak, error) {
	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		return nil, fmt.Errorf("open repo %s: %w", repoDir, err)
	}

	headRef, err := repo.Head()
	if err != nil {
		// Empty repo — nothing to do.
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("resolve HEAD: %w", err)
	}

	headTree, err := treeForCommit(repo, headRef.Hash())
	if err != nil {
		return nil, err
	}
	headEnvPaths := envPathsInTree(headTree)

	iter, err := repo.Log(&gogit.LogOptions{From: headRef.Hash()})
	if err != nil {
		return nil, fmt.Errorf("log: %w", err)
	}
	defer iter.Close()

	// Dedupe by (path, blob hash) so we don't report the same content from
	// every commit that touches the file.
	seen := map[string]struct{}{}
	var leaks []Leak
	commitCount := 0

	err = iter.ForEach(func(c *object.Commit) error {
		if limit > 0 && commitCount >= limit {
			return io.EOF
		}
		commitCount++

		tree, err := c.Tree()
		if err != nil {
			return nil
		}

		err = tree.Files().ForEach(func(f *object.File) error {
			if !looksLikeEnvFile(f.Name) {
				return nil
			}
			if isExample(f.Name) {
				return nil
			}
			key := f.Name + ":" + f.Hash.String()
			if _, dup := seen[key]; dup {
				return nil
			}
			seen[key] = struct{}{}

			content, err := f.Contents()
			if err != nil {
				return nil
			}
			findings := classify.ClassifyText(content)
			// We always report committed .env files, even with no pattern match,
			// because the file existing in history is itself a finding.
			leak := Leak{
				Repo:        repoDir,
				Path:        f.Name,
				Commit:      c.Hash.String(),
				ShortSHA:    c.Hash.String()[:7],
				Author:      fmt.Sprintf("%s <%s>", c.Author.Name, c.Author.Email),
				When:        c.Author.When.Format("2006-01-02"),
				Subject:     firstLine(c.Message),
				Findings:    findings,
				StillExists: contains(headEnvPaths, f.Name),
			}
			leaks = append(leaks, leak)
			return nil
		})
		return err
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return leaks, nil
}

func treeForCommit(repo *gogit.Repository, hash plumbing.Hash) (*object.Tree, error) {
	c, err := repo.CommitObject(hash)
	if err != nil {
		return nil, err
	}
	return c.Tree()
}

func envPathsInTree(t *object.Tree) []string {
	if t == nil {
		return nil
	}
	var out []string
	_ = t.Files().ForEach(func(f *object.File) error {
		base := path.Base(f.Name)
		if looksLikeEnvFile(base) && !isExample(base) {
			out = append(out, f.Name)
		}
		return nil
	})
	return out
}

func looksLikeEnvFile(name string) bool {
	base := path.Base(name)
	if base == ".envrc" {
		return false
	}
	if base == ".env" {
		return true
	}
	return strings.HasPrefix(base, ".env.")
}

func isExample(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".example") ||
		strings.HasSuffix(lower, ".sample") ||
		strings.HasSuffix(lower, ".template")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// OriginURL returns the URL of the origin remote, or "" if not set.
func OriginURL(repoDir string) string {
	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		return ""
	}
	cfg, err := repo.Config()
	if err != nil {
		return ""
	}
	if r, ok := cfg.Remotes["origin"]; ok && len(r.URLs) > 0 {
		return r.URLs[0]
	}
	return ""
}

// ParseGitHubOriginURL extracts (owner, repo) from common GitHub remote URL
// formats: https://github.com/owner/repo(.git), git@github.com:owner/repo(.git),
// ssh://git@github.com/owner/repo(.git).
func ParseGitHubOriginURL(url string) (string, string, bool) {
	url = strings.TrimSuffix(url, ".git")
	for _, prefix := range []string{
		"https://github.com/",
		"http://github.com/",
		"ssh://git@github.com/",
		"git@github.com:",
	} {
		if strings.HasPrefix(url, prefix) {
			rest := strings.TrimPrefix(url, prefix)
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				return parts[0], parts[1], true
			}
		}
	}
	return "", "", false
}

// IsTracked reports whether absPath is currently tracked at HEAD in the repo
// rooted at repoDir. Used by the dashboard for fast committed-.env detection
// without walking history.
func IsTracked(repoDir, absPath string) bool {
	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		return false
	}
	headRef, err := repo.Head()
	if err != nil {
		return false
	}
	commit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return false
	}
	tree, err := commit.Tree()
	if err != nil {
		return false
	}
	rel, err := relPath(repoDir, absPath)
	if err != nil {
		return false
	}
	_, err = tree.File(rel)
	return err == nil
}

func relPath(repoDir, absPath string) (string, error) {
	rel := strings.TrimPrefix(absPath, repoDir)
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, "\\")
	if rel == "" {
		return "", fmt.Errorf("path %s not under repo %s", absPath, repoDir)
	}
	return rel, nil
}

// RemediationHint returns copy-pastable advice for a leak.
func RemediationHint(l Leak) string {
	if l.StillExists {
		return fmt.Sprintf(
			"File still tracked at HEAD. Remove with:\n"+
				"  git rm --cached %s\n"+
				"  echo %s >> .gitignore\n"+
				"Then rotate the leaked credentials and rewrite history with git filter-repo --path %s --invert-paths",
			l.Path, l.Path, l.Path)
	}
	return fmt.Sprintf(
		"File is no longer at HEAD but remains in history.\n"+
			"  Rotate the leaked credentials immediately.\n"+
			"  Rewrite history: git filter-repo --path %s --invert-paths\n"+
			"  Or use BFG: bfg --delete-files %s",
		l.Path, path.Base(l.Path))
}
