# High-Scale Thai–English Entity Resolution & Data Matching Engine

Bilingual (Thai/English) entity resolution for individuals and legal entities, with a
human-in-the-loop review queue, a compliance audit trail, and pluggable data connectors.

Every number and capability below is measured or exercised by a test in this repo. Where something
is partial, it says so.

---

## What it does

Given a **source** and a **destination** dataset, the engine normalizes bilingual names, retrieves
candidates via a blocking index, scores them with five complementary metrics, and then **decides**:
each source record gets at most one auto-matched partner, a ranked shortlist for human review, or an
explicit `NO_MATCH`.

The decision layer is the part that matters. Scoring alone produces a ranked list; it does not
produce a reconciliation.

### Decision rules

A source's rank-1 candidate is auto-matched when its score clears `auto_match_threshold` **and**
either:

- **margin rule** — it beats the runner-up by at least `margin_threshold` (default 0.05), or
- **decisive exact match** — it scores at or above `exact_match_floor` (default 0.99) while the
  runner-up does not. Two candidates that both normalize identically are a genuine tie and go to
  review.

A global greedy pass then enforces **one-to-one**: a destination is claimed by at most one source
per batch. A source that loses a contested destination is demoted to review with a note naming the
winning source and its score.

---

## Measured performance

From `go test ./internal/mockdata/` (4,400 × 4,400 synthetic bilingual records, 19.4M candidate
pair space, fixed seed):

| Metric | Value |
| :-- | :-- |
| Throughput | ~7,900 sources/sec |
| Decision precision | 100.00% |
| Decision recall (auto-match) | 57.6% |
| F1 | 73.1% |
| Top-1 ranking accuracy | 99.19% |
| Candidate rows per source | 3.15 |
| Max destinations auto-matched per source | 1 |
| Max sources claiming one destination | 1 |

**Read these two numbers together.** Top-1 ranking accuracy (99.19%) is how often the correct
partner is ranked first — that is the quality of the *scorer*. Recall (57.6%) is how often the
engine is confident enough to decide *without a human* — that is the calibration of the
*thresholds*. The gap is deliberate: the remainder goes to the review queue rather than being
auto-matched on thin evidence. Lower `margin_threshold` or `auto_match_threshold` to trade
precision for recall.

The benchmark scores **decisions, not pairs**, and the negative set includes hard cases
(same-surname/different-person, same-corporate-prefix/different-entity, generic-token overlap,
initials-only, transposition collisions) alongside the easy branch-number mismatch.

### Benchmarking honestly

An earlier revision of this project advertised "100% accuracy, 7,854 records/sec". That figure was
not meaningful: it scored only the 4,000 labelled pairs while ignoring the ~49,000 additional pairs
the engine emitted, so it measured recall and reported it as precision — true pair-level precision
was about 5.7%. The benchmark also lived in `backend/testdata/`, a directory the Go toolchain
excludes from package resolution, so `go test ./...` silently skipped it and CI never ran it.

It now lives in `backend/internal/mockdata/`, runs under `go test ./...`, and asserts structural
invariants (the 1:1 constraint, no lost rows, `NO_MATCH` rows carry no destination) in addition to
printing metrics.

---

## Capabilities

| Capability | Status |
| :-- | :-- |
| **Bilingual normalizer** | Token-boundary title/honorific stripping (Thai + English), Thai prefix-glued title handling, NFC normalization, tone-mark stripping, consonant-skeleton key |
| **Multi-metric scoring** | Rune-safe Jaro-Winkler, Levenshtein, token-sort, trigram Jaccard, phonetic key; max-dominant blend |
| **Decision layer** | Top-1 + margin rule + decisive-exact-match rule + greedy 1:1 assignment + `NO_MATCH` reporting |
| **Blocking index** | Trigram + token + phonetic inverted index, with frequency cutoffs so ultra-common keys don't degrade retrieval toward O(N) |
| **Data connectors** | Real drivers: PostgreSQL (`pgx/v5`), SQL Server (`go-mssqldb`), MongoDB (`mongo-driver`), Excel (`excelize`), CSV, manual entry |
| **Dynamic schema mapping** | Multi-field name composition, reference/date column mapping, secondary pairing rules (exact / fuzzy / numeric delta / mandatory) |
| **Date handling** | Multi-layout parsing including **Buddhist-era years** (2569 → 2026) and Thai digits (๒๕๖๙); unparseable dates are unknown, never silently "today" |
| **Synonym dictionary** | Brand aliases (`KBank` / `กสิกรไทย` → `kasikornbank`) applied inside the normalizer, so they affect scoring |
| **Compliance audit trail** | Reviewer identity taken from the verified JWT (never the request body), CSV export, append-only enforcement in Postgres |
| **AuthN / RBAC** | HMAC-SHA256 JWT, bcrypt credentials, four roles enforced by middleware on every route |
| **Scheduler & webhooks** | Real cron engine (`robfig/cron/v3`) running reconciliation; Slack / Teams / generic webhooks with retry |
| **Persistence** | In-memory by default; PostgreSQL staging when `DATABASE_URL` is set |
| **React SPA** | Login, role-gated navigation, review queue with token-level diffing, SSE progress, audit dashboard, scheduler panel |
| **LLM edge-case resolver** | Gemini structured-JSON evaluation with a rule-based local fallback |

