// Package discover walks scan paths to find projects and their .env files.
//
// A "project" is any directory that contains at least one .env-style file.
// We name projects by their directory (basename), grouping all .env* files
// in the same directory together. We do NOT group nested dirs as a single
// project — each directory with .env files stands on its own.
package discover

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Project is a single directory that contains .env files.
type Project struct {
	// Name is a human-friendly label. Defaults to the directory's basename;
	// callers may set it to a more specific path-suffix when there are
	// duplicates.
	Name string

	// Dir is the absolute path to the directory.
	Dir string

	// EnvFiles are the absolute paths to the .env-style files in Dir.
	EnvFiles []string

	// ExampleFile is the absolute path to .env.example (or .env.sample) if present.
	ExampleFile string

	// GitRoot is the absolute path to the enclosing git repository, or "" if none.
	GitRoot string
}

// Options control discovery behavior.
type Options struct {
	ScanPaths []string
	MaxDepth  int
	SkipDirs  []string
}

// DefaultSkipDirs are directories never worth descending into.
var DefaultSkipDirs = []string{
	"node_modules", ".git", ".svn", ".hg", "vendor",
	"dist", "build", ".next", ".nuxt", ".cache", "target",
	"__pycache__", ".venv", "venv", "env", ".tox",
	"coverage", ".idea", ".vscode", ".terraform",
	"DerivedData", "Pods",
}

// Discover walks each scan path and returns the deduplicated list of projects.
func Discover(opts Options) ([]Project, error) {
	skip := mergeSkip(opts.SkipDirs)
	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 4
	}

	seen := map[string]*Project{}
	for _, root := range opts.ScanPaths {
		root, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		walkOne(root, maxDepth, skip, seen)
	}

	out := make([]Project, 0, len(seen))
	for _, p := range seen {
		sort.Strings(p.EnvFiles)
		out = append(out, *p)
	}

	resolveDuplicateNames(out)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func walkOne(root string, maxDepth int, skip map[string]struct{}, seen map[string]*Project) {
	rootDepth := strings.Count(filepath.Clean(root), string(os.PathSeparator))
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Permission denied / vanishing dirs / etc — skip silently.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			depth := strings.Count(filepath.Clean(path), string(os.PathSeparator)) - rootDepth
			if depth > maxDepth {
				return filepath.SkipDir
			}
			name := d.Name()
			if _, drop := skip[name]; drop && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !looksLikeEnvFile(d.Name()) {
			return nil
		}
		dir := filepath.Dir(path)
		p, ok := seen[dir]
		if !ok {
			p = &Project{
				Name:    filepath.Base(dir),
				Dir:     dir,
				GitRoot: findGitRoot(dir),
			}
			seen[dir] = p
		}
		if isExample(d.Name()) {
			if p.ExampleFile == "" {
				p.ExampleFile = path
			}
		}
		p.EnvFiles = append(p.EnvFiles, path)
		return nil
	})
}

// looksLikeEnvFile reports whether a basename is a .env-style file.
//
// Matched names:
//   - .env
//   - .env.<anything>     (.env.local, .env.production, .env.example, ...)
//   - env                 (plain "env" file at project root, used by some stacks)
//
// We deliberately exclude .envrc (direnv) — different tool, different shape.
func looksLikeEnvFile(name string) bool {
	if name == ".envrc" {
		return false
	}
	if name == ".env" {
		return true
	}
	if strings.HasPrefix(name, ".env.") {
		return true
	}
	return false
}

func isExample(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".example") ||
		strings.HasSuffix(lower, ".sample") ||
		strings.HasSuffix(lower, ".template")
}

func findGitRoot(dir string) string {
	cur := dir
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

func mergeSkip(extra []string) map[string]struct{} {
	out := make(map[string]struct{}, len(DefaultSkipDirs)+len(extra))
	for _, d := range DefaultSkipDirs {
		out[d] = struct{}{}
	}
	for _, d := range extra {
		out[d] = struct{}{}
	}
	return out
}

// resolveDuplicateNames disambiguates projects with the same basename by
// growing each colliding name's path prefix until every project has a unique
// name. Uses the "minimum unique suffix" strategy: each project starts as its
// basename, and we keep prepending parent path components — but only for
// projects still in a collision — until no duplicates remain.
//
// This handles deep collisions like:
//
//	~/code/repo-a/infra/config/development
//	~/code/repo-b/infra/config/development
//
// where the previous one-level-up scheme would incorrectly produce
// "config/development" for both.
func resolveDuplicateNames(projects []Project) {
	if len(projects) <= 1 {
		return
	}

	type entry struct {
		idx        int
		components []string // path components, leaf-first
		used       int      // how many components are currently included in the name
	}
	work := make([]*entry, len(projects))
	for i := range projects {
		work[i] = &entry{
			idx:        i,
			components: pathComponentsLeafFirst(projects[i].Dir),
			used:       1,
		}
	}

	// Bound the loop by the deepest path so we can never spin forever.
	maxRounds := 0
	for _, e := range work {
		if len(e.components) > maxRounds {
			maxRounds = len(e.components)
		}
	}

	for round := 0; round < maxRounds; round++ {
		groups := map[string][]*entry{}
		for _, e := range work {
			groups[buildName(e.components, e.used)] = append(groups[buildName(e.components, e.used)], e)
		}
		collided := false
		for _, group := range groups {
			if len(group) <= 1 {
				continue
			}
			collided = true
			for _, e := range group {
				if e.used < len(e.components) {
					e.used++
				}
			}
		}
		if !collided {
			break
		}
	}

	for _, e := range work {
		projects[e.idx].Name = buildName(e.components, e.used)
	}
}

// pathComponentsLeafFirst returns the path's components ordered from the
// leaf (basename) to the filesystem root. Empty components from absolute
// paths or trailing separators are dropped.
func pathComponentsLeafFirst(dir string) []string {
	cleaned := filepath.Clean(dir)
	parts := strings.Split(cleaned, string(filepath.Separator))
	out := make([]string, 0, len(parts))
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			out = append(out, parts[i])
		}
	}
	return out
}

// buildName joins the first `used` leaf-first components into a leaf-last
// path string ("parent/leaf"). Always returns at least the basename.
func buildName(leafFirst []string, used int) string {
	if len(leafFirst) == 0 {
		return ""
	}
	if used <= 0 {
		used = 1
	}
	if used > len(leafFirst) {
		used = len(leafFirst)
	}
	parts := make([]string, used)
	for i := 0; i < used; i++ {
		parts[used-1-i] = leafFirst[i]
	}
	return strings.Join(parts, "/")
}

// SmartWalk returns scan paths derived from $PWD when no config exists. It
// picks the nearest git root if there is one, otherwise the current directory.
func SmartWalk(cwd string) []string {
	if root := findGitRoot(cwd); root != "" {
		return []string{root}
	}
	return []string{cwd}
}
