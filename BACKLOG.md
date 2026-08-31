# Remediation Backlog — Entity Matcher

Derived from the code review of commit `1a79ae5`. Ordered by dependency, not by epic number:
correctness of the decision layer first, then the measurement that proves it, then the
platform claims that are currently unbacked.

Severity: **C**ritical / **H**igh / **M**edium.

---

## Status — 2026-08-31

**57 of 59 items closed. The two that remain are open by decision, not by neglect.**

| Round | Epics | Items | State |
| :-- | :-- | --: | :-- |
| Round 1 | A–J | 41 | 39 closed; **A5** and **C5** deliberately left — see below |
| Round 2 | K, L, M, N | 18 | ✅ all closed |

### The two open items, and why they stay open

**A5 — score aggregation.** The AC asks for a configurable weighted mean over enabled metrics,
with `max` demoted to a tie-break. `scorer.go` still computes `max*0.6 + mean*0.4`, and the code
carries a measured weight sweep explaining why: token-sort is the only metric that fires on a
transposed name, so averaging destroys the signal the engine exists for. The sweep records
0.90 scoring higher still (100% top-1, 47.5% recall) and rejects it as fitting a benchmark with
weak negatives. **The argument in the code is stronger than the AC's**; changing it would trade a
measured decision for an unmeasured one.

**C5 — unspaced Thai.** The AC asks that single-token names fall back to trigram-dominant scoring.
There is no explicit branch, but the behaviour emerges — whitespace tokenization yields one token,
the token metric contributes nothing, and trigram carries the comparison. A Thai syllable segmenter
exists and serves romanization. The README documents the limitation. **The AC describes a mechanism
the system reaches another way**; implementing it literally would add a special case for behaviour
already present.

### Where the documentation lives

| Document | Holds |
| :-- | :-- |
| `README.md` | The authoritative measured numbers and the endpoint reference |
| `SPECIFICATION_ENTITY_MATCHER.md` | The original spec, annotated with delivery status; §6 lists what was built beyond it |
| `BACKLOG.md` | This file — the per-item remediation record and the Round 1 audit verdicts |

All three were reconciled on 2026-08-31 against a single measurement run, so the throughput,
peak-heap and scaling figures agree across them.

### What this round actually found

Four defects were **not** in any backlog entry, and every one of them was found by running the
system rather than reading it:

- A **live nil-deref crash** — searching results on any batch with an unmatched source panicked
  (`422b357`), found while scoping I3.
- **`GET /api/jobs` reported 0 sources and 0 destinations** for every job on PostgreSQL
  (`c711c44`), found by verifying the composed stack in M5.
- **Four README scale claims were wrong**, including a dataset size that never reconciled with its
  own throughput figure, and a scaling exponent of 1.12 published as 1.05 (`7392a03`).
- **N5's premise was false**: the "arbitrary SQL execution" it described had never been reachable,
  so the fix was to delete dead code, not to bound a live hole (`82c68ff`).

One pattern recurred four times — **a real implementation with no caller**: EPIC K's connectors,
H3's `ListJobs`, N1's missing `Close`, and I2's `GetResultByID`. Such code compiles, reviews
cleanly, and passes every test, because nothing exercises it. Worth a standing check whenever an
interface gains a method.

---

## EPIC A — Match Decision Layer (C) — ✅ closed except **A5** (open by decision)

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

## EPIC B — Benchmark & Test Integrity (C) — ✅ complete

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

## EPIC C — Normalizer Correctness (H) — ✅ closed except **C5** (partial, by decision)

| ID | Story | AC |
| :-- | :-- | :-- |
| C1 | Token-boundary title stripping | `Williams`→`williams`, `Adams`→`adams`, `Vincent Prince Inc.`→`vincent prince`, `ไทยนาย`preserved |
| C2 | Synonym dictionary wired into `Normalize()` | `KBank` and `กสิกรไทย` both normalize to `kasikornbank` and score 1.0 against each other |
| C3 | Date parsing | Buddhist-era years (2569→2026), multiple layouts, `time.Time{}` for unknown; unknown dates are neutral, not 1.0 |
| C4 | Rune-safe prefix + honest phonetic key | No byte-slicing of UTF-8; consonant-skeleton key that actually drops vowels in both scripts |
| C5 | Unspaced Thai | Single-token names fall back to trigram-dominant scoring |

