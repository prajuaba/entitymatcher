package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"entitymatcher/matcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 1. ResultsQuery.Normalized() -- pure unit test, no database.
// ---------------------------------------------------------------------------

func TestResultsQueryNormalized(t *testing.T) {
	allSortBys := []string{
		SortByCreatedAt, SortByConfidence, SortByNameScore, SortByDateScore,
		SortByStatus, SortBySourceName, SortByReferenceID,
	}

	tests := []struct {
		name       string
		in         ResultsQuery
		wantSortBy string
		wantDir    string
		wantLimit  int
		wantOffset int
		wantSearch string
	}{
		{
			name:       "empty sort_by falls back to created_at",
			in:         ResultsQuery{SortBy: ""},
			wantSortBy: SortByCreatedAt,
			wantDir:    "asc",
			wantLimit:  DefaultResultsPageSize,
		},
		{
			name:       "unknown sort_by falls back to created_at",
			in:         ResultsQuery{SortBy: "id;DROP TABLE"},
			wantSortBy: SortByCreatedAt,
			wantDir:    "asc",
			wantLimit:  DefaultResultsPageSize,
		},
		{
			name:       "SortDir is lowercased",
			in:         ResultsQuery{SortBy: SortByConfidence, SortDir: "DESC"},
			wantSortBy: SortByConfidence,
			wantDir:    "desc",
			wantLimit:  DefaultResultsPageSize,
		},
		{
			name:       "non-desc SortDir becomes asc",
			in:         ResultsQuery{SortBy: SortByConfidence, SortDir: "sideways"},
			wantSortBy: SortByConfidence,
			wantDir:    "asc",
			wantLimit:  DefaultResultsPageSize,
		},
		{
			name:       "zero limit becomes default",
			in:         ResultsQuery{Limit: 0},
			wantSortBy: SortByCreatedAt,
			wantDir:    "asc",
			wantLimit:  DefaultResultsPageSize,
		},
		{
			name:       "negative limit becomes default",
			in:         ResultsQuery{Limit: -5},
			wantSortBy: SortByCreatedAt,
			wantDir:    "asc",
			wantLimit:  DefaultResultsPageSize,
		},
		{
			name:       "limit above max clamps to max",
			in:         ResultsQuery{Limit: MaxResultsPageSize + 1000},
			wantSortBy: SortByCreatedAt,
			wantDir:    "asc",
			wantLimit:  MaxResultsPageSize,
		},
		{
			name:       "negative offset becomes zero",
			in:         ResultsQuery{Offset: -10},
			wantSortBy: SortByCreatedAt,
			wantDir:    "asc",
			wantLimit:  DefaultResultsPageSize,
			wantOffset: 0,
		},
		{
			name:       "search is trimmed",
			in:         ResultsQuery{Search: "   hello world   "},
			wantSortBy: SortByCreatedAt,
			wantDir:    "asc",
			wantLimit:  DefaultResultsPageSize,
			wantSearch: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.Normalized()
			assert.Equal(t, tt.wantSortBy, got.SortBy)
			assert.Equal(t, tt.wantDir, got.SortDir)
			assert.Equal(t, tt.wantLimit, got.Limit)
			assert.Equal(t, tt.wantOffset, got.Offset)
			assert.Equal(t, tt.wantSearch, got.Search)
		})
	}

	// Each of the seven valid SortBy values is preserved as-is.
	for _, sb := range allSortBys {
		t.Run("preserves valid SortBy "+sb, func(t *testing.T) {
			got := ResultsQuery{SortBy: sb}.Normalized()
			assert.Equal(t, sb, got.SortBy)
		})
	}
}

// ---------------------------------------------------------------------------
// 2. Every sort field actually executes against real Postgres.
// ---------------------------------------------------------------------------

// allSortByConstants centralizes the seven SortBy* constants so a new one
// added to repository.go without updating this list makes the coverage gap
// obvious (rather than silently under-testing).
var allSortByConstants = []string{
	SortByCreatedAt, SortByConfidence, SortByNameScore, SortByDateScore,
	SortByStatus, SortBySourceName, SortByReferenceID,
}

