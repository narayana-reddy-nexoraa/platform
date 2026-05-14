# Repo Insights Agent Design

Date: 2026-04-20
Status: Approved for planning
Target repo: `/Users/narayana-nexoraa/Developer/HSD/platform`
Primary audience: Engineering team
Delivery surface: Dedicated Microsoft Teams `repo-insights` channel

## 1. Goal

Build a repo-insights agent that analyzes repository activity for the `platform` repo and posts concise, evidence-backed insights into a dedicated Teams channel.

The agent must support two modes from day one:

1. Weekday scheduled digest
2. On-demand report when a user mentions the bot in the Teams channel

Every report must include the same three lenses:

1. Executive summary
2. Engineering risk
3. Team activity

For v1, the agent reports findings only. It does not tag owners, assign work, create tickets, or trigger remediation.

## 2. Context

The `platform` repo is an enterprise orchestration codebase with active work across Go services, Temporal workflows, Kafka/eventing, deployment infrastructure, Kubernetes manifests, Terraform, tests, and a React UI. Recent activity shows a mix of feature delivery, CI/CD work, infrastructure provisioning, compliance/testing, and fix-heavy follow-up in build and deployment paths.

This makes simple commit summaries insufficient. The agent needs to combine local git signals with GitHub collaboration signals so that the report reflects both code movement and delivery health.

## 3. Recommended Approach

Use a hybrid pipeline with deterministic collection and scoring, plus an LLM for narrative generation.

### Why this approach

- More reliable than a pure "one-shot summary" bot
- Easier to debug when a risk signal is wrong or noisy
- Consistent across daily runs
- Still produces readable, human-friendly reports in Teams

### Rejected alternatives

`Single LLM summarizer`
Fast to prototype, but too inconsistent for repeated operational use.

`Multi-agent specialist system`
Useful later for multi-repo scale, but too complex for one repo and one channel in v1.

## 4. Scope

### In scope

- Analyze one repo only: `platform`
- Read local git activity from the configured repo path
- Read GitHub PR, issue, review, and CI/check activity for the same repo
- Post weekday digests to a Teams channel
- Reply to mention-based requests in that same Teams channel
- Generate three lenses in every report
- Store minimal run state for scheduling, deduplication, and comparisons

### Out of scope

- Cross-repo rollups
- Owner tagging in Teams
- Auto-created tickets or tasks
- Auto-remediation
- Separate dashboard or web UI
- Deep historical BI-style analytics

## 5. Architecture

The v1 architecture is a single repo-insights service with focused internal components.

### Components

`scheduler`
Triggers the weekday digest on a fixed schedule.

`teams-listener`
Receives Teams mentions from the dedicated channel and turns them into on-demand report requests.

`repo-activity-collector`
Reads local git data from the repo checkout.

`github-activity-collector`
Reads GitHub pull requests, review state, issues, labels, check runs, and merge outcomes.

`insight-engine`
Scores and ranks candidate findings for executive summary, engineering risk, and team activity.

`narrative-generator`
Uses an LLM to turn structured findings into concise, stable Teams-ready prose.

`teams-publisher`
Posts scheduled digests to the channel and replies in-thread for mention requests.

`run-state-store`
Stores last processed windows, deduplication keys, baseline metrics, and run metadata.

### Deployment shape

Implement v1 as a Go service inside the `platform` repo so it aligns with the repo's existing language, deployment, and operational model.

Suggested package shape:

- `cmd/repo-insights/`
- `internal/repoinsights/collectors/`
- `internal/repoinsights/scoring/`
- `internal/repoinsights/reporting/`
- `internal/repoinsights/teams/`
- `internal/repoinsights/state/`

## 6. Integrations

### Local git

The agent reads:

- commits in the window
- author counts
- changed files and directories
- churn metrics
- repeated edits to the same files
- hotspot directories

### GitHub

The agent reads:

- opened, updated, merged, and closed PRs
- draft to review to merge movement
- open review requests
- review comments and approval state
- failing or passing checks
- issues opened, closed, and updated
- labels and milestones when present

### Microsoft Teams

The agent posts only to the dedicated `repo-insights` channel.

For v1, Teams integration must support:

- scheduled post creation
- mention detection in channel conversations
- threaded replies to mention requests

Recommended integration model:

- Teams bot/app identity for mentions
- Microsoft Graph APIs for message send/read operations

## 7. Runtime Flows

### 7.1 Scheduled weekday digest