## EPIC D — Security & RBAC (H) — ✅ complete

Currently: no middleware, every route open, `/api/auth/me` returns ADMIN without a token,
`password123` authenticates any account, hardcoded signing key, `Access-Control-Allow-Origin: *`.

| ID | Story | AC |
| :-- | :-- | :-- |
| D1 | Auth middleware + route policy | Every `/api/*` route except `login`/`health` requires a valid token; role policy enforced per route |
| D2 | Credential hardening | bcrypt hashes, `JWT_SECRET` from env (generated + warned in dev), universal password removed |
| D3 | Audit identity | `user_id` on audit entries comes from the verified token, never the request body |
| D4 | CORS allowlist | Origins from `CORS_ORIGINS`, default same-origin |
| D5 | Frontend auth | Login screen, token persistence, `Authorization` header on every call, role-gated navigation |

## EPIC E — Scheduler & Webhooks (H) — ✅ complete

`NewSchedulerManager()` is constructed inside the handler, so every POST is discarded; no
cron engine exists and `DispatchWebhook` is never called.

| ID | Story | AC |
| :-- | :-- | :-- |
| E1 | Singleton manager | Config survives POST→GET |
| E2 | Real cron engine | `robfig/cron/v3` triggers reconciliation on the configured expression; invalid expressions rejected at write time |
| E3 | Webhook dispatch | Fired on completion and on anomaly; Slack/Teams/generic payload shapes; bounded retry |
| E4 | Scheduler UI | Settings panel with cron validation and last-run status |

## EPIC F — Config Robustness (H) — ✅ complete

| ID | Story | AC |
| :-- | :-- | :-- |
| F1 | Merge-on-write + validation | Partial `PUT /api/config` preserves unspecified fields; thresholds bounded to [0,1]; `review <= auto`; weights must be positive |

## EPIC G — Connectors (H) — ✅ complete

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

## EPIC H — Persistence (M) — ✅ complete (was: spec EPIC 2, never started)

| ID | Story | AC |
| :-- | :-- | :-- |
| H1 | Schema + migrations | `match_jobs`, `match_results`, `match_audit_logs` tables created on boot |
| H2 | Repository abstraction | In-memory and Postgres implementations behind one interface; selected by `DATABASE_URL` |
| H3 | Job history API | `GET /api/jobs` lists historical runs with counters and duration |

## EPIC I — Scale & Performance (M) — ✅ complete

| ID | Story | AC |
| :-- | :-- | :-- |
| I1 | Cap trigram posting lists | Ultra-common trigrams skipped during query so per-source cost stays sub-linear |
| I2 | O(1) review actions | Results indexed by ID; no full-batch scan per click |
| I3 | Stop embedding record copies | Results hold IDs; records hydrated on read |

## EPIC J — Deployment & Docs (M) — ✅ complete

| ID | Story | AC |
| :-- | :-- | :-- |
| J1 | Dockerfile Go version | Builder matches `go.mod` |
| J2 | nginx `/api` proxy | SPA served by nginx reaches the backend |
| J3 | Port alignment | Compose and README agree |
| J4 | README truth pass | Every claim maps to working code; measured numbers replace aspirational ones |

---

## Round 1 audit (A–J), 2026-08-31

Per-item verdicts. The tables above carry the original ACs as written; this section records what
the code actually does. Verified against the code, not the table. **36 of 40 items are genuinely closed.** Four are not,
and two of those are the "real implementation with no caller" shape this project keeps hitting.

