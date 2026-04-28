package parse

import (
	"strings"
	"testing"
)

func TestParseSimple(t *testing.T) {
	in := `KEY=value
OTHER=second
`
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	if got[0].Key != "KEY" || got[0].Value != "value" || got[0].Line != 1 {
		t.Errorf("entry 0 mismatch: %+v", got[0])
	}
	if got[1].Key != "OTHER" || got[1].Value != "second" || got[1].Line != 2 {
		t.Errorf("entry 1 mismatch: %+v", got[1])
	}
}

func TestParseDoubleQuoted(t *testing.T) {
	in := `KEY="hello world"`
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0].Value != "hello world" || got[0].Quoted != '"' {
		t.Errorf("got: %+v", got)
	}
}

func TestParseSingleQuoted(t *testing.T) {
	in := `KEY='a # not comment'`
	got, _ := Parse(strings.NewReader(in))
	if len(got) != 1 || got[0].Value != "a # not comment" {
		t.Errorf("got: %+v", got)
	}
}

func TestParseDoubleQuotedEscapes(t *testing.T) {
	in := `KEY="line1\nline2\tend"`
	got, _ := Parse(strings.NewReader(in))
	want := "line1\nline2\tend"
	if got[0].Value != want {
		t.Errorf("want %q, got %q", want, got[0].Value)
	}
}

func TestParseDoubleQuotedMultiline(t *testing.T) {
	in := "KEY=\"hello\nworld\"\nOTHER=ok"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d (%+v)", len(got), got)
	}
	if !strings.Contains(got[0].Value, "hello") || !strings.Contains(got[0].Value, "world") {
		t.Errorf("multiline value lost newline content: %q", got[0].Value)
	}
	if got[1].Key != "OTHER" {
		t.Errorf("second entry not parsed: %+v", got[1])
	}
}

func TestParseTrailingComment(t *testing.T) {
	in := `KEY=value # this is a comment`
	got, _ := Parse(strings.NewReader(in))
	if got[0].Value != "value" {
		t.Errorf("want 'value', got %q", got[0].Value)
	}
}

func TestParseExportPrefix(t *testing.T) {
	in := `export KEY=value`
	got, _ := Parse(strings.NewReader(in))
	if len(got) != 1 || got[0].Key != "KEY" || got[0].Value != "value" {
		t.Errorf("got: %+v", got)
	}
}

func TestParseEmptyValue(t *testing.T) {
	in := `KEY=`
	got, _ := Parse(strings.NewReader(in))
	if len(got) != 1 || got[0].Key != "KEY" || got[0].Value != "" {
		t.Errorf("got: %+v", got)
	}
}

func TestParseSkipsBlankAndComment(t *testing.T) {
	in := `
# a comment

KEY=value
`
	got, _ := Parse(strings.NewReader(in))
	if len(got) != 1 || got[0].Key != "KEY" {
		t.Errorf("got: %+v", got)
	}
	// Line number should reflect actual file line.
	if got[0].Line != 4 {
		t.Errorf("want line 4, got %d", got[0].Line)
	}
}

func TestParseRejectsNumericKeyStart(t *testing.T) {
	in := `1KEY=value`
	got, _ := Parse(strings.NewReader(in))
	if len(got) != 0 {
		t.Errorf("expected no entries, got: %+v", got)
	}
}

func TestParseAcceptsUnderscoreAndDot(t *testing.T) {
	in := "_FOO.BAR=baz"
	got, _ := Parse(strings.NewReader(in))
	if len(got) != 1 || got[0].Key != "_FOO.BAR" {
		t.Errorf("got: %+v", got)
	}
}

func TestParseRejectsSpacesInKey(t *testing.T) {
	in := "BAD KEY=value"
	got, _ := Parse(strings.NewReader(in))
	if len(got) != 0 {
		t.Errorf("expected no entries, got: %+v", got)
	}
}

func TestParseSkipsMalformedLines(t *testing.T) {
	in := "JUST_A_LINE\nKEY=value"
	got, _ := Parse(strings.NewReader(in))
	if len(got) != 1 || got[0].Key != "KEY" {
		t.Errorf("got: %+v", got)
	}
}

func TestParseUnclosedQuoteEOFTolerated(t *testing.T) {
	in := `KEY="oops`
	got, _ := Parse(strings.NewReader(in))
	if len(got) != 1 || got[0].Key != "KEY" {
		t.Errorf("got: %+v", got)
	}
}

func TestFileGet(t *testing.T) {
	f := File{Entries: []Entry{{Key: "A", Value: "1"}, {Key: "B", Value: "2"}}}
	if e, ok := f.Get("B"); !ok || e.Value != "2" {
		t.Errorf("Get(B) = %+v, %v", e, ok)
	}
	if _, ok := f.Get("missing"); ok {
		t.Errorf("Get(missing) should be false")
	}
}

func TestMaskEmpty(t *testing.T) {
	if Mask("") != "" {
		t.Errorf("empty should mask to empty")
	}
}

func TestMaskShort(t *testing.T) {
	if got := Mask("ab"); got != "••" {
		t.Errorf("got %q", got)
	}
	if got := Mask("abcd"); got != "••••" {
		t.Errorf("got %q", got)
	}
}

func TestMaskLong(t *testing.T) {
	got := Mask("abcdefghij")
	if !strings.HasPrefix(got, "ab") || !strings.HasSuffix(got, "ij") {
		t.Errorf("mask shape wrong: %q", got)
	}
	if !strings.Contains(got, "•") {
		t.Errorf("mask missing bullets: %q", got)
	}
}

func TestParseLineNumbers(t *testing.T) {
	in := `A=1
B=2
C=3`
	got, _ := Parse(strings.NewReader(in))
	if got[0].Line != 1 || got[1].Line != 2 || got[2].Line != 3 {
		t.Errorf("line numbers wrong: %+v", got)
	}
}

func TestParseLeadingWhitespaceLine(t *testing.T) {
	in := "   KEY=value"
	got, _ := Parse(strings.NewReader(in))
	if len(got) != 1 || got[0].Value != "value" {
		t.Errorf("got: %+v", got)
	}
}
