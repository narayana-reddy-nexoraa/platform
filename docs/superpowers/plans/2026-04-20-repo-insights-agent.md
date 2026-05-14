# Repo Insights Agent Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go-based repo-insights service for the `platform` repo that posts weekday digests and mention-triggered reports into a dedicated Teams channel using local git plus GitHub activity.

**Architecture:** Add a standalone `cmd/repo-insights` binary backed by Postgres state, deterministic collectors, weighted scoring, and a Teams publisher. Use Microsoft Graph polling for channel mentions instead of webhook-first bot delivery so v1 can run as a simple long-lived service in this repo without a separate ingress flow.

**Tech Stack:** Go 1.25, pgx/sqlc/Postgres, `net/http`, zerolog, Prometheus, Git CLI, GitHub REST API, Microsoft Graph REST API

---

## File Map

- `cmd/repo-insights/main.go`
  Boots the repo-insights process, database pool, metrics server, scheduler, and Teams poller.
- `internal/config/config.go`
  Extends shared env-backed configuration with repo-insights settings.
- `internal/repoinsights/types.go`
  Shared request, window, snapshot, finding, and report types.
- `internal/repoinsights/service.go`
  High-level orchestration contracts and default window helpers.
- `internal/repoinsights/orchestrator.go`
  Runs one scheduled or on-demand insight cycle end-to-end.
- `internal/repoinsights/collectors/git.go`
  Uses local git commands to build commit/churn/hotspot signals.
- `internal/repoinsights/collectors/github.go`
  Pulls PR, review, issue, and check-run signals from GitHub.
- `internal/repoinsights/scoring/engine.go`
  Converts raw signals into ranked progress, risk, and team-activity findings.
- `internal/repoinsights/reporting/formatter.go`
  Produces the stable Teams message layout.
- `internal/repoinsights/reporting/openai.go`
  Optional LLM narrator with deterministic fallback when no API key is configured.
- `internal/repoinsights/teams/client.go`
  Exchanges Graph credentials, lists channel messages, posts messages, and replies in thread.
- `internal/repoinsights/teams/parser.go`
  Parses mention text into time window and focus filters.
- `internal/repoinsights/runtime/scheduler.go`
  Computes weekday cutoffs and fires the scheduled digest loop.
- `internal/repoinsights/runtime/poller.go`
  Polls Teams for fresh mention events and advances the saved cursor.
- `internal/metrics/metrics.go`
  Adds counters and histograms for repo-insights runs and collector failures.
- `db/migrations/000014_create_repo_insights_state.{up,down}.sql`
  Creates run-history and cursor tables.
- `db/queries/repo_insights.sql`
  sqlc queries for run and cursor state.
- `internal/repository/repo_insights_repo.go`
  Postgres/sqlc-backed state repository for run deduplication and cursor tracking.
- `tests/integration/repo_insights_repo_test.go`
  Integration coverage for the new state repository.
- `Makefile`, `Dockerfile`, `docker-compose.yml`
  Build and local-run wiring for the new binary.
- `docs/repo-insights.md`
  Operator notes for env vars, runtime behavior, and Teams/GitHub setup.

