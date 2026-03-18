package scanner

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Onder7994/gitlab-access-token-exporter/internal/gitlab"
)

type Record struct {
	Group         string
	ProjectID     string
	ProjectPath   string
	TokenID       string
	TokenName     string
	Active        float64
	Revoked       float64
	HasExpiry     float64
	ExpiresAtUnix float64
	ExpiresInSec  float64
}

type Scanner struct {
	group       string
	concurrency int
	client      *gitlab.Client
}

func New(group string, concurrency int, client *gitlab.Client) *Scanner {
	if concurrency <= 0 {
		concurrency = 5
	}

	return &Scanner{
		group:       group,
		concurrency: concurrency,
		client:      client,
	}
}

func (s *Scanner) Scan(ctx context.Context) ([]Record, error) {
	projects, err := s.client.ListGroupProjects(ctx, s.group, false)
	if err != nil {
		return nil, err
	}

	jobs := make(chan gitlab.Project)
	results := make(chan []Record, len(projects))
	errCh := make(chan error, 1)

	var wg sync.WaitGroup
	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for p := range jobs {
				tokens, err := s.client.ListProjectAccessTokens(ctx, p.ID)
				if err != nil {
					select {
					case errCh <- fmt.Errorf("project %s: %w", p.PathWithNamespace, err):
					default:
					}
					return
				}

				now := time.Now().UTC()
				records := make([]Record, 0, len(tokens))

				for _, t := range tokens {
					r := Record{
						Group:        s.group,
						ProjectID:    strconv.Itoa(p.ID),
						ProjectPath:  p.PathWithNamespace,
						TokenID:      strconv.Itoa(t.ID),
						TokenName:    t.Name,
						Active:       boolToFloat(t.Active),
						Revoked:      boolToFloat(t.Revoked),
						ExpiresInSec: -1,
					}

					if t.ExpiresAt != "" {
						exp, err := time.Parse("2006-01-02", t.ExpiresAt)
						if err == nil {
							exp = exp.UTC()
							r.HasExpiry = 1
							r.ExpiresAtUnix = float64(exp.Unix())
							r.ExpiresInSec = exp.Sub(now).Seconds()
						}
					}

					records = append(records, r)
				}

				results <- records
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, p := range projects {
			select {
			case <-ctx.Done():
				return
			case jobs <- p:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var all []Record
	for {
		select {
		case err := <-errCh:
			if err != nil {
				return nil, err
			}
		case batch, ok := <-results:
			if !ok {
				return all, nil
			}
			all = append(all, batch...)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