func TestPostgresGetResultsPageAllSortFieldsExecute(t *testing.T) {
	store := testPostgresStore(t)
	batchID := fmt.Sprintf("batch-allsorts-%d", time.Now().UnixNano())

	// Distinct values on every sortable field, five rows.
	names := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	refs := []string{"REF-A", "REF-B", "REF-C", "REF-D", "REF-E"}
	confidences := []float64{0.30, 0.50, 0.10, 0.90, 0.70}
	nameScores := []float64{0.11, 0.22, 0.33, 0.44, 0.55}
	dateScores := []float64{0.60, 0.50, 0.40, 0.30, 0.20}
	statuses := []string{"AUTO_MATCHED", "REVIEW_NEEDED", "CONFIRMED", "REJECTED", "NO_MATCH"}
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	sources := make([]matcher.SourceRecord, 5)
	results := make([]matcher.MatchResultItem, 5)
	for i := 0; i < 5; i++ {
		srcID := fmt.Sprintf("src-%d", i)
		sources[i] = matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     refs[i],
			CustomerNameRaw: names[i],
		}
		results[i] = matcher.MatchResultItem{
			ID:              fmt.Sprintf("result-%d", i),
			BatchID:         batchID,
			SourceID:        srcID,
			DestinationID:   "",
			ConfidenceScore: confidences[i],
			NameScore:       nameScores[i],
			DateScore:       dateScores[i],
			MatchStatus:     statuses[i],
			CreatedAt:       baseTime.Add(time.Duration(i) * time.Second),
		}
	}

	require.NoError(t, store.SaveDataset(batchID, sources, nil))
	require.NoError(t, store.SaveResultsCtx(context.Background(), batchID, results))

	for _, sortBy := range allSortByConstants {
		for _, dir := range []string{"asc", "desc"} {
			t.Run(fmt.Sprintf("%s_%s", sortBy, dir), func(t *testing.T) {
				page, total, err := store.GetResultsPage(ResultsQuery{
					BatchID: batchID,
					SortBy:  sortBy,
					SortDir: dir,
					Limit:   100,
				})
				require.NoError(t, err)
				assert.Equal(t, 5, total)
				assert.Len(t, page, 5)
			})
		}
	}

	// Explicit order check: confidence_score desc.
	page, _, err := store.GetResultsPage(ResultsQuery{
		BatchID: batchID, SortBy: SortByConfidence, SortDir: "desc", Limit: 100,
	})
	require.NoError(t, err)
	gotIDs := idsOf(page)
	assert.Equal(t, []string{"result-3", "result-4", "result-1", "result-0", "result-2"}, gotIDs)

	// Explicit order check: source_name asc.
	page, _, err = store.GetResultsPage(ResultsQuery{
		BatchID: batchID, SortBy: SortBySourceName, SortDir: "asc", Limit: 100,
	})
	require.NoError(t, err)
	gotIDs = idsOf(page)
	assert.Equal(t, []string{"result-0", "result-1", "result-2", "result-3", "result-4"}, gotIDs)
}

func idsOf(items []matcher.MatchResultItem) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}

// ---------------------------------------------------------------------------
// 3. Sorting is stable and paging is complete when the sort key TIES.
// ---------------------------------------------------------------------------

func TestPostgresGetResultsPageTiedConfidenceCompletePaging(t *testing.T) {
	store := testPostgresStore(t)
	batchID := fmt.Sprintf("batch-tieconf-%d", time.Now().UnixNano())

	const n = 9
	results := make([]matcher.MatchResultItem, n)
	for i := 0; i < n; i++ {
		results[i] = matcher.MatchResultItem{
			ID:              fmt.Sprintf("tie-conf-%d", i),
			BatchID:         batchID,
			SourceID:        "src",
			DestinationID:   "dst",
			ConfidenceScore: 0.5, // all tied
			MatchStatus:     "REVIEW_NEEDED",
			CreatedAt:       time.Now(),
		}
	}
	require.NoError(t, store.SaveResultsCtx(context.Background(), batchID, results))

	seen := map[string]int{}
	for offset := 0; offset < n; offset += 3 {
		page, total, err := store.GetResultsPage(ResultsQuery{
			BatchID: batchID, SortBy: SortByConfidence, SortDir: "desc", Limit: 3, Offset: offset,
		})
		require.NoError(t, err)
		assert.Equal(t, n, total)
		for _, item := range page {
			seen[item.ID]++
		}
	}

	require.Len(t, seen, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("tie-conf-%d", i)
		assert.Equal(t, 1, seen[id], "id %s should appear exactly once", id)
	}
}

