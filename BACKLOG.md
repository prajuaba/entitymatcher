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

---

# Round 2 — remaining after `f1a65a0..8198e03`

Derived from working the calibration, file-ingestion and PostgreSQL branch, not from a
fresh review. Every claim below was verified against the code or a running system on
2026-08-30; where a number appears, it was measured rather than estimated.

## Closed by that branch

| Item | Evidence |
| :-- | :-- |
| **H2** — repository selected by `DATABASE_URL` | `selectStore` in `main.go`; verified end to end — upload, match, restart, data still present |
| **G5** (CSV/Excel half) — honest failure | `CSVConnector`/`ExcelConnector.TestConnection` now open and parse the file instead of checking the path is non-empty |
| **J4** (partial) — README truth pass | Endpoint list completed; decision metrics re-measured (recall 57.6% → 59.0%, F1 73.1% → 74.2%, top-1 99.19% → 98.84%, rows/source 3.15 → 2.76) |

`GetResults`, `GetResultsPage`, `ListJobs`, `ListBatches`, `GetAuditLogs` and
`ListCalibrationModels` all sorted on a non-unique column with no tiebreaker; the three
paginated ones could duplicate or drop rows across pages. Fixed in `0fe270d`.

---

## EPIC K — Ingestion reaches only uploaded files (C)

`NewDataConnector` is called from exactly two places: the multipart upload handler, and
`TestConnection`/`IntrospectSchema`. So the PostgreSQL, SQL Server and MongoDB drivers —
and the server-side `file_path` option in the UI — can be *tested* and *introspected*, but
cannot put a single row into a batch. Only an uploaded `.csv`/`.xlsx` can. This is the same
shape of defect as the one just fixed: a real implementation with no caller.

| ID | Story | AC |
| :-- | :-- | :-- |
| K1 | Ingest from a configured connector | An endpoint accepts a saved `ConnectionConfig` and a `batch_id`, pages `FetchRecords` to exhaustion, and writes the batch; a match run then succeeds against it |
| K2 | Paged ingestion, bounded | Ingestion pages rather than issuing one unbounded fetch; the row cap is explicit and a truncated ingest is reported, never silent (mirror the `truncated` flag on `/api/upload/file`) |
| K3 | Connector ingestion in the UI | ConnectionManager's Connect path loads data and starts a batch, instead of only previewing headers |
| K4 | `GET /api/jobs` | `ListJobs` is implemented in both stores and routed nowhere — no endpoint exists. Route it, with pagination bounds. Closes **H3** |

## EPIC L — Cross-script matching stops at RTGS spellings (H)

Characterized and pinned by `matcher/crossscript_regression_test.go`; not fixed. RTGS
romanizes จ as `ch`, while the conventional English spelling of such names uses `j`
(ใจดี → RTGS *chaidi*, written *Jaidee*). `PhoneticSkeleton` has no `j`↔`ch` equivalence, so
the skeletons diverge — `smchjd` vs `smchchd` — leaving that pair at 0.6906, just under the
0.70 review threshold. Measured effect on the benchmark: BILINGUAL_OUT_OF_DICT TP=2/FN=28
of 30, BILINGUAL_IN_DICTIONARY TP=2/FN=7 of 9, against 100/100 for same-script Thai.

| ID | Story | AC |
| :-- | :-- | :-- |
| L1 | Phonetic equivalence classes | `j`↔`ch` (and any sibling pairs found: `v`/`w`, `k`/`g`, `p`/`ph`, `t`/`th`) treated as equivalent **in comparison only** — romanization output stays RTGS-correct. The regression probe's RTGS guard must still pass |
| L2 | Re-measure, do not assume | Re-run `TestFullLoopBigDatasetBenchmark`; precision must stay at 1.0000. The probe's `[0.60, 0.70)` band is expected to fail — that is the signal to update it, with the new numbers recorded |
| L3 | Threshold option for cross-script pairs | 17 of 30 correct out-of-dict pairs already score in [0.70, 0.90). Evaluate a cross-script-specific auto threshold as a cheaper lever than L1, and measure both |

## EPIC M — Hardening and unfinished surface (M)