| Item | Verdict | Evidence |
| :-- | :-- | :-- |
| A1–A4, A6 | ✅ | `IsAutoMatchable`, `MarginThreshold`, `GREEDY_1_1`, `NO_MATCH`, `EmitUnmatched` all live |
| **A5** | ❌ **open by decision** | `scorer.go:510` still computes `max*0.6 + mean*0.4`. Not an oversight: an in-code weight sweep (0.15/0.30/0.60/0.90) records that averaging destroys the transposition signal the engine exists for, and that 0.90 scores better but was rejected to avoid fitting the benchmark. The AC — configurable weighted mean, `max` as tie-break only — is unmet, but the rationale is measured and documented |
| B1–B5 | ✅ | `backend/testdata` gone, `internal/mockdata` runs under `./...`; decision-level P/R/F1; 6 NEG_ categories; 11 api test files |
| C1–C4 | ✅ | title stripping, synonym dictionary wired into `Normalize`, Buddhist-era `-543`, rune-safe slicing |
| **C5** | ⚠️ **partial** | No explicit "single-token → trigram-dominant" branch. A Thai syllable segmenter exists and serves romanization, and unspaced Thai does lean on trigram as an emergent effect, which the README documents as a limitation. Not implemented as specified |
| D1–D5 | ✅ | 24 `RequireAuth` registrations; bcrypt per user with constant-time compare and a dummy hash against enumeration (`password123` is a seeded demo credential, not a bypass); audit identity from `ClaimsFrom(r.Context())`; `CORS_ORIGINS`; LoginScreen + `Authorization` header |
| E1–E4 | ✅ | singleton manager on `Server`; `robfig/cron/v3` with `ParseStandard` validation; `DispatchWebhook` called from two sites; SchedulerPanel |
| F1 | ✅ | `validateAndMergeConfig` merges and bounds |
| G1–G5 | ✅ | real pgx / go-mssqldb / mongo-driver / excelize; unreachable hosts fail honestly |
| H1–H3 | ✅ | `applySchema` on boot; `selectStore`; `/api/jobs` routed |
| I1 | ✅ | `maxPostingRatio` + `skipTrigrams` |
| ~~I2~~ ✅ | **Closed.** `GetResultByID` was implemented in both stores and called from nowhere while `HandleMatchAction` scanned the whole batch for one id — a 573k-row scan per review click at measured scale. Now an indexed lookup; not-found behaviour preserved exactly (no early 404, which would have changed the response for a case that returns 400 today). The test counts store calls rather than checking the action succeeded, because the outcome is identical either way: `getResultsCalls == 0` is the assertion, and it fails if the scan returns |
| ~~I3~~ ✅ | **Closed.** The pipeline embedded a record copy per row (~2.5 rows per source, so each record copied ~2.5×). Rows now carry IDs only and the stores attach records at save time as **pointers into the dataset they already hold** — 8 bytes a row instead of a struct copy — so every read path, the Postgres snapshot columns and the server-side search over them are untouched. Measured: peak heap **2,427.90 → 1,495.99 MiB** (−932 MiB, −38%), bytes/row 1,253 → 478, throughput +1.4%. Snapshots are deliberately kept: they are point-in-time evidence of what the reviewer saw, and they back the Postgres `LIKE` search, so they are now written at save time from the dataset instead of from the row |
| J1–J3 | ✅ | `golang:1.25-alpine` matches `go 1.25.0`; nginx `/api` proxy verified live during M5; compose and README both 8085/3000 |
| J4 | ✅ | truth pass ongoing and current — the scale figures were corrected in M6, and the one claim that could not be re-verified (~9% cross-script cost) is now explicitly marked a lower bound |

**Still open after this audit:** A5 (by decision, rationale recorded in `scorer.go`) and C5 (partial).
I2 and I3 were closed on 2026-08-31; a live nil-deref crash in the results search, found while
scoping I3, was fixed in `422b357`.

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

## EPIC K — Ingestion reaches only uploaded files (C) — ✅ complete

`NewDataConnector` is called from exactly two places: the multipart upload handler, and
`TestConnection`/`IntrospectSchema`. So the PostgreSQL, SQL Server and MongoDB drivers —
and the server-side `file_path` option in the UI — can be *tested* and *introspected*, but
cannot put a single row into a batch. Only an uploaded `.csv`/`.xlsx` can. This is the same
shape of defect as the one just fixed: a real implementation with no caller.

**Closed 2026-08-31.** K1+K2 `3520ee2`, K4 `2f59a27`, K3 in this commit. Deliberate
departures from the ACs:

