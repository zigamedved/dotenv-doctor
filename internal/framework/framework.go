// Package framework detects which web framework a project is built on, and
// flags env keys whose names will be exposed to the public bundle when they
// look secret. This is the "tweetable insight" feature from the plan.
package framework

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Kind enumerates frameworks we recognize.
type Kind string

const (
	Unknown Kind = ""
	NextJS  Kind = "Next.js"
	Vite    Kind = "Vite"
	CRA     Kind = "Create React App"
	Nuxt    Kind = "Nuxt"
	Django  Kind = "Django"
	Rails   Kind = "Rails"
	Express Kind = "Express"
	FastAPI Kind = "FastAPI"
	Flask   Kind = "Flask"
)

// PublicPrefixes is the set of env-key prefixes a framework bakes into the
// client bundle. Anything matching is shipped to the browser; secrets must
// never use these prefixes.
func PublicPrefixes(k Kind) []string {
	switch k {
	case NextJS:
		return []string{"NEXT_PUBLIC_"}
	case Vite:
		return []string{"VITE_"}
	case CRA:
		return []string{"REACT_APP_"}
	case Nuxt:
		return []string{"NUXT_PUBLIC_"}
	default:
		return nil
	}
}

// Detect inspects a project root and returns the most likely framework. It
// prefers strong signals (dependencies in package.json) over weak ones (file
// presence). Returns Unknown if nothing matches.
func Detect(root string) Kind {
	if k := fromPackageJSON(filepath.Join(root, "package.json")); k != Unknown {
		return k
	}
	if k := fromPython(root); k != Unknown {
		return k
	}
	if exists(filepath.Join(root, "config", "application.rb")) || exists(filepath.Join(root, "Gemfile")) {
		// Cheap Rails check.
		if hasGem(filepath.Join(root, "Gemfile"), "rails") {
			return Rails
		}
	}
	return Unknown
}

func fromPackageJSON(path string) Kind {
	data, err := os.ReadFile(path)
	if err != nil {
		return Unknown
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return Unknown
	}
	has := func(name string) bool {
		_, ok := pkg.Dependencies[name]
		if ok {
			return true
		}
		_, ok = pkg.DevDependencies[name]
		return ok
	}
	switch {
	case has("next"):
		return NextJS
	case has("nuxt"), has("nuxt3"):
		return Nuxt
	case has("vite"):
		return Vite
	case has("react-scripts"):
		return CRA
	case has("express"):
		return Express
	}
	return Unknown
}

func fromPython(root string) Kind {
	req := filepath.Join(root, "requirements.txt")
	if data, err := os.ReadFile(req); err == nil {
		text := strings.ToLower(string(data))
		switch {
		case strings.Contains(text, "django"):
			return Django
		case strings.Contains(text, "fastapi"):
			return FastAPI
		case strings.Contains(text, "flask"):
			return Flask
		}
	}
	if exists(filepath.Join(root, "manage.py")) {
		return Django
	}
	if exists(filepath.Join(root, "pyproject.toml")) {
		if data, err := os.ReadFile(filepath.Join(root, "pyproject.toml")); err == nil {
			text := strings.ToLower(string(data))
			switch {
			case strings.Contains(text, "django"):
				return Django
			case strings.Contains(text, "fastapi"):
				return FastAPI
			case strings.Contains(text, "flask"):
				return Flask
			}
		}
	}
	return Unknown
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func hasGem(gemfile, gem string) bool {
	data, err := os.ReadFile(gemfile)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(`gem "`+gem)) ||
		strings.Contains(strings.ToLower(string(data)), strings.ToLower(`gem '`+gem))
}

// IsPublic reports whether key is exposed to the browser by the given framework.
func IsPublic(k Kind, key string) bool {
	for _, p := range PublicPrefixes(k) {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// LooksSecret returns true if the key name suggests the value is sensitive.
// Used together with IsPublic to flag exposure footguns.
func LooksSecret(key string) bool {
	upper := strings.ToUpper(key)
	for _, sub := range []string{"SECRET", "PASSWORD", "PRIVATE", "TOKEN", "API_KEY", "APIKEY", "ACCESS_KEY", "CREDENTIAL"} {
		if strings.Contains(upper, sub) {
			return true
		}
	}
	return false
}
