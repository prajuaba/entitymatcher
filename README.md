# High-Scale Thai–English Entity Resolution & Data Matching Engine

Entity resolution for Thai and English individuals and legal entities, with a human-in-the-loop
review queue, a compliance audit trail, and pluggable data connectors.

**Scope note:** the engine handles Thai-vs-Thai and English-vs-English matching well, including Thai
homophone spelling variants. Cross-script matching (the same entity in Thai script and Latin script)
retrieves candidates via RTGS romanization but does not yet auto-match them — see *Cross-script
matching* under Known limitations for the measurement and why.

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

From `go test ./internal/mockdata/` (4,720 × 4,720 synthetic bilingual records, 22.3M candidate
pair space, fixed seed). Scale figures are from the opt-in harness
(`SCALE_TEST=1 go test ./internal/mockdata/ -run TestScaleSweep`) at 220,000 × 220,000:

| Metric | Value |
| :-- | :-- |
| Throughput | ~8,100 sources/sec at 220k × 220k |
| Verified scale | 220,000 × 220,000 per side in 28.3s (20 cores), 2.5 GiB peak heap |
| Scaling | time ~ O(N^1.05) over 22k → 220k |
| Decision precision | 100.00% |
| Decision recall (auto-match) | 59.0% |
| F1 | 74.2% |
| Top-1 ranking accuracy | 98.84% |
| Candidate rows per source | 2.76 |
| Max destinations auto-matched per source | 1 |
| Max sources claiming one destination | 1 |

**Read these two numbers together.** Top-1 ranking accuracy (98.84%) is how often the correct
partner is ranked first — that is the quality of the *scorer*. Recall (59.0%) is how often the
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
| **Synonym dictionary** | Brand aliases applied inside the normalizer, so they affect scoring. Same-script aliases (`KBank` → `kasikornbank`) work; the Thai↔Latin entries do not, because such pairs are never retrieved — see Known limitations |
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

**Engine** — `GET|PUT /api/config`, `POST /api/upload` (JSON records),
`POST /api/upload/file` (multipart `.csv`/`.xlsx`/`.xls`), `POST /api/match/run`,
`GET /api/match/progress` (SSE), `GET /api/match/results`, `POST /api/match/action`,
`POST /api/match/manual-link`, `GET /api/destinations/search`, `GET /api/export/csv`,
`POST /api/llm/evaluate`

`POST /api/upload/file` takes `source_file` and `destination_file`, plus optional
`batch_id`, `column_mapping` (JSON) and `source_sheet`/`destination_sheet` for Excel.
Files are read through the same connectors used elsewhere. Requests are capped at 50 MiB
and 50,000 rows per file; hitting the row cap sets `truncated` and a warning in the
response rather than silently returning a short dataset.

**Connectors** — `POST /api/connector/test`, `POST /api/connector/introspect`

Database connectors page their reads, and a page is only meaningful over a total order, so
each resolves one before fetching: `extra_params.order_by` if the operator supplied it,
otherwise the table's primary key, otherwise every orderable column. MongoDB sorts on `_id`
by default. A table with no primary key and no orderable column cannot be paged safely, and
the connector says so instead of returning pages that quietly duplicate and drop rows — set
`extra_params.order_by` to resolve it. SQL Server tables may be schema-qualified
(`dbo.Customers`); an unqualified name is read as `dbo`.

**Calibration** (ADMIN) — `POST /api/calibration/fit`, `GET /api/calibration/status`

Fit trains on reviewer decisions in the audit log and reports Brier and ECE before and
after against a holdout. It requires at least 20 labelled observations: below that the
holdout is too small for those metrics to mean anything, and an empty holdout scores 0.0,
which is indistinguishable from a perfect calibrator. Both responses carry a caveat that
the training data comes almost entirely from the review queue, so any fitted calibrator is
well-calibrated for the review band and extrapolating outside it.