1. Scheduler fires every weekday at 9:00 AM Asia/Kolkata.
2. The service computes the reporting window from the previous successful scheduled cutoff to the current cutoff, so Monday captures weekend activity instead of dropping it.
3. Collectors fetch local git and GitHub activity for that window.
4. The insight engine scores findings across progress, risk, and activity.
5. The narrative generator formats the final message.
6. The Teams publisher posts the digest to the `repo-insights` channel.
7. The state store records the completed run and dedup keys.

The digest should also include a lightweight current snapshot for open review backlog and failing checks, even if those items originated before the reporting window.

### 7.2 On-demand mention flow

1. A user mentions the bot in the `repo-insights` channel.
2. The service parses intent, time window, and optional focus area.
3. The same collectors and insight engine run with the requested filters.
4. The bot replies in-thread with the tailored report.

Default behavior when the user does not specify a time window:

- report on the last 7 days
- include all three lenses

Example future requests the parser should support:

- `@repo-insights what changed this week?`
- `@repo-insights risks in infra over the last 3 days`
- `@repo-insights summarize current review backlog`

## 8. Output Contract

Every report must use the same top-level structure:

1. Executive summary
2. Engineering risk
3. Team activity

### 8.1 Weekday digest format

The daily Teams post should be short enough to scan in under one minute.

Recommended structure:

- headline line with repo name and window
- executive summary section
- engineering risk section
- team activity section
- optional footer with "data incomplete" note if one source failed

Target limits:

- 5 to 7 bullets total
- at least 1 bullet per lens
- no raw commit dumps
- no large tables

### 8.2 On-demand reply format

On-demand replies use the same three sections but may go slightly deeper if the user asked for a narrower scope.

If a request is very focused, the bot can keep all three headings but collapse low-signal sections into one-line observations.

## 9. Insight Engine Design

The insight engine should produce structured findings before any LLM wording step.

### 9.1 Delivery progress signals

- merged PR volume and merge velocity
- movement from draft to review to merged
- issues closed versus opened
- milestone or sprint label movement
- completion signals in major subsystems such as API, UI, infra, tests, and deployment
- reduction in open blockers, pending reviews, or failing checks

### 9.2 Engineering risk signals

- failing or flaky CI
- PRs stuck in review too long
- high churn in sensitive areas such as `internal/`, `db/migrations/`, `deploy/`, `k8s/`, and `terraform/`
- repeated short-window edits in the same files or subsystems
- large PRs with weak review activity
- fast merges with limited review evidence
- infra or config churn without matching validation signals
- growing bug or operational issue backlog

### 9.3 Team activity signals

- active contributors in the window
- concentration of work by directory or subsystem
- review participation
- issue discussion volume
- imbalance signals where one contributor carries most change or review load

### 9.4 Scoring model

Use a weighted rule-based scorer in v1.

1. Normalize raw activity into typed signals.
2. Assign weights to each signal.
3. Rank findings separately for progress, risk, and activity.
4. Select the strongest candidates.
5. Pass the selected candidates plus evidence into the LLM for final wording.

This keeps the system explainable and tunable.

## 10. Reliability and Operations

### Required behavior

- partial reports if one upstream source fails
- explicit "data incomplete" note when GitHub or git data is missing
- duplicate-post prevention
- timeout protection for mention-triggered runs
- safe retry behavior for scheduler failures

### State management

V1 only needs lightweight state, stored in SQLite or Postgres:

- last successful digest window
- processed Teams event ids
- report dedup keys
- rolling baselines for comparisons
- run history and error state

## 11. Security and Access

- GitHub access must be scoped to the target repo only
- Teams access must be limited to the target channel or approved team scope
- secrets must be stored using the repo's standard secret management path
- posted messages must avoid leaking sensitive tokens, secrets, or private infra details

## 12. Success Criteria

V1 is successful when:

- the bot posts an unattended weekday digest into the `repo-insights` Teams channel
- users can mention the bot in that channel and receive an on-demand report
- every report includes executive summary, engineering risk, and team activity
- the findings are evidence-backed rather than generic
- repeated daily posts avoid obvious duplication and noise

## 13. Build Order

Recommended implementation order:

1. scheduled digest pipeline
2. scoring and ranking engine
3. Teams publisher
4. on-demand mention flow

This order delivers value early while keeping the architecture reusable.

## 14. Assumptions

- The first deployment analyzes only the local checkout at `/Users/narayana-nexoraa/Developer/HSD/platform`.
- The engineering team's working timezone for the scheduled digest is Asia/Kolkata.
- The dedicated Teams channel already exists or will be created before rollout.
- GitHub and Teams credentials will be available at deploy time.

## 15. Non-Goals for Future Phases

These are intentionally deferred until v1 proves useful:

- multi-repo portfolio insights
- tagging likely owners
- automatic follow-up task creation
- historical trend dashboard
- per-subsystem custom digests