## Task 1: Add Repo-Insights Config and Binary Skeleton

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/config_repo_insights_test.go`
- Create: `internal/repoinsights/types.go`
- Create: `internal/repoinsights/service.go`
- Create: `internal/repoinsights/service_test.go`
- Create: `cmd/repo-insights/main.go`

- [ ] **Step 1: Write the failing config and window tests**

```go
package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad_RepoInsightsDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("REPO_INSIGHTS_ENABLED", "true")

	cfg, err := Load()
	require.NoError(t, err)
	require.True(t, cfg.RepoInsightsEnabled)
	require.Equal(t, "/Users/narayana-nexoraa/Developer/HSD/platform", cfg.RepoInsightsRepoPath)
	require.Equal(t, "narayana-reddy-nexoraa", cfg.RepoInsightsGitHubOwner)
	require.Equal(t, "platform", cfg.RepoInsightsGitHubRepo)
	require.Equal(t, "Asia/Kolkata", cfg.RepoInsightsTimezone)
	require.Equal(t, 7*24*time.Hour, cfg.RepoInsightsDefaultLookback)
}
```

```go
package repoinsights

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultOnDemandWindow_UsesConfiguredLookback(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	svc := Service{now: func() time.Time { return now }, defaultLookback: 7 * 24 * time.Hour}

	window := svc.DefaultOnDemandWindow()
	require.Equal(t, now.Add(-7*24*time.Hour), window.Start)
	require.Equal(t, now, window.End)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config ./internal/repoinsights -run 'TestLoad_RepoInsightsDefaults|TestDefaultOnDemandWindow_UsesConfiguredLookback' -count=1`
Expected: FAIL with unknown repo-insights config fields or missing `internal/repoinsights` package.

- [ ] **Step 3: Write the minimal config, types, and binary skeleton**

```go
type Config struct {
	RepoInsightsEnabled          bool
	RepoInsightsRepoPath         string
	RepoInsightsGitHubOwner      string
	RepoInsightsGitHubRepo       string
	RepoInsightsGitHubToken      string
	RepoInsightsGraphTenantID    string
	RepoInsightsGraphClientID    string
	RepoInsightsGraphClientSecret string
	RepoInsightsTeamsTeamID      string
	RepoInsightsTeamsChannelID   string
	RepoInsightsTeamsBotName     string
	RepoInsightsTimezone         string
	RepoInsightsScheduleHour     int
	RepoInsightsScheduleMinute   int
	RepoInsightsMentionPollInterval time.Duration
	RepoInsightsDefaultLookback  time.Duration
	RepoInsightsOpenAIBaseURL    string
	RepoInsightsOpenAIAPIKey     string
	RepoInsightsOpenAIModel      string
}
```

```go
package repoinsights

import "time"

type Window struct {
	Start time.Time
	End   time.Time
}

type TriggerKind string

const (
	TriggerScheduled TriggerKind = "scheduled"
	TriggerMention   TriggerKind = "mention"
)

type ReportRequest struct {
	Trigger        TriggerKind
	Window         Window
	FocusArea      string
	ReplyToMessage string
}

type CheckRun struct {
	Name       string
	Conclusion string
}

type GitSnapshot struct {
	Commits        []string
	HotDirectories []string
	TotalAdditions int
}

type GitHubSnapshot struct {
	PullRequests  []string
	FailingChecks []CheckRun
}

type ActivitySnapshot struct {
	Git       *GitSnapshot
	GitHub    *GitHubSnapshot
	FocusArea string
}

type Finding struct {
	Score   int
	Summary string
}

type ScoredReport struct {
	ExecutiveSummary []Finding
	EngineeringRisk  []Finding
	TeamActivity     []Finding
}

type RenderInput struct {
	Headline         string
	ExecutiveSummary []string
	EngineeringRisk  []string
	TeamActivity     []string
}

type Service struct {
	now             func() time.Time
	defaultLookback time.Duration
}

func (s Service) DefaultOnDemandWindow() Window {
	end := s.now()
	return Window{Start: end.Add(-s.defaultLookback), End: end}
}
```

```go
func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("component", "repo-insights").Logger()
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	if !cfg.RepoInsightsEnabled {
		logger.Info().Msg("repo insights disabled")
		return
	}

	logger.Info().
		Str("repo_path", cfg.RepoInsightsRepoPath).
		Str("github_repo", cfg.RepoInsightsGitHubOwner+"/"+cfg.RepoInsightsGitHubRepo).
		Msg("repo-insights bootstrap complete")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config ./internal/repoinsights -run 'TestLoad_RepoInsightsDefaults|TestDefaultOnDemandWindow_UsesConfiguredLookback' -count=1`
Expected: PASS

- [ ] **Step 5: Build the new binary**

Run: `go build ./cmd/repo-insights`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_repo_insights_test.go internal/repoinsights/types.go internal/repoinsights/service.go internal/repoinsights/service_test.go cmd/repo-insights/main.go
git commit -m "feat: add repo insights config and binary skeleton"
```

### Task 2: Add Repo-Insights State Tables and Repository

**Files:**
- Create: `db/migrations/000014_create_repo_insights_state.up.sql`
- Create: `db/migrations/000014_create_repo_insights_state.down.sql`
- Create: `db/queries/repo_insights.sql`
- Create: `internal/repository/repo_insights_repo.go`
- Create: `tests/integration/repo_insights_repo_test.go`
- Modify: `tests/integration/setup_test.go`

- [ ] **Step 1: Write the failing repository integration test**

```go
func TestRepoInsightsStateRepository_StartRunAndCursorRoundTrip(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, truncateRepoInsightsTables(ctx))

	repo := repository.NewPostgresRepoInsightsRepository(testPool)
	window := repoinsights.Window{
		Start: time.Date(2026, 4, 19, 3, 30, 0, 0, time.UTC),
		End:   time.Date(2026, 4, 20, 3, 30, 0, 0, time.UTC),
	}

	run, err := repo.StartRun(ctx, repository.StartRepoInsightsRunParams{
		Trigger:  string(repoinsights.TriggerScheduled),
		DedupKey: "scheduled:2026-04-20",
		Window:   window,
	})
	require.NoError(t, err)
	require.Equal(t, "scheduled:2026-04-20", run.DedupKey)

	require.NoError(t, repo.SaveCursor(ctx, "teams-channel", "2026-04-20T03:30:00Z"))
	cursor, err := repo.LoadCursor(ctx, "teams-channel")
	require.NoError(t, err)
	require.Equal(t, "2026-04-20T03:30:00Z", cursor)
}

func truncateRepoInsightsTables(ctx context.Context) error {
	_, err := testPool.Exec(ctx, "TRUNCATE TABLE repo_insights_runs, repo_insights_cursors")
	return err
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/integration/... -run TestRepoInsightsStateRepository_StartRunAndCursorRoundTrip -count=1`
Expected: FAIL with missing migrations, sqlc queries, or repository constructor.

- [ ] **Step 3: Add migration, sqlc queries, and repository implementation**

```sql
CREATE TABLE repo_insights_runs (
    run_id UUID PRIMARY KEY,
    trigger_type VARCHAR(32) NOT NULL,
    dedup_key VARCHAR(255) NOT NULL UNIQUE,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    focus_area VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL,
    teams_thread_id TEXT,
    posted_message_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE repo_insights_cursors (
    consumer_name VARCHAR(128) PRIMARY KEY,
    cursor_value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

```sql
-- name: InsertRepoInsightsRun :one
INSERT INTO repo_insights_runs (
    run_id, trigger_type, dedup_key, window_start, window_end, focus_area, status
) VALUES ($1, $2, $3, $4, $5, $6, 'started')
RETURNING *;

-- name: CompleteRepoInsightsRun :one
UPDATE repo_insights_runs
SET status = $2, teams_thread_id = $3, posted_message_id = $4, completed_at = NOW()
WHERE run_id = $1
RETURNING *;

-- name: RepoInsightsDedupExists :one
SELECT EXISTS(SELECT 1 FROM repo_insights_runs WHERE dedup_key = $1);

-- name: GetRepoInsightsCursor :one
SELECT * FROM repo_insights_cursors WHERE consumer_name = $1;

-- name: UpsertRepoInsightsCursor :exec
INSERT INTO repo_insights_cursors (consumer_name, cursor_value, updated_at)
VALUES ($1, $2, NOW())
ON CONFLICT (consumer_name) DO UPDATE
SET cursor_value = EXCLUDED.cursor_value, updated_at = NOW();
```

```go
type RepoInsightsStateRepository interface {
	StartRun(ctx context.Context, params StartRepoInsightsRunParams) (*RepoInsightsRun, error)
	CompleteRun(ctx context.Context, runID uuid.UUID, status, threadID, messageID string) error
	DedupExists(ctx context.Context, dedupKey string) (bool, error)
	LoadCursor(ctx context.Context, consumerName string) (string, error)
	SaveCursor(ctx context.Context, consumerName, cursor string) error
}
```

- [ ] **Step 4: Regenerate sqlc and run the integration test**

Run: `sqlc generate && go test ./tests/integration/... -run TestRepoInsightsStateRepository_StartRunAndCursorRoundTrip -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add db/migrations/000014_create_repo_insights_state.up.sql db/migrations/000014_create_repo_insights_state.down.sql db/queries/repo_insights.sql internal/repository/db internal/repository/repo_insights_repo.go tests/integration/setup_test.go tests/integration/repo_insights_repo_test.go
git commit -m "feat: add repo insights state repository"
```

### Task 3: Implement the Local Git Collector

**Files:**
- Create: `internal/repoinsights/collectors/git.go`
- Create: `internal/repoinsights/collectors/git_test.go`

- [ ] **Step 1: Write the failing git collector test**

```go
func TestGitCollector_CollectsCommitChurnAndHotspots(t *testing.T) {
	repoDir := t.TempDir()
	runGit := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		require.NoError(t, cmd.Run())
	}
	writeTrackedFile := func(rel, body string) {
		full := filepath.Join(repoDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o644))
	}

	runGit("init")
	runGit("config", "user.name", "Repo Test")
	runGit("config", "user.email", "repo@test.dev")
	writeTrackedFile("internal/service/a.go", "package service\n")
	runGit("add", ".")
	runGit("commit", "-m", "feat: add service file")

	writeTrackedFile("terraform/main.tf", "resource \"null_resource\" \"x\" {}\n")
	runGit("add", ".")
	runGit("commit", "-m", "infra: add terraform")

	collector := collectors.NewGitCollector("git")
	window := repoinsights.Window{Start: time.Now().Add(-24 * time.Hour), End: time.Now().Add(1 * time.Hour)}

	snapshot, err := collector.Collect(context.Background(), repoDir, window)
	require.NoError(t, err)
	require.Len(t, snapshot.Commits, 2)
	require.Contains(t, snapshot.HotDirectories, "internal/service")
	require.Contains(t, snapshot.HotDirectories, "terraform")
	require.Greater(t, snapshot.TotalAdditions, 0)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repoinsights/collectors -run TestGitCollector_CollectsCommitChurnAndHotspots -count=1`
Expected: FAIL with missing collector.

- [ ] **Step 3: Implement the collector**

```go
type GitCollector struct {
	gitBin string
}