| ID | Story | AC |
| :-- | :-- | :-- |
| M1 | Restrict `IntrospectSchema` file paths | It opens any server-side path the caller supplies and returns the first row — an arbitrary file-read primitive, currently reachable by ADMIN/ENGINEER. Confine reads to a configured directory |
| M2 | Stream Excel | `ExcelConnector` uses `GetRows`, loading the whole sheet into memory. **G4**'s "real file streaming" holds for CSV only; use the streaming reader |
| M3 | Let `SaveDataset` report failure | The `Repository` signature has no error return, so a write failure can only be logged. Give it an `error` and have callers surface it |
| M4 | Calibration UI | `POST /api/calibration/fit` and `GET /api/calibration/status` have no frontend at all. Include observation progress toward `MinCalibrationObservations` (20), so an operator sees "14/20" rather than a bare 400 |
| M5 | Verify the containerized stack | `DATABASE_URL` is honoured now, but persistence was verified against a host binary and a throwaway container, not through `docker-compose up`. Confirm the composed stack persists across `down`/`up` |
| M6 | Re-measure scale on target hardware | Throughput, 220k×220k timing and peak heap were left untouched during the metrics correction — they come from the opt-in `SCALE_TEST` harness and are hardware-specific |

## EPIC N — Connector correctness (H) — N1–N4 closed, N5 open

EPIC K covers the connectors being *unreachable* for ingestion. These are defects in the
connectors themselves, found by auditing the SQL Server path and comparing it against the
other two. They matter the moment K1 lands and real rows start flowing through this code.

**Closed 2026-08-31.** N1 `c666245`, N3+N4 `9a73f02`, N2 `446bb70` (PostgreSQL) and
`273451e` (SQL Server, MongoDB). Every fix is mutation-checked: reverting it fails a test.
Two caveats a resuming reader should not have to rediscover —

- **SQL Server is not verified against a server.** No mssql image is available in this
  environment. Its schema qualification, schema-filtered introspection and page ordering
  are asserted over the *generated SQL*, which covers query construction and nothing more.
  PostgreSQL and MongoDB were verified against live instances.
- **MongoDB was added to N2**, which named only the two SQL connectors. `skip/limit` with
  no sort has the same defect, and fixing two of three would have left it live on the third.

Worth stating plainly: the SQL Server connector is the *least* exposed of the three. It
parameterises its introspection query, pages with named parameters, and guards its single
interpolation point with `validateIdentifier` (`connector.go:756`). N5 below is about the
PostgreSQL connector, not this one.

| ID | Story | AC |
| :-- | :-- | :-- |
| ~~N1~~ ✅ | Connectors release their connections | `TestConnection` opens a pool and stores it (`c.pool` `connector.go:118`, `c.conn` `:289`, `c.client` `:398`); `DataConnector` (`:54`) has no `Close`, and no handler closes one. Every `/api/connector/test` and `/api/connector/introspect` call leaks a pool for the life of the process. Add `Close() error` to the interface, implement it for all six connectors, and `defer` it at both call sites |
| ~~N2~~ ✅ | Deterministic paging in connectors | SQL Server pages with `ORDER BY (SELECT NULL)` (`connector.go:352`), which satisfies the `OFFSET/FETCH` syntax but guarantees no order; the PostgreSQL connector has no `ORDER BY` at all (`:227`). Both can therefore duplicate or drop rows across pages. Same defect class as `0fe270d` — order by a stable key (primary key, or the introspected first column) before paging. Blocks K2, which pages to exhaustion. **Shipped** with a stronger fallback than this AC proposed: explicit `extra_params.order_by` → primary key (in key order) → every btree-orderable column → hard error. Ordering by "the introspected first column" was rejected — a non-unique first column reintroduces the very nondeterminism this removes |
| ~~N3~~ ✅ | SQL Server schema-qualified tables | `validateIdentifier` rejects `.`, so `dbo.Customers` and `sales.Orders` are refused and only the login's default schema is reachable — in SQL Server, `dbo.`-qualification is the norm. The PostgreSQL connector already splits and validates both halves (`:205-217`); give SQL Server the same treatment |
| ~~N4~~ ✅ | SQL Server introspection must filter by schema | The query filters `WHERE TABLE_NAME = @TableName` with no `TABLE_SCHEMA` predicate (`:309`). Two schemas holding a same-named table return both column sets, merged and interleaved by `ORDINAL_POSITION` — wrong, and silent. Filter by schema once N3 supplies one |
| **N5** | Bound the PostgreSQL raw-SQL passthrough | When `TableOrQuery` begins with `SELECT`, the PostgreSQL connector concatenates it verbatim (`:146`, `:222`), so any ADMIN/ENGINEER can execute arbitrary SQL as the configured database user. That may be the intent of a query datasource, but it is currently undocumented and unbounded. Decide deliberately: document and keep it read-only (enforce a read-only session/role), or drop it. SQL Server and MongoDB have no equivalent path |
