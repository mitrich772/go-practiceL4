package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mygrep/internal/protocol"
)

func doProcess(t *testing.T, ts *httptest.Server, req protocol.ProcessRequest) protocol.ProcessResponse {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(ts.URL+"/process", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %s", resp.Status)
	}
	var out protocol.ProcessResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func TestServer_Healthz(t *testing.T) {
	ts := httptest.NewServer(New(Handler{Workers: 2}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %s", resp.Status)
	}
}

func TestServer_Process_Basic(t *testing.T) {
	ts := httptest.NewServer(New(Handler{Workers: 4}))
	defer ts.Close()

	resp := doProcess(t, ts, protocol.ProcessRequest{
		ChunkID:   2,
		StartLine: 10,
		Flags:     protocol.GrepFlags{Pattern: "needle", FixedString: true, PrintLineNum: true},
		Data:      "a\nneedle here\nb\nanother needle\nc",
	})
	if resp.ChunkID != 2 {
		t.Errorf("ChunkID: got %d, want 2", resp.ChunkID)
	}
	if len(resp.Matches) != 2 {
		t.Fatalf("matches: got %d, want 2", len(resp.Matches))
	}
	if resp.Matches[0].LineNum != 11 || resp.Matches[1].LineNum != 13 {
		t.Errorf("line numbers: got %+v", resp.Matches)
	}
}

func TestServer_Process_Count(t *testing.T) {
	ts := httptest.NewServer(New(Handler{Workers: 4}))
	defer ts.Close()

	resp := doProcess(t, ts, protocol.ProcessRequest{
		Flags: protocol.GrepFlags{Pattern: "x", FixedString: true, CountOnly: true},
		Data:  "x\ny\nxx\nz\nx",
	})
	if resp.Count != 3 {
		t.Errorf("count: got %d, want 3", resp.Count)
	}
}

func TestServer_Process_Invert(t *testing.T) {
	ts := httptest.NewServer(New(Handler{Workers: 2}))
	defer ts.Close()

	resp := doProcess(t, ts, protocol.ProcessRequest{
		Flags: protocol.GrepFlags{Pattern: "skip", FixedString: true, InvertMatch: true},
		Data:  "keep1\nskip me\nkeep2",
	})
	if len(resp.Matches) != 2 {
		t.Fatalf("got %d matches", len(resp.Matches))
	}
	if resp.Matches[0].Text != "keep1" || resp.Matches[1].Text != "keep2" {
		t.Errorf("matches: %+v", resp.Matches)
	}
}

func TestServer_Process_Empty(t *testing.T) {
	ts := httptest.NewServer(New(Handler{Workers: 2}))
	defer ts.Close()

	resp := doProcess(t, ts, protocol.ProcessRequest{
		Flags: protocol.GrepFlags{Pattern: "x", FixedString: true},
		Data:  "",
	})
	if len(resp.Matches) != 0 || resp.HasMatch {
		t.Errorf("want empty, got %+v", resp)
	}
}

func TestServer_Process_BadRegex(t *testing.T) {
	ts := httptest.NewServer(New(Handler{Workers: 2}))
	defer ts.Close()

	resp := doProcess(t, ts, protocol.ProcessRequest{
		Flags: protocol.GrepFlags{Pattern: "[bad"},
		Data:  "anything",
	})
	if resp.Error == "" {
		t.Error("want error for bad regex")
	}
}