- **`file_path` ingestion was NOT opened**, though this epic's text names it. `/api/upload/file`
  already ingests CSV/Excel from request bytes; accepting a server-side path on the new
  endpoint would let any ADMIN/ENGINEER read an arbitrary file *in full*, turning **M1**'s
  first-row disclosure into whole-file exfiltration through a brand-new route. The endpoint
  takes POSTGRES/SQLSERVER/MONGODB and 400s the rest. **Open `file_path` only after M1.**
- **The UI panel was unusable before K3**, not merely unwired — see the K3 row.
- **Truncation is now exact.** It is decided by asking for one more row, not by
  `len(rows) == cap`, so a source holding exactly 50,000 rows is no longer mislabelled as
  truncated. The paging helper is shared, so `/api/upload/file` stopped issuing a single
  50,000-row fetch too.

| ID | Story | AC |
| :-- | :-- | :-- |
| ~~K1~~ ✅ | Ingest from a configured connector | An endpoint accepts a saved `ConnectionConfig` and a `batch_id`, pages `FetchRecords` to exhaustion, and writes the batch; a match run then succeeds against it |
| ~~K2~~ ✅ | Paged ingestion, bounded | Ingestion pages rather than issuing one unbounded fetch; the row cap is explicit and a truncated ingest is reported, never silent (mirror the `truncated` flag on `/api/upload/file`) |
| ~~K3~~ ✅ | Connector ingestion in the UI | ConnectionManager's Connect path loads data and starts a batch, instead of only previewing headers. **Shipped with a prerequisite fix:** the panel had inputs for host, database and table only — no port, username or password — so every request it sent carried port 1433, user `sa` and a password of eight literal `•` characters. Test Connection and Introspect Schema had never been usable against a real server either. Added the three missing inputs, defaulted the port from the selected type, and cleared the fake password. Initially shipped with no automated test, since the project had no frontend test framework. That gap is now closed: Vitest + Testing Library added, with 12 tests over the store action and the ingest UI. Two are mutation-checked — reporting a truncated ingest as a success, and sending `port` as a string, each fail their own test |
| ~~K4~~ ✅ | `GET /api/jobs` | `ListJobs` is implemented in both stores and routed nowhere — no endpoint exists. Route it, with pagination bounds. Closes **H3**. **Shipped**, and it surfaced a store divergence now pinned by a test: `PostgresStore.SaveDataset` inserts a `match_jobs` row, so a batch uploaded but never matched *is* listed, while the in-memory store derives jobs from progress and does *not* list it. Which semantic is correct is a product question, deliberately left open rather than settled inside the endpoint |

## EPIC L — Cross-script matching stops at RTGS spellings (H) — ✅ complete

Characterized and pinned by `matcher/crossscript_regression_test.go`; not fixed. RTGS
romanizes จ as `ch`, while the conventional English spelling of such names uses `j`
(ใจดี → RTGS *chaidi*, written *Jaidee*). `PhoneticSkeleton` has no `j`↔`ch` equivalence, so
the skeletons diverge — `smchjd` vs `smchchd` — leaving that pair at 0.6906, just under the
0.70 review threshold. Measured effect on the benchmark: BILINGUAL_OUT_OF_DICT TP=2/FN=28
of 30, BILINGUAL_IN_DICTIONARY TP=2/FN=7 of 9, against 100/100 for same-script Thai.

**Closed 2026-08-31.** L1+L2 `d0f4d7a`. The entry's diagnosis was **incomplete**: it blamed
`PhoneticSkeleton`, but folding only the skeleton moved the score by *zero*. `romanizedScore`
is `min(skeletonScore, fullPartsScore)`, and `CrossScriptPartsScore` compares the
vowel-bearing romanization, where `jaidee` vs `chaidi` still scored 0.6667. The skeleton was
never the binding constraint. The fold now applies in both places.

**Selection rule for folds — fold only a letter RTGS never emits.** RTGS's alphabet contains
no `j` and no `v`, so folding those rewrites only the English side. `g` occurs only inside
`ng`, and `t`/`p` occur both alone and inside `th`/`ph`, so folding them corrupts the RTGS
side (*thongdi* → *thhongdi*) and merges ต/ท and ป/พ, which Thai contrasts. `t`→`th` and
`p`→`ph` each measured **+1 TP with no precision loss** — rejected anyway, because the
benchmark holds no pair that would expose the merged contrast, so the gain is fitting the
test set. Recorded in `rtgs.go` so they are not re-added on the strength of the numbers.