func NewGitCollector(gitBin string) *GitCollector {
	return &GitCollector{gitBin: gitBin}
}

func (c *GitCollector) Collect(ctx context.Context, repoPath string, window repoinsights.Window) (*repoinsights.GitSnapshot, error) {
	cmd := exec.CommandContext(ctx, c.gitBin,
		"-C", repoPath,
		"log",
		"--since", window.Start.Format(time.RFC3339),
		"--until", window.End.Format(time.RFC3339),
		"--numstat",
		"--format=%H|%an|%ad|%s",
		"--date=iso-strict",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return parseGitLog(out), nil
}
```

- [ ] **Step 4: Run the collector test**

Run: `go test ./internal/repoinsights/collectors -run TestGitCollector_CollectsCommitChurnAndHotspots -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/repoinsights/collectors/git.go internal/repoinsights/collectors/git_test.go
git commit -m "feat: add local git activity collector"
```

### Task 4: Add GitHub and Teams Clients Plus Mention Parsing

**Files:**
- Create: `internal/repoinsights/collectors/github.go`
- Create: `internal/repoinsights/collectors/github_test.go`
- Create: `internal/repoinsights/teams/client.go`
- Create: `internal/repoinsights/teams/client_test.go`
- Create: `internal/repoinsights/teams/parser.go`
- Create: `internal/repoinsights/teams/parser_test.go`

- [ ] **Step 1: Write failing client and parser tests**

```go
func TestGitHubCollector_CollectsPRsReviewsAndChecks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/pulls"):
			io.WriteString(w, `[{"number":42,"title":"feat: repo insights","draft":false,"state":"open","requested_reviewers":[{"login":"alice"}]}]`)
		case strings.Contains(r.URL.Path, "/actions/runs"):
			io.WriteString(w, `{"workflow_runs":[{"id":9,"status":"completed","conclusion":"failure","name":"CI"}]}`)
		default:
			io.WriteString(w, `[]`)
		}
	}))
	defer server.Close()

	collector := collectors.NewGitHubCollector(server.URL, "token", "owner", "repo")
	snapshot, err := collector.Collect(context.Background(), repoinsights.Window{Start: time.Now().Add(-24 * time.Hour), End: time.Now()})
	require.NoError(t, err)
	require.Len(t, snapshot.PullRequests, 1)
	require.Len(t, snapshot.FailingChecks, 1)
}
```

```go
func TestMentionParser_ParsesWindowAndFocus(t *testing.T) {
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	req := teams.ParseMention("repo insights risks in infra over the last 3 days", now, 7*24*time.Hour)
	require.Equal(t, "infra", req.FocusArea)
	require.Equal(t, now.Add(-72*time.Hour), req.Window.Start)
	require.Equal(t, now, req.Window.End)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/repoinsights/collectors ./internal/repoinsights/teams -run 'TestGitHubCollector_CollectsPRsReviewsAndChecks|TestMentionParser_ParsesWindowAndFocus' -count=1`
Expected: FAIL with missing GitHub collector or Teams parser/client.

- [ ] **Step 3: Implement GitHub collection, Graph client, and mention parsing**

```go
type GitHubCollector struct {
	baseURL string
	token   string
	owner   string
	repo    string
	client  *http.Client
}

func (c *GitHubCollector) Collect(ctx context.Context, window repoinsights.Window) (*repoinsights.GitHubSnapshot, error) {
	pulls, err := c.getPulls(ctx, window)
	if err != nil {
		return nil, err
	}
	checks, err := c.getWorkflowRuns(ctx, window)
	if err != nil {
		return nil, err
	}
	return &repoinsights.GitHubSnapshot{PullRequests: pulls, FailingChecks: checks}, nil
}
```

```go
type Client struct {
	tokenURL string
	graphURL string
	http     *http.Client
	cfg      GraphConfig
}

func (c *Client) ListChannelMessages(ctx context.Context, since string) ([]Message, error) {
	token, err := c.exchangeToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.graphURL+"/teams/"+c.cfg.TeamID+"/channels/"+c.cfg.ChannelID+"/messages", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	q := req.URL.Query()
	q.Set("$top", "25")
	req.URL.RawQuery = q.Encode()

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct{ Value []Message `json:"value"` }
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return filterMessagesSince(payload.Value, since), nil
}

func (c *Client) PostChannelMessage(ctx context.Context, body string) (string, error) {
	return c.postMessage(ctx, "/teams/"+c.cfg.TeamID+"/channels/"+c.cfg.ChannelID+"/messages", body)
}

func (c *Client) ReplyToMessage(ctx context.Context, messageID, body string) error {
	_, err := c.postMessage(ctx, "/teams/"+c.cfg.TeamID+"/channels/"+c.cfg.ChannelID+"/messages/"+messageID+"/replies", body)
	return err
}

func (c *Client) exchangeToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("scope", "https://graph.microsoft.com/.default")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.AccessToken, nil
}
```

```go
func ParseMention(body string, now time.Time, defaultLookback time.Duration) repoinsights.ReportRequest {
	req := repoinsights.ReportRequest{
		Trigger: repoinsights.TriggerMention,
		Window:  repoinsights.Window{Start: now.Add(-defaultLookback), End: now},
	}

	lower := strings.ToLower(stripHTML(body))
	if strings.Contains(lower, "infra") {
		req.FocusArea = "infra"
	}
	if matches := regexp.MustCompile(`last (\d+) days`).FindStringSubmatch(lower); len(matches) == 2 {
		days, _ := strconv.Atoi(matches[1])
		req.Window.Start = now.Add(-time.Duration(days) * 24 * time.Hour)
	}
	return req
}
```

- [ ] **Step 4: Re-run the client and parser tests**

Run: `go test ./internal/repoinsights/collectors ./internal/repoinsights/teams -run 'TestGitHubCollector_CollectsPRsReviewsAndChecks|TestMentionParser_ParsesWindowAndFocus' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/repoinsights/collectors/github.go internal/repoinsights/collectors/github_test.go internal/repoinsights/teams/client.go internal/repoinsights/teams/client_test.go internal/repoinsights/teams/parser.go internal/repoinsights/teams/parser_test.go
git commit -m "feat: add github and teams integration clients"
```

### Task 5: Implement Scoring, Narration, and Teams Formatting

**Files:**
- Create: `internal/repoinsights/scoring/engine.go`
- Create: `internal/repoinsights/scoring/engine_test.go`
- Create: `internal/repoinsights/reporting/formatter.go`
- Create: `internal/repoinsights/reporting/formatter_test.go`
- Create: `internal/repoinsights/reporting/openai.go`
- Create: `internal/repoinsights/reporting/openai_test.go`

- [ ] **Step 1: Write the failing scoring and formatting tests**

```go
func TestEngine_RanksRiskFromFailingChecksAndInfraChurn(t *testing.T) {
	engine := scoring.NewEngine()
	snapshot := repoinsights.ActivitySnapshot{
		GitHub: &repoinsights.GitHubSnapshot{FailingChecks: []repoinsights.CheckRun{{Name: "CI", Conclusion: "failure"}}},
		Git:    &repoinsights.GitSnapshot{HotDirectories: []string{"terraform", "deploy"}},
	}

	report := engine.Score(snapshot)
	require.NotEmpty(t, report.EngineeringRisk)
	require.Contains(t, report.EngineeringRisk[0].Summary, "failing")
}
```

```go
func TestFormatter_RenderScheduledReport(t *testing.T) {
	msg := reporting.FormatTeamsReport(repoinsights.RenderInput{
		Headline: "platform repo insights for Apr 20",
		ExecutiveSummary: []string{"Delivery moved forward in CI and infra work."},
		EngineeringRisk:  []string{"One workflow remains red."},
		TeamActivity:     []string{"Most activity concentrated in terraform and internal/handler."},
	})

	require.Contains(t, msg, "Executive summary")
	require.Contains(t, msg, "Engineering risk")
	require.Contains(t, msg, "Team activity")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/repoinsights/scoring ./internal/repoinsights/reporting -run 'TestEngine_RanksRiskFromFailingChecksAndInfraChurn|TestFormatter_RenderScheduledReport' -count=1`
Expected: FAIL with missing scoring engine or formatter.

- [ ] **Step 3: Implement the weighted scorer, formatter, and optional OpenAI narrator**

```go
func (e *Engine) Score(snapshot repoinsights.ActivitySnapshot) repoinsights.ScoredReport {
	var risk []repoinsights.Finding
	if len(snapshot.GitHub.FailingChecks) > 0 {
		risk = append(risk, repoinsights.Finding{Score: 9, Summary: "Failing checks are blocking clean delivery."})
	}
	for _, dir := range snapshot.Git.HotDirectories {
		if dir == "terraform" || dir == "deploy" || dir == "k8s" {
			risk = append(risk, repoinsights.Finding{Score: 7, Summary: "Infra churn is concentrated in deployment-sensitive paths."})
			break
		}
	}
	sort.Slice(risk, func(i, j int) bool { return risk[i].Score > risk[j].Score })
	return repoinsights.ScoredReport{EngineeringRisk: risk}
}
```

```go
func FormatTeamsReport(input repoinsights.RenderInput) string {
	var b strings.Builder
	b.WriteString("**" + input.Headline + "**\n\n")
	b.WriteString("Executive summary\n")
	for _, line := range input.ExecutiveSummary {
		b.WriteString("- " + line + "\n")
	}
	b.WriteString("\nEngineering risk\n")
	for _, line := range input.EngineeringRisk {
		b.WriteString("- " + line + "\n")
	}
	b.WriteString("\nTeam activity\n")
	for _, line := range input.TeamActivity {
		b.WriteString("- " + line + "\n")
	}
	return b.String()
}
```

```go
type Narrator interface {
	Narrate(ctx context.Context, report repoinsights.ScoredReport) (repoinsights.RenderInput, error)
}

type TemplateNarrator struct{}
type OpenAINarrator struct { BaseURL, APIKey, Model string; HTTPClient *http.Client }
```

- [ ] **Step 4: Run the scoring and formatting tests**

Run: `go test ./internal/repoinsights/scoring ./internal/repoinsights/reporting -run 'TestEngine_RanksRiskFromFailingChecksAndInfraChurn|TestFormatter_RenderScheduledReport' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/repoinsights/scoring/engine.go internal/repoinsights/scoring/engine_test.go internal/repoinsights/reporting/formatter.go internal/repoinsights/reporting/formatter_test.go internal/repoinsights/reporting/openai.go internal/repoinsights/reporting/openai_test.go
git commit -m "feat: add repo insights scoring and report formatting"
```

### Task 6: Wire the Orchestrator, Scheduler, Poller, and Metrics

**Files:**
- Create: `internal/repoinsights/orchestrator.go`
- Create: `internal/repoinsights/orchestrator_test.go`
- Create: `internal/repoinsights/runtime/scheduler.go`
- Create: `internal/repoinsights/runtime/scheduler_test.go`
- Create: `internal/repoinsights/runtime/poller.go`
- Modify: `internal/metrics/metrics.go`
- Modify: `cmd/repo-insights/main.go`

- [ ] **Step 1: Write the failing orchestration and scheduler tests**

```go
func TestOrchestrator_RunScheduledDigestPublishesAndPersistsRun(t *testing.T) {
	state := &fakeState{}
	publisher := &fakePublisher{}
	orchestrator := &repoinsights.Orchestrator{
		State:           state,
		GitCollector:    fakeGitCollector{},
		GitHubCollector: fakeGitHubCollector{},
		Engine:          fakeEngine{},
		Narrator:        fakeNarrator{},
		Publisher:       publisher,
		RepoPath:        "/tmp/platform",
	}
	req := repoinsights.ReportRequest{
		Trigger: repoinsights.TriggerScheduled,
		Window:  repoinsights.Window{Start: time.Date(2026, 4, 19, 3, 30, 0, 0, time.UTC), End: time.Date(2026, 4, 20, 3, 30, 0, 0, time.UTC)},
	}

	err := orchestrator.Run(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, publisher.posts, 1)
	require.Equal(t, "scheduled:2026-04-20", state.savedDedupKey)
}

type fakeState struct{ savedDedupKey string }

func (f *fakeState) DedupExists(_ context.Context, dedupKey string) (bool, error) {
	f.savedDedupKey = dedupKey
	return false, nil
}

type fakePublisher struct{ posts []string }

func (f *fakePublisher) Publish(_ context.Context, _ repoinsights.ReportRequest, body string) error {
	f.posts = append(f.posts, body)
	return nil
}

type fakeGitCollector struct{}

func (fakeGitCollector) Collect(_ context.Context, _ string, _ repoinsights.Window) (*repoinsights.GitSnapshot, error) {
	return &repoinsights.GitSnapshot{HotDirectories: []string{"terraform"}}, nil
}

type fakeGitHubCollector struct{}

func (fakeGitHubCollector) Collect(_ context.Context, _ repoinsights.Window) (*repoinsights.GitHubSnapshot, error) {
	return &repoinsights.GitHubSnapshot{FailingChecks: []repoinsights.CheckRun{{Name: "CI", Conclusion: "failure"}}}, nil
}

type fakeEngine struct{}

func (fakeEngine) Score(_ repoinsights.ActivitySnapshot) repoinsights.ScoredReport {
	return repoinsights.ScoredReport{EngineeringRisk: []repoinsights.Finding{{Score: 9, Summary: "Failing checks are blocking clean delivery."}}}
}

type fakeNarrator struct{}

func (fakeNarrator) Narrate(_ context.Context, _ repoinsights.ScoredReport) (repoinsights.RenderInput, error) {
	return repoinsights.RenderInput{
		Headline:         "platform repo insights for Apr 20",
		ExecutiveSummary: []string{"Delivery moved forward."},
		EngineeringRisk:  []string{"Failing checks are blocking clean delivery."},
		TeamActivity:     []string{"Work concentrated in terraform."},
	}, nil
}
```

```go
func TestNextCutoff_MondayCapturesWeekendWindow(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	last := time.Date(2026, 4, 17, 9, 0, 0, 0, loc) // Friday
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, loc)  // Monday

	window := runtime.WindowFromPreviousCutoff(last, now)
	require.Equal(t, last, window.Start)
	require.Equal(t, now, window.End)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/repoinsights ./internal/repoinsights/runtime -run 'TestOrchestrator_RunScheduledDigestPublishesAndPersistsRun|TestNextCutoff_MondayCapturesWeekendWindow' -count=1`
Expected: FAIL with missing orchestrator or scheduler logic.

- [ ] **Step 3: Implement orchestration, polling, and metrics**

```go
func (o *Orchestrator) Run(ctx context.Context, req ReportRequest) error {
	if exists, err := o.state.DedupExists(ctx, o.dedupKey(req)); err != nil || exists {
		return err
	}

	gitSnapshot, err := o.gitCollector.Collect(ctx, o.repoPath, req.Window)
	if err != nil {
		return err
	}
	githubSnapshot, err := o.githubCollector.Collect(ctx, req.Window)
	if err != nil {
		o.logger.Warn().Err(err).Msg("github snapshot incomplete")
	}

	scored := o.engine.Score(ActivitySnapshot{Git: gitSnapshot, GitHub: githubSnapshot, FocusArea: req.FocusArea})
	rendered, err := o.narrator.Narrate(ctx, scored)
	if err != nil {
		return err
	}
	message := reporting.FormatTeamsReport(rendered)
	return o.publisher.Publish(ctx, req, message)
}
```

```go
var RepoInsightsRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: namespace,
	Name:      "repo_insights_runs_total",
	Help:      "Total repo-insights runs by trigger and result.",
}, []string{"trigger", "result"})
```

```go
func (p *Poller) Run(ctx context.Context) error {
	cursor, _ := p.state.LoadCursor(ctx, p.cursorName)
	messages, err := p.teams.ListChannelMessages(ctx, cursor)
	if err != nil {
		return err
	}
	for _, msg := range messages {
		if msg.FromBot {
			continue
		}
		req := teams.ParseMention(msg.Body, p.clock.Now(), p.defaultLookback)
		req.ReplyToMessage = msg.ID
		if err := p.orchestrator.Run(ctx, req); err != nil {
			p.logger.Error().Err(err).Str("message_id", msg.ID).Msg("mention run failed")
		}
		_ = p.state.SaveCursor(ctx, p.cursorName, msg.LastModifiedDateTime)
	}
	return nil
}
```

- [ ] **Step 4: Run orchestration tests**

Run: `go test ./internal/repoinsights ./internal/repoinsights/runtime -run 'TestOrchestrator_RunScheduledDigestPublishesAndPersistsRun|TestNextCutoff_MondayCapturesWeekendWindow' -count=1`
Expected: PASS

- [ ] **Step 5: Smoke-test the binary**

Run: `go build ./cmd/repo-insights && go test ./internal/repoinsights/... -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/repoinsights/orchestrator.go internal/repoinsights/orchestrator_test.go internal/repoinsights/runtime/scheduler.go internal/repoinsights/runtime/scheduler_test.go internal/repoinsights/runtime/poller.go internal/metrics/metrics.go cmd/repo-insights/main.go
git commit -m "feat: wire repo insights runtime loops"
```

### Task 7: Add Build, Compose, and Operator Wiring

**Files:**
- Modify: `Makefile`
- Modify: `Dockerfile`
- Modify: `docker-compose.yml`
- Modify: `.github/workflows/ci.yml`
- Create: `docs/repo-insights.md`

- [ ] **Step 1: Write the failing build and compose checks**

Run: `make build-repo-insights`
Expected: FAIL with `No rule to make target 'build-repo-insights'`.

Run: `docker compose config | rg repo-insights`
Expected: FAIL because no `repo-insights` service exists yet.

- [ ] **Step 2: Add build targets, container binary, compose service, and docs**

```make
build-repo-insights:
	go build -o bin/repo-insights ./cmd/repo-insights

