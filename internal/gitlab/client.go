package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type Project struct {
	ID                int    `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
	Name              string `json:"name"`
	WebURL            string `json:"web_url"`
}

type AccessToken struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Active      bool       `json:"active"`
	Revoked     bool       `json:"revoked"`
	ExpiresAt   string     `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	AccessLevel int        `json:"access_level"`
	Scopes      []string   `json:"scopes"`
}

func New(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) WithToken(token string) *Client {
	return &Client{
		baseURL: c.baseURL,
		token:   token,
		http:    c.http,
	}
}

func (c *Client) ListGroupProjects(ctx context.Context, group string, withShared bool) ([]Project, error) {
	var out []Project
	page := 1

	for {
		u := fmt.Sprintf(
			"%s/api/v4/groups/%s/projects?include_subgroups=true&with_shared=%t&simple=true&per_page=100&page=%d",
			c.baseURL,
			url.PathEscape(group),
			withShared,
			page,
		)

		var batch []Project
		if err := c.getJSON(ctx, u, &batch); err != nil {
			return nil, fmt.Errorf("list group projects page=%d: %w", page, err)
		}

		if len(batch) == 0 {
			break
		}

		out = append(out, batch...)
		if len(batch) < 100 {
			break
		}
		page++
	}

	return out, nil
}

func (c *Client) ListProjectAccessTokens(ctx context.Context, projectID int) ([]AccessToken, error) {
	var out []AccessToken
	page := 1

	for {
		u := fmt.Sprintf(
			"%s/api/v4/projects/%d/access_tokens?per_page=100&page=%d",
			c.baseURL,
			projectID,
			page,
		)

		var batch []AccessToken
		if err := c.getJSON(ctx, u, &batch); err != nil {
			return nil, fmt.Errorf("list project access tokens project=%d page=%d: %w", projectID, page, err)
		}

		if len(batch) == 0 {
			break
		}

		out = append(out, batch...)
		if len(batch) < 100 {
			break
		}

		page++
	}

	return out, nil
}

func (c *Client) GetProject(ctx context.Context, idOrPath string) (Project, error) {
	u := fmt.Sprintf(
		"%s/api/v4/projects/%s",
		c.baseURL,
		url.PathEscape(idOrPath),
	)

	var p Project
	if err := c.getJSON(ctx, u, &p); err != nil {
		return Project{}, fmt.Errorf("get project %q: %w", idOrPath, err)
	}

	return p, nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return json.NewDecoder(resp.Body).Decode(dst)
}
