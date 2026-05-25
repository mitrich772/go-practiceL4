package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"mygrep/internal/chunk"
	"mygrep/internal/protocol"
)

// Config описывает параметры распределённого запуска.
type Config struct {
	Servers []string      // адреса серверов host:port
	Quorum  int           // минимальное число успешных ответов; <=0 — N/2+1
	Timeout time.Duration // таймаут на один HTTP-вызов
	Client  *http.Client  // опционально: HTTP-клиент (для тестов)
}

// Result — агрегированный результат распределённого запроса.
type Result struct {
	Matches    []protocol.Match // итог в порядке исходного файла
	Count      int              // суммарный счётчик для -c
	HasMatch   bool             // был ли хотя бы один матч на любом сервере
	Successes  int              // число успешных ответов
	Failures   int              // число неуспешных серверов
	Quorum     int              // целевой кворум
	QuorumMet  bool             // достигнут ли кворум
	Servers    int              // общее число серверов
	ErrorsList []string         // тексты ошибок по серверам, для отладки
}

// Run распределённо ищет flags.Pattern в input.
//
// fileName используется только для оформления вывода при флаге -l
// (печатать имя файла, если есть совпадения).
func Run(ctx context.Context, cfg Config, flags protocol.GrepFlags, fileName, input string) (Result, error) {
	n := len(cfg.Servers)
	if n == 0 {
		return Result{}, errors.New("no servers configured")
	}

	quorum := cfg.Quorum
	if quorum <= 0 {
		quorum = n/2 + 1
	}
	if quorum > n {
		return Result{}, fmt.Errorf("quorum (%d) cannot exceed number of servers (%d)", quorum, n)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	httpClient := cfg.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	chunks := chunk.Split(input, n)

	type chanResult struct {
		idx    int
		server string
		resp   protocol.ProcessResponse
		err    error
	}
	resCh := make(chan chanResult, n)

	// Контекст с дедлайном гарантирует, что отстающие горутины не будут жить
	// дольше окна ожидания, даже если http.Client тайм-аут не сработает.
	dispatchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int, addr string, c chunk.Chunk) {
			defer wg.Done()
			req := protocol.ProcessRequest{
				ChunkID:   idx,
				StartLine: c.StartLine,
				FileName:  fileName,
				Flags:     flags,
				Data:      c.Data,
			}
			resp, err := postProcess(dispatchCtx, httpClient, addr, req)
			select {
			case resCh <- chanResult{idx: idx, server: addr, resp: resp, err: err}:
			case <-dispatchCtx.Done():
			}
		}(i, strings.TrimSpace(cfg.Servers[i]), chunks[i])
	}

	// Финализатор: дождётся всех воркеров и закроет канал.
	go func() {
		wg.Wait()
		close(resCh)
	}()

	result := Result{
		Servers: n,
		Quorum:  quorum,
	}

	// Соберём успешные ответы по индексу чанка.
	successByChunk := make(map[int]protocol.ProcessResponse, n)

	for r := range resCh {
		if r.err != nil {
			result.Failures++
			result.ErrorsList = append(result.ErrorsList,
				fmt.Sprintf("%s: %v", r.server, r.err))
			continue
		}
		if r.resp.Error != "" {
			result.Failures++
			result.ErrorsList = append(result.ErrorsList,
				fmt.Sprintf("%s: %s", r.server, r.resp.Error))
			continue
		}
		// Дублирующие ответы по одному chunk_id игнорируем (на случай
		// будущих расширений с репликацией заданий).
		if _, exists := successByChunk[r.idx]; !exists {
			successByChunk[r.idx] = r.resp
			result.Successes++
		}
	}

	result.QuorumMet = result.Successes >= quorum

	// Соберём ответы в порядке chunk_id.
	ids := make([]int, 0, len(successByChunk))
	for id := range successByChunk {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		resp := successByChunk[id]
		if resp.HasMatch {
			result.HasMatch = true
		}
		if flags.CountOnly {
			result.Count += resp.Count
			continue
		}
		result.Matches = append(result.Matches, resp.Matches...)
	}

	return result, nil
}

func postProcess(ctx context.Context, c *http.Client, addr string, req protocol.ProcessRequest) (protocol.ProcessResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return protocol.ProcessResponse{}, fmt.Errorf("marshal: %w", err)
	}
	url := "http://" + addr + "/process"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return protocol.ProcessResponse{}, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(httpReq)
	if err != nil {
		return protocol.ProcessResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return protocol.ProcessResponse{}, fmt.Errorf("status %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	var out protocol.ProcessResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return protocol.ProcessResponse{}, fmt.Errorf("decode: %w", err)
	}
	return out, nil
}
