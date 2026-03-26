package scanner

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Onder7994/gitlab-access-token-exporter/internal/config"
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

type projectJob struct {
	project gitlab.Project
	client  *gitlab.Client
}

type Scanner struct {
	group       string
	projects    []config.ProjectConfig
	withShared  bool
	concurrency int
	client      *gitlab.Client
}

func New(group string, projects []config.ProjectConfig, withShared bool, concurrency int, client *gitlab.Client) *Scanner {
	if concurrency <= 0 {
		concurrency = 5
	}

	return &Scanner{
		group:       group,
		projects:    projects,
		withShared:  withShared,
		concurrency: concurrency,
		client:      client,
	}
}

func (s *Scanner) Scan(ctx context.Context) ([]Record, error) {
	type sourceResult struct {
		jobs []projectJob
		err  error
	}

	groupCh := make(chan sourceResult, 1)
	projectsCh := make(chan sourceResult, 1)

	if s.group != "" {
		go func() {
			projects, err := s.client.ListGroupProjects(ctx, s.group, s.withShared)
			if err != nil {
				groupCh <- sourceResult{err: err}
				return
			}
			jobs := make([]projectJob, 0, len(projects))
			for _, p := range projects {
				jobs = append(jobs, projectJob{project: p, client: s.client})
			}
			groupCh <- sourceResult{jobs: jobs}
		}()
	} else {
		groupCh <- sourceResult{}
	}

	if len(s.projects) > 0 {
		go func() {
			jobs, err := s.resolveProjects(ctx)
			projectsCh <- sourceResult{jobs: jobs, err: err}
		}()
	} else {
		projectsCh <- sourceResult{}
	}

	groupResult := <-groupCh
	if groupResult.err != nil {
		return nil, fmt.Errorf("list group projects: %w", groupResult.err)
	}

	projectsResult := <-projectsCh
	if projectsResult.err != nil {
		return nil, fmt.Errorf("resolve explicit projects: %w", projectsResult.err)
	}

	jobMap := make(map[int]projectJob, len(groupResult.jobs)+len(projectsResult.jobs))
	for _, j := range groupResult.jobs {
		jobMap[j.project.ID] = j
	}
	for _, j := range projectsResult.jobs {
		jobMap[j.project.ID] = j
	}

	merged := make([]projectJob, 0, len(jobMap))
	for _, j := range jobMap {
		merged = append(merged, j)
	}

	return s.collectTokens(ctx, merged)
}

func (s *Scanner) resolveProjects(ctx context.Context) ([]projectJob, error) {
	type result struct {
		job projectJob
		err error
	}

	results := make(chan result, len(s.projects))
	sem := make(chan struct{}, s.concurrency)

	var wg sync.WaitGroup
	for _, pc := range s.projects {
		pc := pc
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// choise token
			cl := s.client
			if pc.Token != "" {
				cl = s.client.WithToken(pc.Token)
			}

			proj, err := cl.GetProject(ctx, pc.Path)
			if err != nil {
				results <- result{err: fmt.Errorf("project %q: %w", pc.Path, err)}
				return
			}

			results <- result{job: projectJob{project: proj, client: cl}}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var jobs []projectJob
	for r := range results {
		if r.err != nil {
			return nil, r.err
		}
		jobs = append(jobs, r.job)
	}

	return jobs, nil
}

func (s *Scanner) collectTokens(ctx context.Context, jobs []projectJob) ([]Record, error) {
	type jobCh = chan projectJob

	jobsCh := make(jobCh, len(jobs))
	results := make(chan []Record, len(jobs))
	errCh := make(chan error, 1)

	var wg sync.WaitGroup
	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobsCh {
				// usage client connect for specific project
				tokens, err := j.client.ListProjectAccessTokens(ctx, j.project.ID)
				if err != nil {
					select {
					case errCh <- fmt.Errorf("project %s: %w", j.project.PathWithNamespace, err):
					default:
					}
					return
				}

				now := time.Now().UTC()
				records := make([]Record, 0, len(tokens))
				for _, t := range tokens {
					r := Record{
						Group:        projectNamespace(j.project.PathWithNamespace),
						ProjectID:    strconv.Itoa(j.project.ID),
						ProjectPath:  j.project.PathWithNamespace,
						TokenID:      strconv.Itoa(t.ID),
						TokenName:    t.Name,
						Active:       boolToFloat(t.Active),
						Revoked:      boolToFloat(t.Revoked),
						ExpiresInSec: -1,
					}

					if t.ExpiresAt != "" {
						if exp, err := time.Parse("2006-01-02", t.ExpiresAt); err == nil {
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
		defer close(jobsCh)
		for _, j := range jobs {
			select {
			case <-ctx.Done():
				return
			case jobsCh <- j:
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

func projectNamespace(PathWithNamespace string) string {
	i := strings.LastIndex(PathWithNamespace, "/")
	if i < 0 {
		return PathWithNamespace
	}
	return PathWithNamespace[:i]
}

func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
