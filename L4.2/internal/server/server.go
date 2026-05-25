// Package server реализует HTTP-сервер обработки чанков.
//
// Каждый сервер обрабатывает строки своего чанка параллельно через пул
// воркеров на горутинах с обменом через каналы — что напрямую отражает
// требование задания "использовать каналы и горутины для параллелизма
// внутри каждого сервера".
package server

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"

	"mygrep/internal/grep"
	"mygrep/internal/protocol"
)

// Handler — http.Handler с настраиваемым числом воркеров.
type Handler struct {
	Workers int
	Logger  *log.Logger
}

// New возвращает http.ServeMux с обработчиками /process и /healthz.
func New(h Handler) *http.ServeMux {
	if h.Workers <= 0 {
		h.Workers = runtime.NumCPU()
	}
	if h.Logger == nil {
		h.Logger = log.Default()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/process", h.process)
	mux.HandleFunc("/healthz", h.healthz)
	return mux
}

func (h Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.HealthResponse{Status: "ok"})
}

type job struct {
	idx     int
	lineNum int
	line    string
}

type lineResult struct {
	idx     int
	lineNum int
	text    string
}

func (h Handler) process(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req protocol.ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	matcher, err := grep.NewMatcher(req.Flags)
	if err != nil {
		writeJSON(w, http.StatusOK, protocol.ProcessResponse{
			ChunkID: req.ChunkID,
			Error:   err.Error(),
		})
		return
	}

	// Нормализуем переносы строк и режем на строки.
	var lines []string
	if req.Data != "" {
		lines = strings.Split(strings.ReplaceAll(req.Data, "\r\n", "\n"), "\n")
	}

	// StartLine клиента — это 1-based номер первой строки чанка в исходном
	// файле. Если клиент не выставил — считаем, что это начало (1).
	startLine := req.StartLine
	if startLine <= 0 {
		startLine = 1
	}

	resp := protocol.ProcessResponse{ChunkID: req.ChunkID}

	// Горячий путь для -c: считаем без сборки строк.
	if req.Flags.CountOnly {
		resp.Count = h.countMatches(matcher, lines)
		resp.HasMatch = resp.Count > 0
		writeJSON(w, http.StatusOK, resp)
		return
	}

	results := h.collectMatches(matcher, lines, startLine)

	// Для -l нам важен только сам факт совпадения.
	if req.Flags.ListFiles {
		resp.HasMatch = len(results) > 0
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp.Matches = results
	resp.HasMatch = len(results) > 0
	writeJSON(w, http.StatusOK, resp)
}

// countMatches распараллеливает подсчёт совпадений по пулу воркеров.
func (h Handler) countMatches(m *grep.Matcher, lines []string) int {
	if len(lines) == 0 {
		return 0
	}

	workers := h.Workers
	if workers > len(lines) {
		workers = len(lines)
	}

	jobs := make(chan string, workers*2)
	counts := make(chan int, workers)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := 0
			for line := range jobs {
				if m.Match(line) {
					local++
				}
			}
			counts <- local
		}()
	}

	go func() {
		for _, l := range lines {
			jobs <- l
		}
		close(jobs)
	}()

	wg.Wait()
	close(counts)

	total := 0
	for c := range counts {
		total += c
	}
	return total
}

// collectMatches распараллеливает поиск с сохранением исходного порядка строк.
func (h Handler) collectMatches(m *grep.Matcher, lines []string, startLine int) []protocol.Match {
	if len(lines) == 0 {
		return nil
	}

	workers := h.Workers
	if workers > len(lines) {
		workers = len(lines)
	}

	jobs := make(chan job, workers*2)
	out := make(chan lineResult, workers*2)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if m.Match(j.line) {
					out <- lineResult{idx: j.idx, lineNum: j.lineNum, text: j.line}
				}
			}
		}()
	}

	// Продюсер.
	go func() {
		for i, l := range lines {
			jobs <- job{idx: i, lineNum: startLine + i, line: l}
		}
		close(jobs)
	}()

	// Закрыватель out.
	go func() {
		wg.Wait()
		close(out)
	}()

	var hits []lineResult
	for r := range out {
		hits = append(hits, r)
	}

	// Воркеры могли вернуть совпадения вне порядка, пересортируем по idx
	// чтобы итог соответствовал исходному файлу.
	sort.Slice(hits, func(i, j int) bool { return hits[i].idx < hits[j].idx })

	matches := make([]protocol.Match, len(hits))
	for i, h := range hits {
		matches[i] = protocol.Match{LineNum: h.lineNum, Text: h.text}
	}
	return matches
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
