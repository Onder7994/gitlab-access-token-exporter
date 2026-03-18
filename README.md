# gitlab-access-token-exporter

An exporter that iterates through all projects and subprojects in a specified group, discovers access tokens, and exposes metrics for them.

### Supported metrics and labels

`gitlab_project_access_token_expires_at_timestamp_seconds` - Unix timestamp of the token expiration date.

`gitlab_project_access_token_expires_in_seconds` - Number of seconds remaining until the token expires.

`gitlab_project_access_token_active` - Token active flag. `1` means the token is active, `0` means it is inactive.

`gitlab_project_access_token_revoked` - Token revoked flag. `1` means the token is revoked, `0` means it is not revoked.

`gitlab_project_access_token_has_expiry` - Indicates whether the token has an expiration date. `1` means an expiration date is set, `0` means it is not set.

`gitlab_token_exporter_last_success_unixtime` - Unix timestamp of the last successful GitLab scan.

`gitlab_token_exporter_last_scan_duration_seconds` - Duration of the last GitLab scan.

`gitlab_token_exporter_last_scan_error` - Status of the last GitLab scan. `1` means the scan failed, `0` means it succeeded.

`gitlab_token_exporter_projects_seen` - Number of projects processed during the last successful scan.

`gitlab_token_exporter_tokens_seen` - Number of access tokens processed during the last successful scan.

The following labels are added:

1. `group` - GitLab group path, for example `security/soc`.
2. `project_id` - Project ID.
3. `project_path` - Full project path, for example `security/soc/<project_name>`.
4. `token_id` - Access token ID.
5. `token_name` - Token name.

### Exporter configuration

The configuration file is located in the project root: `config.yaml`.

Configuration values can also be overridden via environment variables. Priority is as follows:

1. Default values in the code.
2. Values from `config.yaml`.
3. Values from environment variables.

GitLab settings:

1. `gitlab.base_url` or env `GITLAB_BASE_URL` - Base GitLab URL, for example `https://gitlab.example.com`.
2. `gitlab.token` or env `GITLAB_TOKEN` - Access token.
3. `gitlab.group` or env `GITLAB_GROUP` - Group path to scan.
4. `gitlab.with_shared` or env `GITLAB_WITH_SHARED` - Whether to include shared projects in the scan.

HTTP settings:

1. `http.timeout_seconds` or env `HTTP_TIMEOUT_SECONDS` - Timeout for HTTP requests to GitLab.

Scan settings:

1. `scan.interval_seconds` or env `SCAN_INTERVAL_SECONDS` - Interval for background GitLab scans.
2. `scan.concurrency` or env `SCAN_CONCURRENCY` - Number of concurrent requests to projects.

Metrics settings:

1. `metrics.listen` or env `METRICS_LISTEN` - Address and port for the exporter's HTTP server.
2. `metrics.path` or env `METRICS_PATH` - HTTP path for metrics.

### Local run

```bash
export GITLAB_TOKEN=<token>
go run ./cmd/gitlab-access-token-exporter -config config.yaml
```

Check metrics:
```bash
curl http://127.0.0.1:9108/metrics
```

Docker build:
```bash
docker build -t gitlab-access-token-exporter .
```