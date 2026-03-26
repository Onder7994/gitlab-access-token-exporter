package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Onder7994/gitlab-access-token-exporter/internal/config"
	"github.com/Onder7994/gitlab-access-token-exporter/internal/exporter"
	"github.com/Onder7994/gitlab-access-token-exporter/internal/gitlab"
	"github.com/Onder7994/gitlab-access-token-exporter/internal/scanner"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to yaml config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	gl := gitlab.New(
		cfg.GitLab.BaseURL,
		cfg.GitLab.Token,
		time.Duration(cfg.HTTP.TimeoutSeconds)*time.Second,
	)

	s := scanner.New(
		cfg.GitLab.Group,
		cfg.GitLab.Projects,
		cfg.GitLab.WithShared,
		cfg.Scan.Concurrency,
		gl,
	)

	reg := prometheus.NewRegistry()
	exp := exporter.New(reg)

	go exp.Run(ctx, time.Duration(cfg.Scan.IntervalSeconds)*time.Second, s)

	mux := http.NewServeMux()
	mux.Handle(cfg.Metrics.Path, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", handleLiveness)
	mux.HandleFunc("/readyz", handleReadiness(exp))

	srv := &http.Server{
		Addr:    cfg.Metrics.Listen,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("listening on %s%s", cfg.Metrics.Listen, cfg.Metrics.Path)
	log.Printf("liveness: /healthz")
	log.Printf("readiness: /readyz")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server: %v", err)
	}
}

func handleLiveness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

func handleReadiness(exp *exporter.Exporter) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !exp.Ready() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "not ready",
				"reason": "waiting for first successful scan",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}
}
