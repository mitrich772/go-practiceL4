package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"L4.5/internal/stats"
)

func TestStats(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/stats", strings.NewReader(`{"numbers":[1,2,3,4,5]}`))
	rec := httptest.NewRecorder()

	New().Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got stats.Result
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}

	want := stats.Result{Count: 5, Sum: 15, Min: 1, Max: 5, Avg: 3, P95: 5}
	if got != want {
		t.Fatalf("body = %+v, want %+v", got, want)
	}
}

func TestStatsEmptyArray(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/stats", strings.NewReader(`{"numbers":[]}`))
	rec := httptest.NewRecorder()

	New().Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStatsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/stats", strings.NewReader(`{`))
	rec := httptest.NewRecorder()

	New().Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
