package chunk

import (
	"strings"
	"testing"
)

func TestSplit_EmptyInput(t *testing.T) {
	got := Split("", 3)
	if len(got) != 3 {
		t.Fatalf("want 3 chunks, got %d", len(got))
	}
	for i, c := range got {
		if c.Data != "" {
			t.Errorf("chunk %d: want empty data, got %q", i, c.Data)
		}
	}
}

func TestSplit_TrailingNewline(t *testing.T) {
	// "a\nb\n" должен трактоваться как две строки.
	got := Split("a\nb\n", 2)
	if len(got) != 2 {
		t.Fatalf("want 2 chunks, got %d", len(got))
	}
	if got[0].Data != "a" || got[0].StartLine != 1 {
		t.Errorf("chunk 0: got {%d,%q}", got[0].StartLine, got[0].Data)
	}
	if got[1].Data != "b" || got[1].StartLine != 2 {
		t.Errorf("chunk 1: got {%d,%q}", got[1].StartLine, got[1].Data)
	}
}

func TestSplit_CRLF(t *testing.T) {
	got := Split("a\r\nb\r\nc\r\n", 3)
	if len(got) != 3 {
		t.Fatalf("want 3 chunks, got %d", len(got))
	}
	expected := []string{"a", "b", "c"}
	for i, c := range got {
		if c.Data != expected[i] {
			t.Errorf("chunk %d: want %q, got %q", i, expected[i], c.Data)
		}
	}
}

func TestSplit_FewerLinesThanServers(t *testing.T) {
	got := Split("only one line", 4)
	if len(got) != 4 {
		t.Fatalf("want 4 chunks, got %d", len(got))
	}
	if got[0].Data != "only one line" || got[0].StartLine != 1 {
		t.Errorf("chunk 0: got {%d,%q}", got[0].StartLine, got[0].Data)
	}
	for i := 1; i < 4; i++ {
		if got[i].Data != "" {
			t.Errorf("chunk %d: want empty, got %q", i, got[i].Data)
		}
	}
}

func TestSplit_PreservesAllLines(t *testing.T) {
	const N = 7
	lines := []string{
		"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliet",
		"kilo", "lima", "mike", "november", "oscar",
		"papa", "quebec", "romeo",
	}
	input := strings.Join(lines, "\n") + "\n"
	chunks := Split(input, N)

	var reassembled []string
	for _, c := range chunks {
		if c.Data == "" {
			continue
		}
		reassembled = append(reassembled, strings.Split(c.Data, "\n")...)
	}
	if strings.Join(reassembled, "|") != strings.Join(lines, "|") {
		t.Fatalf("reassembled mismatch:\nwant: %v\ngot:  %v", lines, reassembled)
	}
}

func TestSplit_StartLines(t *testing.T) {
	chunks := Split("l1\nl2\nl3\nl4\nl5\n", 3)
	wantStart := []int{1, 3, 5}
	for i, w := range wantStart {
		if chunks[i].StartLine != w {
			t.Errorf("chunk %d: want StartLine=%d, got %d", i, w, chunks[i].StartLine)
		}
	}
}

func TestSplit_ZeroN(t *testing.T) {
	got := Split("a\nb\n", 0)
	if len(got) != 1 {
		t.Fatalf("want 1 chunk for n=0, got %d", len(got))
	}
	if got[0].Data != "a\nb" {
		t.Errorf("got %q", got[0].Data)
	}
}