func TestPostgresGetResultsPageTiedStatusCompletePaging(t *testing.T) {
	store := testPostgresStore(t)
	batchID := fmt.Sprintf("batch-tiestatus-%d", time.Now().UnixNano())

	const n = 9
	results := make([]matcher.MatchResultItem, n)
	for i := 0; i < n; i++ {
		results[i] = matcher.MatchResultItem{
			ID:              fmt.Sprintf("tie-status-%d", i),
			BatchID:         batchID,
			SourceID:        "src",
			DestinationID:   "dst",
			ConfidenceScore: float64(i) / 10,
			MatchStatus:     "REVIEW_NEEDED", // all tied
			CreatedAt:       time.Now(),
		}
	}
	require.NoError(t, store.SaveResultsCtx(context.Background(), batchID, results))

	seen := map[string]int{}
	for offset := 0; offset < n; offset += 3 {
		page, total, err := store.GetResultsPage(ResultsQuery{
			BatchID: batchID, SortBy: SortByStatus, SortDir: "asc", Limit: 3, Offset: offset,
		})
		require.NoError(t, err)
		assert.Equal(t, n, total)
		for _, item := range page {
			seen[item.ID]++
		}
	}

	require.Len(t, seen, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("tie-status-%d", i)
		assert.Equal(t, 1, seen[id], "id %s should appear exactly once", id)
	}
}

// ---------------------------------------------------------------------------
// 4. Search, against real Postgres.
// ---------------------------------------------------------------------------

func TestPostgresGetResultsPageSearch(t *testing.T) {
	store := testPostgresStore(t)
	batchID := fmt.Sprintf("batch-search-%d", time.Now().UnixNano())

	sources := []matcher.SourceRecord{
		{
			ID:              "search-src-1",
			BatchID:         batchID,
			ReferenceID:     "refsearchone",
			CustomerNameRaw: "MixedCaseSourceName",
			Attributes:      map[string]interface{}{"internal_note": "ZZZHIDDENTOKENZZZ"},
		},
	}
	dests := []matcher.DestinationRecord{
		{
			ID:              "search-dst-1",
			BatchID:         batchID,
			CustomerID:      "CUST-SEARCH-99",
			CustomerNameRaw: "DestinationCorpName",
		},
	}
	results := []matcher.MatchResultItem{
		{
			ID:            "search-result-1",
			BatchID:       batchID,
			SourceID:      "search-src-1",
			DestinationID: "search-dst-1",
			MatchStatus:   "AUTO_MATCHED",
			CreatedAt:     time.Now(),
		},
	}

	require.NoError(t, store.SaveDataset(batchID, sources, dests))
	require.NoError(t, store.SaveResultsCtx(context.Background(), batchID, results))

	cases := []struct {
		name      string
		search    string
		wantCount int
	}{
		{"matches source customer_name_raw", "MixedCaseSourceName", 1},
		{"matches source reference_id", "refsearchone", 1},
		{"matches destination customer_name_raw", "DestinationCorpName", 1},
		{"matches destination customer_id", "CUST-SEARCH-99", 1},
		{"case-insensitive: lowercase search for mixed-case stored name", "mixedcasesourcename", 1},
		{"case-insensitive: uppercase search for lowercase stored ref", "REFSEARCHONE", 1},
		{"no match anywhere", "totallyabsentterm123", 0},
		{"matches only inside attributes, not a searchable field", "ZZZHIDDENTOKENZZZ", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, total, err := store.GetResultsPage(ResultsQuery{BatchID: batchID, Search: tc.search, Limit: 100})
			require.NoError(t, err)
			assert.Equal(t, tc.wantCount, total)
			assert.Len(t, page, tc.wantCount)
		})
	}
}

