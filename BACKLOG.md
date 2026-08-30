# Remediation Backlog — Entity Matcher

Derived from the code review of commit `1a79ae5`. Ordered by dependency, not by epic number:
correctness of the decision layer first, then the measurement that proves it, then the
platform claims that are currently unbacked.

Severity: **C**ritical / **H**igh / **M**edium.

---

## EPIC A — Match Decision Layer (C)

The engine ranks well (99.3% top-1 on the built-in dataset) but never *decides*. It emits
13.1 pairs per source and marks every pair over 0.90 as `AUTO_MATCHED`, so 38.6% of
labelled non-match sources receive a confident verdict against some unrelated record.

| ID | Story | AC |
| :-- | :-- | :-- |
| A1 | Rank candidates per source and select top-1 | Only the best-scoring candidate per source is eligible for auto-match |
| A2 | Margin rule | Auto-match requires `best >= auto_threshold` **and** `best - runner_up >= margin` (default 0.05); otherwise `REVIEW_NEEDED` |
| A3 | Global 1:1 assignment | A destination is claimed by at most one source per batch; conflicts resolved greedily by descending score |
| A4 | Unmatched reporting | Sources with no candidate over `review_threshold` emit a `NO_MATCH` row, included in results and CSV export |
| A5 | Score aggregation rework | Replace `max*0.6 + avg*0.4` with a configurable weighted mean over enabled metrics; `max` retained only as a tie-break signal |
| A6 | Config surface | `margin_threshold`, `assignment_strategy` (`GREEDY_1_1` \| `TOP_1` \| `ALL_CANDIDATES`), `emit_unmatched` |

## EPIC B — Benchmark & Test Integrity (C)

The 100%/7,854 rec/s headline is measured over 4,000 labelled pairs while ~49,000 emitted
pairs go uncounted, and the file lives in `backend/testdata/`, which the Go toolchain
excludes from `./...` — so it never runs in CI.

| ID | Story | AC |
| :-- | :-- | :-- |
| B1 | Make the benchmark runnable | Move `backend/testdata` → `backend/internal/mockdata`; `go test ./...` executes it |
| B2 | Decision-level metrics | Precision/recall/F1 computed over the assignment output, not over raw pairs |
| B3 | Hard negatives | Add same-surname/different-person, same-corporate-prefix/different-entity, shared-generic-token, and initials-only categories |
| B4 | API tests | Auth enforcement per role, config merge, scheduler persistence, pagination bounds |
| B5 | Regression tests | Normalizer boundary cases (Williams/Adams/Vincent/ไทยนาย), date parsing, assignment invariants |

## EPIC C — Normalizer Correctness (H)

| ID | Story | AC |
| :-- | :-- | :-- |
| C1 | Token-boundary title stripping | `Williams`→`williams`, `Adams`→`adams`, `Vincent Prince Inc.`→`vincent prince`, `ไทยนาย`preserved |
| C2 | Synonym dictionary wired into `Normalize()` | `KBank` and `กสิกรไทย` both normalize to `kasikornbank` and score 1.0 against each other |
| C3 | Date parsing | Buddhist-era years (2569→2026), multiple layouts, `time.Time{}` for unknown; unknown dates are neutral, not 1.0 |
| C4 | Rune-safe prefix + honest phonetic key | No byte-slicing of UTF-8; consonant-skeleton key that actually drops vowels in both scripts |
| C5 | Unspaced Thai | Single-token names fall back to trigram-dominant scoring |

## EPIC D — Security & RBAC (H)

Currently: no middleware, every route open, `/api/auth/me` returns ADMIN without a token,
`password123` authenticates any account, hardcoded signing key, `Access-Control-Allow-Origin: *`.

| ID | Story | AC |
| :-- | :-- | :-- |
| D1 | Auth middleware + route policy | Every `/api/*` route except `login`/`health` requires a valid token; role policy enforced per route |
| D2 | Credential hardening | bcrypt hashes, `JWT_SECRET` from env (generated + warned in dev), universal password removed |
| D3 | Audit identity | `user_id` on audit entries comes from the verified token, never the request body |
| D4 | CORS allowlist | Origins from `CORS_ORIGINS`, default same-origin |
| D5 | Frontend auth | Login screen, token persistence, `Authorization` header on every call, role-gated navigation |

## EPIC E — Scheduler & Webhooks (H)

`NewSchedulerManager()` is constructed inside the handler, so every POST is discarded; no
cron engine exists and `DispatchWebhook` is never called.

| ID | Story | AC |
| :-- | :-- | :-- |
| E1 | Singleton manager | Config survives POST→GET |
| E2 | Real cron engine | `robfig/cron/v3` triggers reconciliation on the configured expression; invalid expressions rejected at write time |
| E3 | Webhook dispatch | Fired on completion and on anomaly; Slack/Teams/generic payload shapes; bounded retry |
| E4 | Scheduler UI | Settings panel with cron validation and last-run status |

## EPIC F — Config Robustness (H)

| ID | Story | AC |
| :-- | :-- | :-- |
| F1 | Merge-on-write + validation | Partial `PUT /api/config` preserves unspecified fields; thresholds bounded to [0,1]; `review <= auto`; weights must be positive |

## EPIC G — Connectors (H)

All connectors are simulated. `TestConnection` returns success for hosts that do not exist,
`IntrospectSchema` returns hardcoded columns, `FetchRecords` fabricates rows, Excel is never
opened, and `go.mod` contains no drivers.

| ID | Story | AC |
| :-- | :-- | :-- |
| G1 | PostgreSQL | Real ping, `information_schema` introspection, paged fetch |
| G2 | SQL Server | Real ping, `INFORMATION_SCHEMA` introspection, `OFFSET/FETCH` paging |
| G3 | MongoDB | Real ping, sampled field inference, `skip/limit` paging |
| G4 | CSV + Excel | Real file streaming; `.xlsx` parsed via `excelize` |
| G5 | Honest failure | A bad host fails; no fabricated rows anywhere |

## EPIC H — Persistence (M) — spec EPIC 2, never started

| ID | Story | AC |
| :-- | :-- | :-- |
| H1 | Schema + migrations | `match_jobs`, `match_results`, `match_audit_logs` tables created on boot |
| H2 | Repository abstraction | In-memory and Postgres implementations behind one interface; selected by `DATABASE_URL` |
| H3 | Job history API | `GET /api/jobs` lists historical runs with counters and duration |

## EPIC I — Scale & Performance (M)

| ID | Story | AC |
| :-- | :-- | :-- |
| I1 | Cap trigram posting lists | Ultra-common trigrams skipped during query so per-source cost stays sub-linear |
| I2 | O(1) review actions | Results indexed by ID; no full-batch scan per click |
| I3 | Stop embedding record copies | Results hold IDs; records hydrated on read |

## EPIC J — Deployment & Docs (M)

| ID | Story | AC |
| :-- | :-- | :-- |
| J1 | Dockerfile Go version | Builder matches `go.mod` |
| J2 | nginx `/api` proxy | SPA served by nginx reaches the backend |
| J3 | Port alignment | Compose and README agree |
| J4 | README truth pass | Every claim maps to working code; measured numbers replace aspirational ones |
