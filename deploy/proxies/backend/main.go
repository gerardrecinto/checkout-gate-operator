// Tiny backend for the proxy lab. /healthz is what the proxies probe, and
// POST /fail makes the instance answer 503 everywhere until POST /recover.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

func newHandler(name string) http.Handler {
	var failing atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("POST /fail", func(w http.ResponseWriter, _ *http.Request) { failing.Store(true) })
	mux.HandleFunc("POST /recover", func(w http.ResponseWriter, _ *http.Request) { failing.Store(false) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			http.Error(w, "failing", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintf(w, "%s\n", name)
	})
	return mux
}

func main() {
	name := os.Getenv("NAME")
	if name == "" {
		name = "backend"
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("backend", name)
	srv := &http.Server{Addr: ":8080", Handler: newHandler(name), ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}
