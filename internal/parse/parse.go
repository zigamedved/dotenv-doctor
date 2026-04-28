// Package parse turns a .env file into a slice of Entry values.
//
// We deliberately do not depend on godotenv at runtime: we keep line numbers
// for accurate leak-source reporting, and we never want to expose unmasked
// values outside of an explicit caller request. The compatibility tests in
// parse_test.go cover the godotenv-style fixtures we care about.
package parse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

// Entry is a single key/value pair from a .env file.
type Entry struct {
	Key   string
	Value string
	Line  int // 1-based line where the key appears
	// Quoted reports the surrounding quote char of the value, or 0 if unquoted.
	Quoted rune
}

// File is a parsed .env file.
type File struct {
	Path    string
	Entries []Entry
}

// Keys returns the entry keys in order.
func (f File) Keys() []string {
	out := make([]string, 0, len(f.Entries))
	for _, e := range f.Entries {
		out = append(out, e.Key)
	}
	return out
}

// Get returns the entry for a key (case-sensitive), or false.
func (f File) Get(key string) (Entry, bool) {
	for _, e := range f.Entries {
		if e.Key == key {
			return e, true
		}
	}
	return Entry{}, false
}

// ParseFile reads and parses a file by path.
func ParseFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	entries, err := Parse(bytes.NewReader(data))
	if err != nil {
		return File{}, fmt.Errorf("%s: %w", path, err)
	}
	return File{Path: path, Entries: entries}, nil
}

// Parse reads .env-formatted bytes from r and returns the entries.
//
// Supports:
//   - KEY=value
//   - KEY="value with spaces"
//   - KEY='single quoted'
//   - KEY=value # trailing comment (only when unquoted)
//   - export KEY=value
//   - empty values (KEY=)
//   - line continuation in double-quoted values via literal newlines
//   - escape sequences inside double quotes: \n, \r, \t, \\, \"
//
// Does not support shell expansion ($VAR) — values are treated literally.
func Parse(r io.Reader) ([]Entry, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var (
		entries []Entry
		lineNo  int
	)
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Strip optional `export ` prefix.
		if strings.HasPrefix(trimmed, "export ") || strings.HasPrefix(trimmed, "export\t") {
			trimmed = strings.TrimLeftFunc(trimmed[len("export"):], unicode.IsSpace)
		}
		eq := strings.IndexByte(trimmed, '=')
		if eq <= 0 {
			// Malformed line; skip silently. We don't want a single bad line
			// to abort the scan of someone's whole repo set.
			continue
		}
		key := strings.TrimRightFunc(trimmed[:eq], unicode.IsSpace)
		if !validKey(key) {
			continue
		}
		raw := trimmed[eq+1:]

		entry := Entry{Key: key, Line: lineNo}

		switch {
		case strings.HasPrefix(raw, `"`):
			val, consumed, ok, err := readDoubleQuoted(raw, scanner, &lineNo)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			entry.Value = val
			entry.Quoted = '"'
			_ = consumed
		case strings.HasPrefix(raw, `'`):
			val, ok := readSingleQuoted(raw)
			if !ok {
				continue
			}
			entry.Value = val
			entry.Quoted = '\''
		default:
			entry.Value = stripTrailingComment(strings.TrimSpace(raw))
		}

		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func validKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		if i == 0 && (unicode.IsDigit(r)) {
			return false
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

// readDoubleQuoted reads a double-quoted value that may span multiple lines.
// The leading quote in raw[0] is consumed.
func readDoubleQuoted(raw string, scanner *bufio.Scanner, lineNo *int) (string, int, bool, error) {
	var (
		sb       strings.Builder
		consumed int
	)
	// Skip the opening quote.
	body := raw[1:]
	consumed = 1
	for {
		closed := false
		var i int
		for i = 0; i < len(body); i++ {
			c := body[i]
			if c == '\\' && i+1 < len(body) {
				switch body[i+1] {
				case 'n':
					sb.WriteByte('\n')
				case 'r':
					sb.WriteByte('\r')
				case 't':
					sb.WriteByte('\t')
				case '\\':
					sb.WriteByte('\\')
				case '"':
					sb.WriteByte('"')
				case '\'':
					sb.WriteByte('\'')
				default:
					sb.WriteByte(body[i+1])
				}
				i++
				continue
			}
			if c == '"' {
				closed = true
				break
			}
			sb.WriteByte(c)
		}
		consumed += i
		if closed {
			return sb.String(), consumed, true, nil
		}
		// Need to read another line.
		if !scanner.Scan() {
			// EOF without closing quote — keep what we have.
			return sb.String(), consumed, true, nil
		}
		*lineNo++
		sb.WriteByte('\n')
		body = scanner.Text()
		consumed += len(body) + 1
	}
}

func readSingleQuoted(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "'") {
		return "", false
	}
	end := strings.IndexByte(raw[1:], '\'')
	if end < 0 {
		return raw[1:], true
	}
	return raw[1 : 1+end], true
}

// stripTrailingComment removes a trailing `# comment` that follows whitespace.
// Inside-quoted handling is done by the quoted readers above, so this only
// runs on already-unquoted values.
func stripTrailingComment(v string) string {
	for i := 0; i < len(v); i++ {
		if v[i] == '#' && (i == 0 || isSpace(v[i-1])) {
			return strings.TrimRightFunc(v[:i], unicode.IsSpace)
		}
	}
	return v
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

// Mask returns a redacted display form of value:
//   - empty stays empty
//   - <=4 chars: bullets only
//   - otherwise: first 2 + bullets + last 2 (matches port-whisperer aesthetic)
func Mask(value string) string {
	if value == "" {
		return ""
	}
	n := len(value)
	if n <= 4 {
		return strings.Repeat("•", n)
	}
	// At most 8 bullets to keep tables tidy.
	mid := n - 4
	if mid > 8 {
		mid = 8
	}
	return value[:2] + strings.Repeat("•", mid) + value[n-2:]
}
