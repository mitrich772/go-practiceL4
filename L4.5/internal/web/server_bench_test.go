package web

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func BenchmarkStatsHandler_1000(b *testing.B) {
	benchmarkStatsHandler(b, 1000)
}

func BenchmarkStatsHandler_10000(b *testing.B) {
	benchmarkStatsHandler(b, 10000)
}

func benchmarkStatsHandler(b *testing.B, size int) {
	handler := New().Routes()
	body := buildStatsBody(size)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/stats", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d; body = %s", rec.Code, rec.Body.String())
		}
		sinkBody = rec.Body.Len()
	}
}

func buildStatsBody(size int) []byte {
	var b strings.Builder
	b.WriteString(`{"numbers":[`)
	for i := 0; i < size; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%d", i%1000))
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

var sinkBody int
