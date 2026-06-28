package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// runServe starts the HTTP daemon.
//
// Endpoints:
//
//	POST /webhook   Trigger an immediate sync (optional bearer-token auth)
//	GET  /metrics   Prometheus text-format metrics
//	GET  /healthz   Always 200 OK (liveness probe)
//	GET  /status    JSON snapshot of the last sync result
func runServe(args []string) {
	fs, cfg := newFlagSet("serve")
	addSyncFlags(fs, cfg)
	fs.StringVar(&cfg.ListenAddr, "listen", ":8080", "Address to listen on (host:port)")
	fs.StringVar(&cfg.WebhookSecret, "webhook-secret", "", "If set, require 'Authorization: Bearer <secret>' on /webhook")
	fs.DurationVar(&cfg.WatchInterval, "interval", 0, "Also run periodic syncs at this interval (0 = webhook-only)")
	parseAndMerge(fs, cfg, args)
	requireSyncConfig(cfg)

	// Buffered channel of size 1 — a pending sync absorbs additional triggers.
	syncTrigger := make(chan struct{}, 1)
	enqueue := func() {
		select {
		case syncTrigger <- struct{}{}:
		default: // already queued
		}
	}

	// Background sync worker
	go func() {
		for range syncTrigger {
			runOnce(cfg)
		}
	}()

	// Optional periodic sync
	if cfg.WatchInterval > 0 {
		logInfo("Periodic sync every %s", cfg.WatchInterval)
		go func() {
			ticker := time.NewTicker(cfg.WatchInterval)
			defer ticker.Stop()
			for range ticker.C {
				enqueue()
			}
		}()
	}

	// Initial sync at startup
	enqueue()

	mux := http.NewServeMux()

	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if cfg.WebhookSecret != "" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+cfg.WebhookSecret {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		// Drain body to be polite to the caller
		io.Copy(io.Discard, r.Body)

		enqueue()
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintln(w, "sync queued")
		logInfo("Webhook received — sync queued")
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprint(w, globalMetrics.text())
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(globalMetrics.statusJSON())
	})

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logInfo("Shutdown signal received")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	logInfo("Listening on %s", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		must(err, "HTTP server")
	}
	logInfo("Server stopped")
}
