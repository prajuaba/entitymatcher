package store

import (
	"testing"

	"entitymatcher/matcher"

	"github.com/stretchr/testify/require"
)

// seedRankedResults builds one batch holding a rank-1 row plus three rank>1
// alternates, which is the shape backlog S2 is about: on production data the
// rank>1 rows are 128,974 of the 171,402-row review queue.
func seedRankedResults(t *testing.T) *Store {
	t.Helper()
	s := NewStore()
	items := []matcher.MatchResultItem{
		{ID: "b-1", BatchID: "b", SourceID: "src-1", DestinationID: "dest-1", MatchStatus: "REVIEW_NEEDED", Rank: 1},
		{ID: "b-2", BatchID: "b", SourceID: "src-1", DestinationID: "dest-2", MatchStatus: "REVIEW_NEEDED", Rank: 2},
		{ID: "b-3", BatchID: "b", SourceID: "src-1", DestinationID: "dest-3", MatchStatus: "REVIEW_NEEDED", Rank: 3},
		{ID: "b-4", BatchID: "b", SourceID: "src-1", DestinationID: "dest-4", MatchStatus: "REVIEW_NEEDED", Rank: 4},
	}
	require.NoError(t, s.SaveResultsCtx(t.Context(), "b", items))
	return s
}

func TestRank1OnlyExcludesAlternatesAndCountAgrees(t *testing.T) {
	s := seedRankedResults(t)

	rows, total, err := s.GetResultsPage(ResultsQuery{BatchID: "b", Rank1Only: true, Limit: 50})
	require.NoError(t, err)

	// The count must reflect the SAME filter as the page. A count that still
	// included the alternates would report 4 while returning 1 row, which is
	// precisely how paging breaks -- the UI would render empty trailing pages.
	require.Equal(t, 1, total, "total count must agree with the filtered page")
	require.Len(t, rows, 1)
	require.Equal(t, 1, rows[0].Rank)
}

func TestRank1OnlyDefaultsOffAndReturnsAlternates(t *testing.T) {
	s := seedRankedResults(t)

	// No Rank1Only set: behaviour must be byte-identical to before the flag
	// existed, so every existing caller is unaffected.
	rows, total, err := s.GetResultsPage(ResultsQuery{BatchID: "b", Limit: 50})
	require.NoError(t, err)
	require.Equal(t, 4, total)
	require.Len(t, rows, 4)

	ranks := map[int]bool{}
	for _, r := range rows {
		ranks[r.Rank] = true
	}
	require.True(t, ranks[2] && ranks[3] && ranks[4], "alternates must still be returned by default")
}