| ID | Story | AC |
| :-- | :-- | :-- |
| ~~L1~~ ✅ | Phonetic equivalence classes | `j`↔`ch` (and any sibling pairs found: `v`/`w`, `k`/`g`, `p`/`ph`, `t`/`th`) treated as equivalent **in comparison only** — romanization output stays RTGS-correct. The regression probe's RTGS guard must still pass |
| ~~L2~~ ✅ | Re-measure, do not assume | Re-run `TestFullLoopBigDatasetBenchmark`; precision must stay at 1.0000. The probe's `[0.60, 0.70)` band is expected to fail — that is the signal to update it, with the new numbers recorded |
| ~~L3~~ ✅ | Threshold option for cross-script pairs | 17 of 30 correct out-of-dict pairs already score in [0.70, 0.90). Evaluate a cross-script-specific auto threshold as a cheaper lever than L1, and measure both. **Measured** by `internal/mockdata/crossscript_threshold_test.go`. On this corpus the distributions separate cleanly: false friends max **0.6266**, correct out-of-dict pairs min **0.6819** (median 0.8400), and **0/52 false friends** cross any bar down to 0.70. **Not shipped, and the evidence is not sufficient on its own:** the sweep scores ground-truth pairings in isolation, so it is blind to the real precision risk — a source matching a *different* destination. The pipeline already shows `WrongScored>=Auto = 2` at 0.90. Shipping needs a cross-script flag on `MatchResultItem`, which today has none, plumbed through the scorer, schema and Postgres store. **Pipeline-level measurement added** (`crossscript_pipeline_sweep_test.go`), closing the isolation sweep's blind spot by running the real engine at each bar and judging against ground truth: `crossFP` is **0 at every threshold down to 0.75**, while `crossTP` rises 5 → 19. Collateral is entirely same-script — `sameFP` first appears at 0.82 and reaches 15 at 0.75 — which a cross-script-only policy would not cause. Counter proven non-vacuous by mutation (inverting the ground-truth test makes `crossFP` non-zero). **Shipped** as `cross_script_auto_threshold`, default **0.84** (`AutoThresholdFor` in `pipeline.go`), applied at BOTH decision sites — the initial one and the 1:1 re-evaluation, which overwrites it otherwise. Measured: cross-script TP 5 → 13, precision unchanged at 1.0000, false friends unchanged at TN=52/FP=0, recall 0.5902 → 0.5923, F1 0.7423 → 0.7440. `0` means unset and falls back to `auto_match_threshold` — a config persisted before this field existed deserializes as 0, and treating that as a real bar would auto-match every cross-script pair. Shipping it also made the sweep harness measure a constant (it varied the wrong knob); it now varies `CrossScriptAutoThreshold` with `AutoMatchThreshold` pinned, measuring the shipped policy directly, and asserts `crossTP` is not constant. **Remaining caveat:** the negatives are 52 synthetic false friends, not production data. The sweep shows `crossFP=0` down to 0.75, so 0.84 is deliberately conservative |

## EPIC M — Hardening and unfinished surface (M) — ✅ complete

