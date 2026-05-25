package grep

import (
	"testing"

	"mygrep/internal/protocol"
)

func mustMatcher(t *testing.T, f protocol.GrepFlags) *Matcher {
	t.Helper()
	m, err := NewMatcher(f)
	if err != nil {
		t.Fatalf("NewMatcher: %v", err)
	}
	return m
}

func TestMatcher_FixedString(t *testing.T) {
	m := mustMatcher(t, protocol.GrepFlags{Pattern: "ERR", FixedString: true})
	if !m.Match("an ERR happened") {
		t.Error("want match")
	}
	if m.Match("nothing here") {
		t.Error("want no match")
	}
}

func TestMatcher_IgnoreCase_Fixed(t *testing.T) {
	m := mustMatcher(t, protocol.GrepFlags{Pattern: "err", FixedString: true, IgnoreCase: true})
	if !m.Match("ERR found") {
		t.Error("want match")
	}
}

func TestMatcher_Regex(t *testing.T) {
	m := mustMatcher(t, protocol.GrepFlags{Pattern: `^foo\d+$`})
	if !m.Match("foo42") {
		t.Error("want match")
	}
	if m.Match("bar42") {
		t.Error("want no match")
	}
}

func TestMatcher_IgnoreCase_Regex(t *testing.T) {
	m := mustMatcher(t, protocol.GrepFlags{Pattern: `^foo`, IgnoreCase: true})
	if !m.Match("FOObar") {
		t.Error("want match")
	}
}

func TestMatcher_Invert(t *testing.T) {
	m := mustMatcher(t, protocol.GrepFlags{Pattern: "skip", FixedString: true, InvertMatch: true})
	if !m.Match("keep this") {
		t.Error("want match (inverted)")
	}
	if m.Match("skip this") {
		t.Error("want no match (inverted)")
	}
}

func TestMatcher_BadRegex(t *testing.T) {
	if _, err := NewMatcher(protocol.GrepFlags{Pattern: "[invalid"}); err == nil {
		t.Fatal("want error for invalid regex")
	}
}