run-repo-insights:
	go run ./cmd/repo-insights
```

```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/repo-insights ./cmd/repo-insights
COPY --from=builder /bin/repo-insights /bin/repo-insights
EXPOSE 8083 9094
```

```yaml
  repo-insights:
    build:
      context: .
      dockerfile: Dockerfile
    command: ["/bin/repo-insights"]
    environment:
      - DATABASE_URL=postgres://narayana:narayana@postgres:5432/narayana?sslmode=disable
      - REPO_INSIGHTS_ENABLED=true
      - REPO_INSIGHTS_REPO_PATH=/workspace
    volumes:
      - .:/workspace
    depends_on:
      postgres:
        condition: service_healthy
      migrate:
        condition: service_completed_successfully
```

```yaml
      - name: Build repo-insights binary
        run: CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/repo-insights ./cmd/repo-insights
```

- [ ] **Step 3: Re-run build and compose validation**

Run: `make build-repo-insights && docker compose config | rg repo-insights`
Expected: PASS

- [ ] **Step 4: Run the full targeted verification set**

Run: `go test ./internal/config ./internal/repoinsights/... ./tests/integration/... -count=1`
Expected: PASS

Run: `go build ./cmd/repo-insights`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add Makefile Dockerfile docker-compose.yml .github/workflows/ci.yml docs/repo-insights.md
git commit -m "chore: add repo insights runtime wiring"
```

## Self-Review Checklist

- Confirm every spec requirement maps to a task:
  - weekday digest: Task 6
  - mention-based on-demand reports: Tasks 4 and 6
  - local git + GitHub collection: Tasks 3 and 4
  - Teams-only delivery: Tasks 4 and 6
  - run deduplication and cursor state: Task 2
  - consistent three-section output: Task 5
- Search this plan for placeholders before execution:
  - `rg -n 'TBD|TODO|implement later|appropriate error handling|similar to Task|omitted|/\\*|\\.\\.\\.' docs/superpowers/plans/2026-04-20-repo-insights-agent.md`
- Keep runtime scope focused:
  - no owner tagging
  - no ticket creation
  - no cross-repo support

## Verification Commands

- `go test ./internal/config ./internal/repoinsights/... -count=1`
- `go test ./tests/integration/... -run RepoInsights -count=1`
- `go build ./cmd/repo-insights`
- `make build-repo-insights`
- `docker compose config | rg repo-insights`
