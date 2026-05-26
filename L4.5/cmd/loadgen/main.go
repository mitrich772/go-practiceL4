package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	url := flag.String("url", "http://localhost:8080/stats", "stats endpoint URL")
	requests := flag.Int("requests", 1000, "total request count")
	concurrency := flag.Int("concurrency", 8, "parallel workers")
	size := flag.Int("size", 1000, "numbers per request")
	flag.Parse()

	body := buildBody(*size)
	client := &http.Client{Timeout: 10 * time.Second}

	start := time.Now()
	jobs := make(chan int)
	var wg sync.WaitGroup
	var okCount int64

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				resp, err := client.Post(*url, "application/json", bytes.NewReader(body))
				if err != nil {
					log.Printf("request failed: %v", err)
					continue
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					atomic.AddInt64(&okCount, 1)
				}
			}
		}()
	}

	for i := 0; i < *requests; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	elapsed := time.Since(start)
	fmt.Printf("requests=%d ok=%d elapsed=%s rps=%.2f\n", *requests, okCount, elapsed, float64(*requests)/elapsed.Seconds())
}

func buildBody(size int) []byte {
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