### Roles

| Role | Access |
| :-- | :-- |
| `ADMIN` | Everything, including scheduler configuration |
| `ENGINEER` | Config, connectors, ingestion, running matches |
| `REVIEWER` | Review queue; confirm / reject / manual link |
| `AUDITOR` | Read-only audit trail and regulatory export |

`GET` of config, dictionary and scheduler settings is open to any authenticated role; writes are
restricted as above. Enforcement is covered by `backend/main_test.go`, which asserts the status code
for each role against representative routes.

---

## Quick start

### Docker (recommended)

```bash
docker compose up --build
```

The backend serves the API and the built SPA at **http://localhost:8085**. An nginx-fronted copy of
the SPA is also published on http://localhost:3000, proxying `/api` to the backend with buffering
disabled so the SSE progress stream works.

### Local

```bash
cd frontend && npm install && npm run build
cd ../backend && go build -o server . && PORT=8085 JWT_SECRET=dev-secret ./server
```

For frontend development with hot reload, `npm run dev` proxies `/api` to port 8085.

### Demo accounts

`admin`, `engineer_alex`, `reviewer_sarah`, `auditor_mike` — all with password `password123`.
These are seeded demo credentials with bcrypt-hashed passwords; replace them before any real
deployment.

### Configuration

| Variable | Default | Meaning |
| :-- | :-- | :-- |
| `PORT` | `8085` | HTTP listen port |
| `JWT_SECRET` | random per start | Token signing key. **Set this** — otherwise tokens do not survive a restart, and the server logs a warning |
| `DATABASE_URL` | unset | Enables PostgreSQL persistence; in-memory when unset |
| `CORS_ORIGINS` | unset | Comma-separated allowlist; same-origin only when unset |
| `GEMINI_API_KEY` | unset | Enables the LLM resolver; falls back to the local rule-based analyzer when unset |

Engine tuning (`PUT /api/config`, merge-on-write and validated): `auto_match_threshold`,
`review_threshold`, `margin_threshold`, `exact_match_floor`, `assignment_strategy`
(`GREEDY_1_1` / `TOP_1` / `ALL_CANDIDATES`), `max_alternatives_per_source`, `emit_unmatched`,
`date_tolerance_days`, `weights`, `algorithms`, `worker_count`, `max_candidates_per_src`.

---

## Testing

```bash
cd backend
go test ./...                 # full suite; Postgres tests skip without a database
go test -race ./store/

# with a real database
docker run -d --name em-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=entity_matcher \
  -p 55432:5432 postgres:16-alpine
TEST_DATABASE_URL="postgres://postgres:test@localhost:55432/entity_matcher?sslmode=disable" \
  go test ./store/
```

A live MongoDB on `localhost:27017` enables the Mongo connector integration test; it skips
otherwise.

---

## API

Authentication: `Authorization: Bearer <token>` on every route except `/api/health` and
`/api/auth/login`. `/api/match/progress` additionally accepts `?access_token=` because
`EventSource` cannot send headers.

**Auth** — `POST /api/auth/login`, `GET /api/auth/me`

**Engine** — `GET|PUT /api/config`, `POST /api/upload`, `POST /api/match/run`,
`GET /api/match/progress` (SSE), `GET /api/match/results`, `POST /api/match/action`,
`POST /api/match/manual-link`, `GET /api/destinations/search`, `GET /api/export/csv`,
`POST /api/llm/evaluate`

**Connectors** — `POST /api/connector/test`, `POST /api/connector/introspect`

**Governance** — `GET /api/audit/logs`, `GET /api/audit/export`

**Operations** — `GET|POST /api/scheduler/config`, `GET|POST /api/dictionary`,
`POST /api/seed`, `POST /api/seed/big`, `GET /api/health`

---

## Known limitations

- **Thai word segmentation.** Tokenization is whitespace-based. Thai is frequently written without
  inter-word spaces, so unspaced company names lean on the trigram metric rather than token
  comparison. A dictionary-based segmenter would improve this.
- **Recall is threshold-bound.** 57.6% of true pairs are auto-matched at the default thresholds;
  the rest reach a human. This is a deliberate operating point, not a ceiling.
- **Scale is verified to ~4,400 × 4,400.** The blocking index has frequency cutoffs intended for
  larger corpora, but the 100,000+ figure this project was originally described with has not been
  measured. Results are accumulated in memory during a run.
- **`api/handlers.go` still calls the non-error-returning `SaveResults`.** The error-returning
  `SaveResultsCtx` exists and is used by the store layer; the handler should be migrated so a
  persistence failure surfaces to the caller.
- **Transliteration is dictionary-based**, not a phonetic model — coverage is limited to the mapped
  pairs plus the synonym dictionary.
- **Demo users are compiled in.** There is no user management, registration, or password rotation.

See `BACKLOG.md` for the remediation backlog this codebase was built against.
