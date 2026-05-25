// mygrep-server — один из узлов распределённого grep.
//
// Каждый узел поднимает HTTP-сервер и обрабатывает чанк входных данных,
// который ему передал клиент через POST /process.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"mygrep/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on (host:port or :port)")
	workers := flag.Int("workers", runtime.NumCPU(), "number of worker goroutines per request")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "mygrep-server — узел распределённого grep")
		fmt.Fprintln(os.Stderr, "Usage:")
		flag.PrintDefaults()
	}
	flag.Parse()

	logger := log.New(os.Stderr, "[mygrep-server] ", log.LstdFlags|log.Lmsgprefix)

	mux := server.New(server.Handler{
		Workers: *workers,
		Logger:  logger,
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Корректное завершение по SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("listening on %s (workers=%d)", *addr, *workers)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Println("shutdown requested")
	case err := <-errCh:
		logger.Fatalf("server error: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("shutdown error: %v", err)
	}
}
