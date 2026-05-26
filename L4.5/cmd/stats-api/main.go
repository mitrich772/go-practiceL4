package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"L4.5/internal/web"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "HTTP listen address")
	flag.Parse()

	server := &http.Server{
		Addr:              *addr,
		Handler:           web.New().Routes(),
		ReadHeaderTimeout: 3 * time.Second,
	}

	log.Printf("stats API listening on http://%s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