// ---------------------------------------------------------------------------
// 5. A row with a nil Destination must NOT match an arbitrary search term.
// ---------------------------------------------------------------------------

func TestPostgresGetResultsPageNilDestinationDoesNotFalselyMatch(t *testing.T) {
	store := testPostgresStore(t)
	batchID := fmt.Sprintf("batch-nildest-pg-%d", time.Now().UnixNano())

	sources := []matcher.SourceRecord{
		{ID: "nodest-src-1", BatchID: batchID, ReferenceID: "REF-NODEST", CustomerNameRaw: "OnlySourceNoMatch"},
	}
	results := []matcher.MatchResultItem{
		{
			ID:            "nodest-result-1",
			BatchID:       batchID,
			SourceID:      "nodest-src-1",
			DestinationID: "", // NO_MATCH: legitimately no destination
			MatchStatus:   "NO_MATCH",
			CreatedAt:     time.Now(),
		},
	}

	require.NoError(t, store.SaveDataset(batchID, sources, nil))
	require.NoError(t, store.SaveResultsCtx(context.Background(), batchID, results))

	// A term present nowhere must return 0 rows -- not a false positive from the nil destination.
	page, total, err := store.GetResultsPage(ResultsQuery{BatchID: batchID, Search: "termnotpresentanywhere", Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, page)

	// The NO_MATCH row's own source name must still be found.
	page, total, err = store.GetResultsPage(ResultsQuery{BatchID: batchID, Search: "OnlySourceNoMatch", Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, page, 1)
	assert.Equal(t, "nodest-result-1", page[0].ID)
}

func TestMemoryStoreGetResultsPageNilDestinationDoesNotFalselyMatch(t *testing.T) {
	memStore := NewStore()
	batchID := "batch-nildest-mem-1"

	sources := []matcher.SourceRecord{
		{ID: "nodest-src-1", BatchID: batchID, ReferenceID: "REF-NODEST", CustomerNameRaw: "OnlySourceNoMatch"},
	}
	require.NoError(t, memStore.SaveDataset(batchID, sources, nil))

	results := []matcher.MatchResultItem{
		{
			ID:            "nodest-result-1",
			BatchID:       batchID,
			SourceID:      "nodest-src-1",
			DestinationID: "", // NO_MATCH: legitimately no destination
			MatchStatus:   "NO_MATCH",
			CreatedAt:     time.Now(),
		},
	}
	require.NoError(t, memStore.SaveResultsCtx(context.Background(), batchID, results))

	// This is the regression test: a nil Destination pointer must not count as an
	// automatic match for ANY search term.
	page, total, err := memStore.GetResultsPage(ResultsQuery{BatchID: batchID, Search: "termnotpresentanywhere", Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, page)

	page, total, err = memStore.GetResultsPage(ResultsQuery{BatchID: batchID, Search: "OnlySourceNoMatch", Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, page, 1)
	assert.Equal(t, "nodest-result-1", page[0].ID)
}

// ---------------------------------------------------------------------------
// 6. Postgres and the in-memory store agree.
// ---------------------------------------------------------------------------

// buildCrossStoreFixture returns freshly-allocated sources/results each call so
// two stores sharing a batch ID don't alias the same backing arrays (the memory
// store mutates result items in place to hydrate Source/Destination pointers).
func buildCrossStoreFixture(batchID string) ([]matcher.SourceRecord, []matcher.MatchResultItem) {
	baseTime := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	type row struct {
		id         string
		name       string
		ref        string
		confidence float64
		nameScore  float64
		dateScore  float64
		status     string
	}
	rows := []row{
		{"r-a", "alice", "ref-a", 0.10, 0.15, 0.90, "AUTO_MATCHED"},
		{"r-b", "bob", "ref-b", 0.40, 0.55, 0.30, "CONFIRMED"},
		{"r-c", "charlie", "ref-c", 0.70, 0.35, 0.60, "NO_MATCH"},
		{"r-d", "dave", "ref-d", 0.20, 0.75, 0.10, "REJECTED"},
	}

	sources := make([]matcher.SourceRecord, len(rows))
	results := make([]matcher.MatchResultItem, len(rows))
	for i, r := range rows {
		srcID := "src-" + r.id
		sources[i] = matcher.SourceRecord{
			ID:              srcID,
			BatchID:         batchID,
			ReferenceID:     r.ref,
			CustomerNameRaw: r.name,
		}
		results[i] = matcher.MatchResultItem{
			ID:              r.id,
			BatchID:         batchID,
			SourceID:        srcID,
			DestinationID:   "",
			ConfidenceScore: r.confidence,
			NameScore:       r.nameScore,
			DateScore:       r.dateScore,
			MatchStatus:     r.status,
			CreatedAt:       baseTime.Add(time.Duration(i) * time.Second),
		}
	}
	return sources, results
}

func TestPostgresAndMemoryStoreAgreeOnSortOrder(t *testing.T) {
	pg := testPostgresStore(t)
	mem := NewStore()

	batchID := fmt.Sprintf("batch-crossstore-%d", time.Now().UnixNano())

	pgSources, pgResults := buildCrossStoreFixture(batchID)
	require.NoError(t, pg.SaveDataset(batchID, pgSources, nil))
	require.NoError(t, pg.SaveResultsCtx(context.Background(), batchID, pgResults))

	memSources, memResults := buildCrossStoreFixture(batchID)
	require.NoError(t, mem.SaveDataset(batchID, memSources, nil))
	require.NoError(t, mem.SaveResultsCtx(context.Background(), batchID, memResults))

	sortFields := []string{
		SortByConfidence, SortByDateScore, SortByNameScore, SortByCreatedAt,
		SortByStatus, SortBySourceName, SortByReferenceID,
	}

	for _, sortBy := range sortFields {
		for _, dir := range []string{"asc", "desc"} {
			t.Run(fmt.Sprintf("%s_%s", sortBy, dir), func(t *testing.T) {
				q := ResultsQuery{BatchID: batchID, SortBy: sortBy, SortDir: dir, Limit: 100}

				pgPage, _, err := pg.GetResultsPage(q)
				require.NoError(t, err)

				memPage, _, err := mem.GetResultsPage(q)
				require.NoError(t, err)

				assert.Equal(t, idsOf(memPage), idsOf(pgPage), "sortBy=%s dir=%s", sortBy, dir)
			})
		}
	}

	// Pin the exact expected order for confidence desc on both stores, so
	// agreement isn't merely "both happen to be wrong the same way".
	want := []string{"r-c", "r-b", "r-d", "r-a"}
	q := ResultsQuery{BatchID: batchID, SortBy: SortByConfidence, SortDir: "desc", Limit: 100}
	pgPage, _, err := pg.GetResultsPage(q)
	require.NoError(t, err)
	memPage, _, err := mem.GetResultsPage(q)
	require.NoError(t, err)
	assert.Equal(t, want, idsOf(pgPage))
	assert.Equal(t, want, idsOf(memPage))
}

// ---------------------------------------------------------------------------
// 7. CountResultsByStatus on both stores.
// ---------------------------------------------------------------------------

func seedStatusCountFixture(t *testing.T, repo Repository, batchID string) {
	t.Helper()

	sources := []matcher.SourceRecord{
		{ID: "cnt-src-1", BatchID: batchID, ReferenceID: "REF-CNT-1", CustomerNameRaw: "CountableAlpha"},
		{ID: "cnt-src-2", BatchID: batchID, ReferenceID: "REF-CNT-2", CustomerNameRaw: "CountableBravo"},
		{ID: "cnt-src-3", BatchID: batchID, ReferenceID: "REF-CNT-3", CustomerNameRaw: "Unrelated"},
	}
	require.NoError(t, repo.SaveDataset(batchID, sources, nil))

	results := []matcher.MatchResultItem{
		{ID: "cnt-1", BatchID: batchID, SourceID: "cnt-src-1", MatchStatus: "AUTO_MATCHED", CreatedAt: time.Now()},
		{ID: "cnt-2", BatchID: batchID, SourceID: "cnt-src-1", MatchStatus: "AUTO_MATCHED", CreatedAt: time.Now()},
		{ID: "cnt-3", BatchID: batchID, SourceID: "cnt-src-2", MatchStatus: "REVIEW_NEEDED", CreatedAt: time.Now()},
		{ID: "cnt-4", BatchID: batchID, SourceID: "cnt-src-3", MatchStatus: "REJECTED", CreatedAt: time.Now()},
	}
	require.NoError(t, repo.SaveResultsCtx(context.Background(), batchID, results))
}

func TestCountResultsByStatus(t *testing.T) {
	stores := map[string]Repository{
		"postgres": testPostgresStore(t),
		"memory":   NewStore(),
	}

	for name, repo := range stores {
		t.Run(name, func(t *testing.T) {
			batchID := fmt.Sprintf("batch-statuscount-%s-%d", name, time.Now().UnixNano())
			seedStatusCountFixture(t, repo, batchID)

			// Correct per-status counts, no search filter.
			counts, err := repo.CountResultsByStatus(batchID, "")
			require.NoError(t, err)
			assert.Equal(t, 2, counts["AUTO_MATCHED"])
			assert.Equal(t, 1, counts["REVIEW_NEEDED"])
			assert.Equal(t, 1, counts["REJECTED"])

			// Honours the search filter: "Countable" only matches src-1 (2 rows, both
			// AUTO_MATCHED) and src-2 (1 row, REVIEW_NEEDED), never src-3 ("Unrelated").
			counts, err = repo.CountResultsByStatus(batchID, "Countable")
			require.NoError(t, err)
			assert.Equal(t, 2, counts["AUTO_MATCHED"])
			assert.Equal(t, 1, counts["REVIEW_NEEDED"])
			assert.Equal(t, 0, counts["REJECTED"])

			// IGNORES any status filter: CountResultsByStatus has no status parameter at
			// all, so a search matching rows of multiple statuses must report ALL of
			// them together, never narrowed down to just one.
			counts, err = repo.CountResultsByStatus(batchID, "")
			require.NoError(t, err)
			assert.Len(t, counts, 3, "expected all three distinct statuses present, unfiltered by status")

			// Non-nil empty map (and nil error) for a batch that does not exist.
			counts, err = repo.CountResultsByStatus("batch-does-not-exist-xyz", "")
			require.NoError(t, err)
			require.NotNil(t, counts)
			assert.Empty(t, counts)
		})
	}
}

// ---------------------------------------------------------------------------
// 8. An unknown batch id is an empty page, not an error, on BOTH stores.
// ---------------------------------------------------------------------------

func TestGetResultsPageUnknownBatchIsEmptyNotError(t *testing.T) {
	stores := map[string]Repository{
		"postgres": testPostgresStore(t),
		"memory":   NewStore(),
	}

	for name, repo := range stores {
		t.Run(name, func(t *testing.T) {
			page, total, err := repo.GetResultsPage(ResultsQuery{BatchID: "no-such-batch-ever-" + name, Limit: 100})
			require.NoError(t, err)
			assert.Equal(t, 0, total)
			assert.Empty(t, page)
		})
	}
}

// ---------------------------------------------------------------------------
// 9. LIKE metacharacters in a search term must be escaped, matching semantics.
// ---------------------------------------------------------------------------

// TestPostgresLikeMetacharactersAreEscaped is the regression test for the LIKE-metacharacter-escaping defect --
// an unescaped search term let `_` (SQL LIKE "any single character") and `%` (SQL LIKE "any sequence of characters")
// turn a literal-substring search into a wildcard pattern match in Postgres, silently diverging from the
// in-memory store's literal `strings.Contains` semantics.
func TestPostgresLikeMetacharactersAreEscaped(t *testing.T) {
	batchID := fmt.Sprintf("batch-likeescape-%d", time.Now().UnixNano())

	postgresStore := testPostgresStore(t)
	memStore := NewStore()

	sourceA := matcher.SourceRecord{
		ID:              "source-a",
		BatchID:         batchID,
		ReferenceID:     "ref-a",
		CustomerNameRaw: "Under_score Name",
	}
	sourceB := matcher.SourceRecord{
		ID:              "source-b",
		BatchID:         batchID,
		ReferenceID:     "ref-b",
		CustomerNameRaw: "Percent%Sign",
	}
	sourceC := matcher.SourceRecord{
		ID:              "source-c",
		BatchID:         batchID,
		ReferenceID:     "ref-c",
		CustomerNameRaw: "Plain Name Nothing Special",
	}

	resultA := matcher.MatchResultItem{
		ID:            "result-a",
		BatchID:       batchID,
		SourceID:      "source-a",
		DestinationID: "",
		MatchStatus:   "AUTO_MATCHED",
		CreatedAt:     time.Now(),
	}
	resultB := matcher.MatchResultItem{
		ID:            "result-b",
		BatchID:       batchID,
		SourceID:      "source-b",
		DestinationID: "",
		MatchStatus:   "AUTO_MATCHED",
		CreatedAt:     time.Now(),
	}
	resultC := matcher.MatchResultItem{
		ID:            "result-c",
		BatchID:       batchID,
		SourceID:      "source-c",
		DestinationID: "",
		MatchStatus:   "AUTO_MATCHED",
		CreatedAt:     time.Now(),
	}

	pgSources := []matcher.SourceRecord{sourceA, sourceB, sourceC}
	pgResults := []matcher.MatchResultItem{resultA, resultB, resultC}

	memSources := []matcher.SourceRecord{sourceA, sourceB, sourceC}
	memResults := []matcher.MatchResultItem{resultA, resultB, resultC}

	err := postgresStore.SaveDataset(batchID, pgSources, nil)
	require.NoError(t, err)
	err = postgresStore.SaveResultsCtx(context.Background(), batchID, pgResults)
	require.NoError(t, err)

	err = memStore.SaveDataset(batchID, memSources, nil)
	require.NoError(t, err)
	err = memStore.SaveResultsCtx(context.Background(), batchID, memResults)
	require.NoError(t, err)

	// Test underscore search
	pgResultsA, _, err := postgresStore.GetResultsPage(ResultsQuery{BatchID: batchID, Search: "_", Limit: 100})
	require.NoError(t, err)
	memResultsA, _, err := memStore.GetResultsPage(ResultsQuery{BatchID: batchID, Search: "_", Limit: 100})
	require.NoError(t, err)
	pgIDsA := idsOf(pgResultsA)
	memIDsA := idsOf(memResultsA)
	assert.ElementsMatch(t, pgIDsA, memIDsA, "underscore search mismatch between stores")
	assert.Equal(t, []string{"result-a"}, pgIDsA, "underscore search should match only row A")

	// Test percent sign search
	pgResultsB, _, err := postgresStore.GetResultsPage(ResultsQuery{BatchID: batchID, Search: "%", Limit: 100})
	require.NoError(t, err)
	memResultsB, _, err := memStore.GetResultsPage(ResultsQuery{BatchID: batchID, Search: "%", Limit: 100})
	require.NoError(t, err)
	pgIDsB := idsOf(pgResultsB)
	memIDsB := idsOf(memResultsB)
	assert.ElementsMatch(t, pgIDsB, memIDsB, "percent search mismatch between stores")
	assert.Equal(t, []string{"result-b"}, pgIDsB, "percent search should match only row B")

	// Test plain name search
	pgResultsC, _, err := postgresStore.GetResultsPage(ResultsQuery{BatchID: batchID, Search: "Plain Name", Limit: 100})
	require.NoError(t, err)
	memResultsC, _, err := memStore.GetResultsPage(ResultsQuery{BatchID: batchID, Search: "Plain Name", Limit: 100})
	require.NoError(t, err)
	pgIDsC := idsOf(pgResultsC)
	memIDsC := idsOf(memResultsC)
	assert.ElementsMatch(t, pgIDsC, memIDsC, "plain name search mismatch between stores")
	assert.Equal(t, []string{"result-c"}, pgIDsC, "plain name search should match only row C")
}