**Governance** — `GET /api/audit/logs`, `GET /api/audit/export`

**Operations** — `GET|POST /api/scheduler/config`, `GET|POST /api/dictionary`,
`POST /api/seed`, `POST /api/seed/big`, `GET /api/health`

---

## Known limitations

- **Thai word segmentation.** Tokenization is whitespace-based. Thai is frequently written without
  inter-word spaces, so unspaced company names lean on the trigram metric rather than token
  comparison. A dictionary-based segmenter would improve this.
- **Recall is threshold-bound.** 59.0% of true pairs are auto-matched at the default thresholds;
  the rest reach a human. This is a deliberate operating point, not a ceiling.
- **Peak heap is ~2 GiB at 220k × 220k**, and `docker-compose.yml` sets no memory limit. That is
  2% of a 121 GiB host but would OOM a typical 2 GiB container, so size the container for the
  corpus. Results accumulate in memory during a run and every row embeds full copies of both
  records (~630–960 bytes/row); profiling showed this costs memory and API payload size but *not*
  runtime, since GC CPU fraction falls with scale.
- **Cross-script retrieval costs ~9% throughput.** Measured by an interleaved A/B in one process
  (alternating `use_romanized_match` on/off, median of three pairs, 57,620 records per side):
  9,389 vs 10,326 sources/sec. The feature is behind that flag — both the scoring signal and the
  index construction — so a deployment that never mixes scripts can turn it off.

  **Methodology note, because this was got wrong once.** An earlier revision of this file claimed a
  42% cost. That came from comparing scale-sweep runs taken hours apart under different machine
  load, and the run-to-run spread under load (4,722–8,127 sources/sec for the same configuration)
  was wider than the effect being measured. The benchmark itself is deterministic — back-to-back
  runs are byte-identical — so comparing code versions requires an interleaved A/B in one load
  window, not two timestamps.

- **Scaling is O(N^1.05), not linear.** The residual comes from the candidate set growing with
  corpus size; bounded top-K selection caps its sort cost but does not eliminate it.
- **Cross-script matching retrieves but does not decide.** Thai-script records and Latin-script
  records are now matched through RTGS romanization: a Thai syllable segmenter determines consonant
  position, `RomanizeThai` produces the romanization, and a shared phonetic skeleton puts both
  scripts in one space that the blocking index and the scorer both use. Retrieval works — for
  entities in the transliteration map, unretrieved pairs went from 7 of 9 to **0**; for entities
  outside it, from 20 of 30 to **3**, with mean score rising from 0.000 to 0.755.

  Auto-matching still does not fire for out-of-dictionary pairs, and the measurement shows this is a
  property of the feature rather than of the threshold:

      OUT_OF_DICT true pairs     n=16  min 0.705  p50 0.761  max 0.800  mean 0.755
      FALSE_FRIEND must-reject   n=41  min 0.702  p50 0.762  max 0.828  mean 0.761

  The distributions overlap completely and the false friends average *higher*. No threshold
  separates them; lowering `auto_match_threshold` would admit more wrong matches than right ones.
  The cause is that the phonetic skeleton drops vowels, and Thai names draw on a small consonant
  inventory, so สมชาย/Somsak (different people) and สุชาติ ประเจริญ/Suchat Prachaerin (same person)
  look equally alike once vowels are gone. Comparing full romanizations with vowels retained is the
  next lever; `NEG_BILINGUAL_FALSE_FRIEND` (n=52) is in place to measure whether it holds precision.

  The syllable segmenter is 6-of-8 accurate on its target cases. The two failures are
  Sanskrit/Pali-derived words where a written vowel is silent (ประเสริฐ, สุชาติ); resolving those
  needs a lexicon, and the correct expectations are kept in the test table rather than fitted to the
  implementation.

- **Demo users are compiled in.** There is no user management, registration, or password rotation.

See `BACKLOG.md` for the remediation backlog this codebase was built against.
