package exporter

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/Onder7994/gitlab-access-token-exporter/internal/scanner"
	"github.com/prometheus/client_golang/prometheus"
)

type Exporter struct {
	expiresAt *prometheus.GaugeVec
	expiresIn *prometheus.GaugeVec
	active    *prometheus.GaugeVec
	revoked   *prometheus.GaugeVec
	hasExpiry *prometheus.GaugeVec

	lastSuccess  prometheus.Gauge
	lastDuration prometheus.Gauge
	lastError    prometheus.Gauge
	projectsSeen prometheus.Gauge
	tokensSeen   prometheus.Gauge

	ready atomic.Bool
}

func New(reg prometheus.Registerer) *Exporter {
	labels := []string{"group", "project_id", "project_path", "token_id", "token_name"}

	e := &Exporter{
		expiresAt: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gitlab_project_access_token_expires_at_timestamp_seconds",
			Help: "Expiration time of Gitlab project access token as Unix timestamp.",
		}, labels),
		expiresIn: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gitlab_project_access_token_expires_in_seconds",
			Help: "Seconds until Gitlab project access token expiration",
		}, labels),
		active: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gitlab_project_access_token_active",
			Help: "Whether Gitlab project access token is active (1/0).",
		}, labels),
		revoked: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gitlab_project_access_token_revoked",
			Help: "Whether Gitlab project access token is revoked (1/0).",
		}, labels),
		hasExpiry: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gitlab_project_access_token_has_expiry",
			Help: "Whether Gitlab project access token has expiration date (1/0)",
		}, labels),
		lastSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gitlab_token_exporter_last_success_unixtime",
			Help: "Unix time of last successful Gitlab scan.",
		}),
		lastDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gitlab_token_exporter_last_scan_duration_seconds",
			Help: "Duration of last Gitlab scan in seconds",
		}),
		lastError: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gitlab_token_exporter_last_scan_error",
			Help: "Whether last Gitlab scan ended with error (1/0).",
		}),
		projectsSeen: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gitlab_token_exporter_projects_seen",
			Help: "Projects processed during last successful scan.",
		}),
		tokensSeen: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gitlab_token_exporter_tokens_seen",
			Help: "Tokens processed durong last successful scan.",
		}),
	}

	reg.MustRegister(
		e.expiresAt,
		e.expiresIn,
		e.active,
		e.revoked,
		e.hasExpiry,
		e.lastSuccess,
		e.lastDuration,
		e.lastError,
		e.projectsSeen,
		e.tokensSeen,
	)

	return e
}

func (e *Exporter) Ready() bool {
	return e.ready.Load()
}

func (e *Exporter) Run(ctx context.Context, interval time.Duration, s *scanner.Scanner) {
	e.refresh(ctx, s)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.refresh(ctx, s)
		}
	}
}

func (e *Exporter) refresh(ctx context.Context, s *scanner.Scanner) {
	start := time.Now()
	records, err := s.Scan(ctx)
	duration := time.Since(start).Seconds()
	e.lastDuration.Set(duration)

	if err != nil {
		e.lastError.Set(1)
		log.Printf("scan failed: %v", err)
		return
	}

	e.lastError.Set(0)
	e.lastSuccess.SetToCurrentTime()
	e.ready.Store(true) // after first success scan ready will true

	e.expiresAt.Reset()
	e.expiresIn.Reset()
	e.active.Reset()
	e.revoked.Reset()
	e.hasExpiry.Reset()

	projectSet := map[string]struct{}{}
	for _, r := range records {
		lbls := prometheus.Labels{
			"group":        r.Group,
			"project_id":   r.ProjectID,
			"project_path": r.ProjectPath,
			"token_id":     r.TokenID,
			"token_name":   r.TokenName,
		}

		e.active.With(lbls).Set(r.Active)
		e.revoked.With(lbls).Set(r.Revoked)
		e.hasExpiry.With(lbls).Set(r.HasExpiry)

		if r.HasExpiry == 1 {
			e.expiresAt.With(lbls).Set(r.ExpiresAtUnix)
			e.expiresIn.With(lbls).Set(r.ExpiresInSec)
		}

		projectSet[r.ProjectID] = struct{}{}
	}

	e.projectsSeen.Set(float64(len(projectSet)))
	e.tokensSeen.Set(float64(len(records)))
}
