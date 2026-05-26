package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/pprof"

	"L4.5/internal/stats"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.Health)
	mux.HandleFunc("/stats", s.Stats)
	registerPprof(mux)
	return mux
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "method must be GET")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"result": "ok"})
}

func (s *Server) Stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "method must be POST")
		return
	}

	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	values, ok := raw["numbers"].([]any)
	if !ok {
		writeError(w, http.StatusBadRequest, "numbers must be an array")
		return
	}

	numbers := make([]float64, 0, len(values))
	for _, value := range values {
		number, ok := value.(float64)
		if !ok {
			writeError(w, http.StatusBadRequest, "numbers must contain only numeric values")
			return
		}
		numbers = append(numbers, number)
	}

	result, err := stats.Calculate(numbers)
	if errors.Is(err, stats.ErrEmptyInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count": result.Count,
		"sum":   result.Sum,
		"min":   result.Min,
		"max":   result.Max,
		"avg":   result.Avg,
		"p95":   result.P95,
	})
}

func registerPprof(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}