| ID | Story | AC |
| :-- | :-- | :-- |
| ~~M1~~ ✅ | Restrict `IntrospectSchema` file paths | It opens any server-side path the caller supplies and returns the first row — an arbitrary file-read primitive, currently reachable by ADMIN/ENGINEER. Confine reads to a configured directory. **Shipped:** `CONNECTOR_FILE_ROOT` gates caller-supplied paths at both connector endpoints; unset denies. Symlinks are resolved *before* the containment check, and containment requires `root` or `root + separator` — a bare `HasPrefix` would accept `/data/private` under a root of `/data/priv`. All three properties are mutation-checked, as is the endpoint guard itself (removing it while leaving the helper tested fails the end-to-end test). **Unblocks the `file_path` half of K1**, which was deliberately left closed pending this |
| ~~M2~~ ✅ | Stream Excel | `ExcelConnector` uses `GetRows`, loading the whole sheet into memory. **G4**'s "real file streaming" holds for CSV only; use the streaming reader. **Shipped:** both `IntrospectSchema` and `FetchRecords` use excelize's row iterator with an early exit at `limit`. **The gain is smaller than it first appears.** With two distinct cell values a 5-row read allocates 1.9 MB against 47.5 MB (24×), but that fixture hides the real cost: excelize parses the shared-string table up front, which streaming cannot avoid. On realistic unique text — 20k rows — it is 49.5 MB against 95.5 MB, roughly **half**, not a 24× cut. Both regimes are asserted separately so the honest figure is in the test output, not just a commit message |
| ~~M3~~ ✅ | Let `SaveDataset` report failure | The `Repository` signature has no error return, so a write failure can only be logged. Give it an `error` and have callers surface it. **Shipped:** 12 previously-logged failure paths in `PostgresStore.SaveDataset` now return. The function uses a **named** error return so the existing deferred rollback still fires — converting `log; return` to `return fmt.Errorf(...)` without it would have turned a reported failure into a leaked open transaction. All four handlers respond 500. Mutation-checked: ignoring the error at a call site fails the test |
| ~~M4~~ ✅ | Calibration UI | `POST /api/calibration/fit` and `GET /api/calibration/status` have no frontend at all. Include observation progress toward `MinCalibrationObservations` (20), so an operator sees "14/20" rather than a bare 400. **Shipped** as an ADMIN-only panel with 7 tests. Two details beyond the AC: the server's selection-bias caveat renders **verbatim** rather than paraphrased, and the panel warns when `calibration_enabled` is false — a fit can otherwise show Brier 0.24 → 0.12 while affecting no scoring at all. The insufficient-observations response is plain text, not JSON; parsing it as JSON is mutation-checked to fail |
| ~~M5~~ ✅ | Verify the containerized stack | `DATABASE_URL` is honoured now, but persistence was verified against a host binary and a throwaway container, not through `docker-compose up`. Confirm the composed stack persists across `down`/`up`. **Verified 2026-08-31** against `docker compose up`: backend logged `Using PostgreSQL persistence`, nginx proxied `/api` (J2), and after `down` (volume retained) + `up` the counts were identical — jobs=1, results=3, sources=3, dests=3 — with a match **re-run successfully against the restored batch**, proving the dataset survived and not just the results. The build also revalidated the K3 lockfile fix, since both Dockerfiles run `npm ci`. **It found a real bug:** `GET /api/jobs` reported `total_sources: 0, total_destinations: 0` for a job that processed 3 of each — `SaveDataset` created the `match_jobs` row without the counts and every `UpdateProgress` took the `ON CONFLICT` path, whose `DO UPDATE SET` omitted `total_sources`; `total_destinations` was in no INSERT anywhere. PostgreSQL-only; the in-memory store derives both correctly. Fixed and confirmed live: 0/0 → 3/3 |
| ~~M6~~ ✅ | Re-measure scale on target hardware | Throughput, 220k×220k timing and peak heap were left untouched during the metrics correction — they come from the opt-in `SCALE_TEST` harness and are hardware-specific. **Re-measured 2026-08-31** on 20 cores / 121 GiB. Four README claims were wrong: the dataset is **230,120** per side, not 220,000 (the harness's `SCALE_N=100000` expands ~2.3×); throughput **8,450**/s not ~8,100; wall time **27.2s** not 28.3s; peak heap **2.40 GiB** not 2.5, and the limitations section said ~2 GiB. Scaling is **O(N^1.12)**, not the claimed O(N^1.05) — per-step exponents 1.16 / 1.01 / 1.20, least-squares 1.12 over four points. The benchmark's own figures (4,720 × 4,720, 22.3M pair space, 2.76 rows/source) were verified correct. **Left unverified:** the "~9% cross-script throughput" claim has no automated harness and predates EPIC L, which added a fold on that exact path; the README now says to treat it as a lower bound. **Superseded by I3:** removing the embedded record copies moved these again — throughput ~8,590/s, wall time 26.8s, peak heap 1.46 GiB, scaling O(N^1.09). The figures in this row are what M6 measured *before* that change; `README.md` holds the current set |

## EPIC N — Connector correctness (H) — ✅ complete

EPIC K covers the connectors being *unreachable* for ingestion. These are defects in the
connectors themselves, found by auditing the SQL Server path and comparing it against the
other two. They matter the moment K1 lands and real rows start flowing through this code.

**Closed 2026-08-31.** N1 `c666245`, N3+N4 `9a73f02`, N2 `446bb70` (PostgreSQL) and
`273451e` (SQL Server, MongoDB), N5 `82c68ff`. Every fix is mutation-checked: reverting it fails a test.
Two caveats a resuming reader should not have to rediscover —

- **SQL Server is not verified against a server.** No mssql image is available in this
  environment. Its schema qualification, schema-filtered introspection and page ordering
  are asserted over the *generated SQL*, which covers query construction and nothing more.
  PostgreSQL and MongoDB were verified against live instances.
- **MongoDB was added to N2**, which named only the two SQL connectors. `skip/limit` with
  no sort has the same defect, and fixing two of three would have left it live on the third.
- **Line numbers in the rows below refer to the pre-fix code** as it stood at `5b2658a`.
  They are kept as written so the original findings stay legible; they no longer
  resolve against the current file.

Worth stating plainly: the SQL Server connector is the *least* exposed of the three. It
parameterises its introspection query, pages with named parameters, and guards its single
interpolation point with `validateIdentifier` (`connector.go:1084`). N5 was about the
PostgreSQL connector, not this one — and turned out to be dead code rather than an
exposure; see its row below.

| ID | Story | AC |
| :-- | :-- | :-- |
| ~~N1~~ ✅ | Connectors release their connections | `TestConnection` opens a pool and stores it (`c.pool` `connector.go:118`, `c.conn` `:289`, `c.client` `:398`); `DataConnector` (`:54`) has no `Close`, and no handler closes one. Every `/api/connector/test` and `/api/connector/introspect` call leaks a pool for the life of the process. Add `Close() error` to the interface, implement it for all six connectors, and `defer` it at both call sites |
| ~~N2~~ ✅ | Deterministic paging in connectors | SQL Server pages with `ORDER BY (SELECT NULL)` (`connector.go:352`), which satisfies the `OFFSET/FETCH` syntax but guarantees no order; the PostgreSQL connector has no `ORDER BY` at all (`:227`). Both can therefore duplicate or drop rows across pages. Same defect class as `0fe270d` — order by a stable key (primary key, or the introspected first column) before paging. Blocks K2, which pages to exhaustion. **Shipped** with a stronger fallback than this AC proposed: explicit `extra_params.order_by` → primary key (in key order) → every btree-orderable column → hard error. Ordering by "the introspected first column" was rejected — a non-unique first column reintroduces the very nondeterminism this removes |
| ~~N3~~ ✅ | SQL Server schema-qualified tables | `validateIdentifier` rejects `.`, so `dbo.Customers` and `sales.Orders` are refused and only the login's default schema is reachable — in SQL Server, `dbo.`-qualification is the norm. The PostgreSQL connector already splits and validates both halves (`:205-217`); give SQL Server the same treatment |
| ~~N4~~ ✅ | SQL Server introspection must filter by schema | The query filters `WHERE TABLE_NAME = @TableName` with no `TABLE_SCHEMA` predicate (`:309`). Two schemas holding a same-named table return both column sets, merged and interleaved by `ORDINAL_POSITION` — wrong, and silent. Filter by schema once N3 supplies one |
| ~~N5~~ ✅ | Bound the PostgreSQL raw-SQL passthrough | **The premise of this entry was wrong.** It recorded a live hole — arbitrary SQL as the configured DB user. In both `IntrospectSchema` and `FetchRecords`, `validateIdentifier` runs *before* the `SELECT` branch, and its `^[A-Za-z_][A-Za-z0-9_]*$` rejects any query on its spaces, so the branch had never executed. Proven by running the passthrough tests against the committed code: all three failed with `invalid identifier: SELECT ...`. Dead code, not an exposure. Removed in `82c68ff` — free, since nothing can depend on a path that never ran; making it work would have *added* surface instead. A query datasource is now refused explicitly, before connecting |
