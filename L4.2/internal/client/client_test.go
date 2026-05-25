package client

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mygrep/internal/protocol"
	"mygrep/internal/server"
)

func startServers(t *testing.T, n int) []string {
	t.Helper()
	addrs := make([]string, n)
	for i := 0; i < n; i++ {
		ts := httptest.NewServer(server.New(server.Handler{Workers: 2}))
		t.Cleanup(ts.Close)
		// httptest.NewServer возвращает URL вида http://127.0.0.1:PORT
		addrs[i] = strings.TrimPrefix(ts.URL, "http://")
	}
	return addrs
}

func TestRun_QuorumMetAndOrdered(t *testing.T) {
	servers := startServers(t, 3)

	input := strings.Join([]string{
		"alpha bravo",
		"charlie ERROR delta",
		"echo foxtrot",
		"golf ERROR hotel",
		"india juliet",
		"kilo lima",
		"mike november ERROR",
		"oscar papa",
		"quebec ERROR romeo",
	}, "\n") + "\n"

	res, err := Run(context.Background(), Config{Servers: servers, Timeout: 5 * time.Second},
		protocol.GrepFlags{Pattern: "ERROR", FixedString: true, PrintLineNum: true},
		"", input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.QuorumMet {
		t.Fatalf("quorum not met: %+v", res)
	}
	if res.Successes != 3 {
		t.Errorf("successes: got %d, want 3", res.Successes)
	}
	if len(res.Matches) != 4 {
		t.Fatalf("matches: got %d, want 4: %+v", len(res.Matches), res.Matches)
	}
	wantLines := []int{2, 4, 7, 9}
	for i, w := range wantLines {
		if res.Matches[i].LineNum != w {
			t.Errorf("match %d line: got %d, want %d", i, res.Matches[i].LineNum, w)
		}
	}
}

func TestRun_CountOnly(t *testing.T) {
	servers := startServers(t, 4)
	input := "x\ny\nx\nz\nx\nx\ny\n"

	res, err := Run(context.Background(), Config{Servers: servers, Timeout: 5 * time.Second},
		protocol.GrepFlags{Pattern: "x", FixedString: true, CountOnly: true},
		"", input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Count != 4 {
		t.Errorf("count: got %d, want 4", res.Count)
	}
}

func TestRun_QuorumNotMet(t *testing.T) {
	// Один живой, два мёртвых сервера; quorum = 2; не достигнем.
	live := startServers(t, 1)
	dead := []string{"127.0.0.1:1", "127.0.0.1:2"} // невалидные адреса

	cfg := Config{
		Servers: append(live, dead...),
		Quorum:  2,
		Timeout: 1 * time.Second,
	}
	res, err := Run(context.Background(), cfg,
		protocol.GrepFlags{Pattern: "x", FixedString: true},
		"", "x\ny\nz")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if res.QuorumMet {
		t.Errorf("expected quorum NOT met: %+v", res)
	}
	if res.Failures != 2 {
		t.Errorf("failures: got %d, want 2", res.Failures)
	}
}

func TestRun_InvalidConfig(t *testing.T) {
	_, err := Run(context.Background(), Config{}, protocol.GrepFlags{Pattern: "x"}, "", "")
	if err == nil {
		t.Error("want error for empty server list")
	}

	servers := startServers(t, 2)
	_, err = Run(context.Background(), Config{Servers: servers, Quorum: 5},
		protocol.GrepFlags{Pattern: "x"}, "", "")
	if err == nil {
		t.Error("want error for quorum > servers")
	}
}

func TestRun_BadRegexBubblesUp(t *testing.T) {
	servers := startServers(t, 3)
	res, err := Run(context.Background(), Config{Servers: servers, Timeout: 2 * time.Second},
		protocol.GrepFlags{Pattern: "[bad"},
		"", "anything")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Successes != 0 {
		t.Errorf("successes: got %d, want 0", res.Successes)
	}
	if len(res.ErrorsList) == 0 {
		t.Error("want bad-regex errors collected")
	}
}

// Имитирует docker compose сценарий: сервера начинают слушать с задержкой.
// Клиент с ConnectRetry должен дождаться их и собрать кворум.
func TestRun_ConnectRetryWaitsForLateStart(t *testing.T) {
	// Подбираем свободный порт, биндим listener и сразу закрываем —
	// гарантия, что порт пока никто не слушает. Через 300мс запускаем сервер.
	pickPort := func() string {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("pickPort: %v", err)
		}
		addr := l.Addr().String()
		l.Close()
		return addr
	}
	addr := pickPort()

	// Стартуем сервер с задержкой.
	go func() {
		time.Sleep(300 * time.Millisecond)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		s := &http.Server{Handler: server.New(server.Handler{Workers: 2})}
		go func() { _ = s.Serve(l) }()
		t.Cleanup(func() { _ = s.Close() })
	}()

	cfg := Config{
		Servers:      []string{addr},
		Timeout:      5 * time.Second,
		ConnectRetry: 2 * time.Second,
	}
	res, err := Run(context.Background(), cfg,
		protocol.GrepFlags{Pattern: "x", FixedString: true, CountOnly: true},
		"", "x\ny\nx\n")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.QuorumMet {
		t.Fatalf("expected quorum met after retry: %+v", res)
	}
	if res.Count != 2 {
		t.Errorf("count: got %d, want 2", res.Count)
	}
}

// Убеждаемся, что Run корректно работает, даже когда все сервера временно
// возвращают 500 — это эквивалентно полному отказу инфраструктуры.
func TestRun_AllFail(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/process", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	res, err := Run(context.Background(), Config{
		Servers: []string{strings.TrimPrefix(ts.URL, "http://"), strings.TrimPrefix(ts.URL, "http://")},
		Timeout: 2 * time.Second,
	}, protocol.GrepFlags{Pattern: "x", FixedString: true}, "", "x\ny")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.QuorumMet {
		t.Errorf("expected quorum not met")
	}
	if res.Failures != 2 {
		t.Errorf("failures: got %d, want 2", res.Failures)
	}
}
