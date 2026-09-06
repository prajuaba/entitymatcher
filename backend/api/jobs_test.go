package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"entitymatcher/matcher"
	"entitymatcher/store"
	"github.com/stretchr/testify/require"
)

func TestListJobsPaginationBounds(t *testing.T) {
	server := NewServer(store.NewStore())

	cases := []struct {
		query        string
		expectLimit  float64
		expectOffset float64
	}{
		{"", 20, 0},
		{"?limit=5", 5, 0},
		{"?limit=0", 20, 0},
		{"?limit=-3", 20, 0},
		{"?limit=99999", 200, 0},
		{"?offset=-1", 20, 0},
		{"?limit=abc", 20, 0},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/jobs"+tc.query, nil)
			w := httptest.NewRecorder()

			server.HandleListJobs(w, req)

			require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

			var resp map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

			limit, ok := resp["limit"].(float64)
			require.True(t, ok, "limit must be float64")
			require.Equal(t, tc.expectLimit, limit, "limit mismatch")

			offset, ok := resp["offset"].(float64)
			require.True(t, ok, "offset must be float64")
			require.Equal(t, tc.expectOffset, offset, "offset mismatch")
		})
	}
}

func TestListJobsRejectsNonGet(t *testing.T) {
	server := NewServer(store.NewStore())

	req := httptest.NewRequest("POST", "/api/jobs", nil)
	w := httptest.NewRecorder()

	server.HandleListJobs(w, req)

	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestListJobsReturnsEmptyArrayNotNull(t *testing.T) {
	server := NewServer(store.NewStore())

	req := httptest.NewRequest("GET", "/api/jobs", nil)
	w := httptest.NewRecorder()

	server.HandleListJobs(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	bodyStr := w.Body.String()
	require.Contains(t, bodyStr, `"jobs":[]`, "expected jobs array to be empty, not null")
	require.NotContains(t, bodyStr, `"jobs":null`, "jobs field should not be null to avoid JS .map() breakage")
}

func TestListJobsReturnsRunsMostRecentFirst(t *testing.T) {
	st := store.NewStore()
	server := NewServer(st)

	batchIDs := []string{"batch-a", "batch-b", "batch-c"}
	now := time.Now()

	// batch-c is most recent (most recent StartedAt)
	// batch-b is middle
	// batch-a is oldest (oldest StartedAt)
	st.UpdateProgress(matcher.BatchProgress{
		BatchID:     batchIDs[2], // batch-c
		Status:      "COMPLETED",
		StartedAt:   now.Add(-1 * time.Hour),
		CompletedAt: now,
	})
	st.UpdateProgress(matcher.BatchProgress{
		BatchID:     batchIDs[1], // batch-b
		Status:      "COMPLETED",
		StartedAt:   now.Add(-2 * time.Hour),
		CompletedAt: now,
	})
	st.UpdateProgress(matcher.BatchProgress{
		BatchID:     batchIDs[0], // batch-a
		Status:      "COMPLETED",
		StartedAt:   now.Add(-3 * time.Hour),
		CompletedAt: now,
	})

	// Test full list (no pagination)
	req := httptest.NewRequest("GET", "/api/jobs", nil)
	w := httptest.NewRecorder()
	server.HandleListJobs(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	jobsRaw, ok := resp["jobs"].([]interface{})
	require.True(t, ok, "jobs must be an array")
	require.Equal(t, 3, len(jobsRaw), "expected 3 jobs")

	jobIDs := make([]string, len(jobsRaw))
	for i, job := range jobsRaw {
		jobMap, ok := job.(map[string]interface{})
		require.True(t, ok, "each job must be an object")
		batchID, ok := jobMap["batch_id"].(string)
		require.True(t, ok, "batch_id must be string")
		jobIDs[i] = batchID
	}

	// Most recent first: batch-c, batch-b, batch-a
	require.Equal(t, []string{"batch-c", "batch-b", "batch-a"}, jobIDs,
		"jobs should be ordered by started_at descending")

	// Test pagination: first page
	req = httptest.NewRequest("GET", "/api/jobs?limit=2&offset=0", nil)
	w = httptest.NewRecorder()
	server.HandleListJobs(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	jobsRaw, ok = resp["jobs"].([]interface{})
	require.True(t, ok, "jobs must be an array")
	require.Equal(t, 2, len(jobsRaw), "first page should have 2 jobs")

	page1IDs := make(map[string]bool)
	for _, job := range jobsRaw {
		jobMap, _ := job.(map[string]interface{})
		batchID, _ := jobMap["batch_id"].(string)
		page1IDs[batchID] = true
	}

	// Test pagination: second page
	req = httptest.NewRequest("GET", "/api/jobs?limit=2&offset=2", nil)
	w = httptest.NewRecorder()
	server.HandleListJobs(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	jobsRaw, ok = resp["jobs"].([]interface{})
	require.True(t, ok, "jobs must be an array")
	require.Equal(t, 1, len(jobsRaw), "second page should have 1 job")

	page2IDs := make(map[string]bool)
	for _, job := range jobsRaw {
		jobMap, _ := job.(map[string]interface{})
		batchID, _ := jobMap["batch_id"].(string)
		page2IDs[batchID] = true
	}

	// Ensure pages are disjoint
	require.Equal(t, 0, len(intersectStringSets(page1IDs, page2IDs)),
		"pagination pages should be disjoint")
}

func TestListJobsPostgresParity(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pgStore, err := store.NewPostgresStore(ctx, dsn)
	require.NoError(t, err)
	defer pgStore.Close()

	server := NewServer(pgStore)

	batchID := fmt.Sprintf("jobs-parity-test-%d", time.Now().UnixNano())

	// Save dataset (this creates a match_jobs row in PostgreSQL per postgres_store.go)
	pgStore.SaveDataset(batchID,
		[]matcher.SourceRecord{
			{ID: "s1", BatchID: batchID, ReferenceID: "REF1", CustomerNameRaw: "Test Co"},
		},
		[]matcher.DestinationRecord{
			{ID: "d1", BatchID: batchID, CustomerID: "CUST1", CustomerNameRaw: "Test Co"},
		},
	)

	// Clean up batch after test
	defer pgStore.DeleteBatch(batchID)

	req := httptest.NewRequest("GET", "/api/jobs?limit=20&offset=0", nil)
	w := httptest.NewRecorder()
	server.HandleListJobs(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	jobsRaw, ok := resp["jobs"].([]interface{})
	require.True(t, ok, "jobs must be an array")

	found := false
	for _, job := range jobsRaw {
		jobMap, ok := job.(map[string]interface{})
		require.True(t, ok, "each job must be an object")
		jobBatchID, ok := jobMap["batch_id"].(string)
		require.True(t, ok, "batch_id must be string")
		if jobBatchID == batchID {
			found = true
			break
		}
	}

	// The PostgreSQL store creates a match_jobs row inside SaveDataset (see postgres_store.go ~line 131)
	// with status 'IDLE', so an uploaded-but-never-matched batch IS listed by ListJobs here,
	// unlike the in-memory store which derives jobs only from progress and would NOT list it.
	// This test pins and documents that divergence — it does not assert the two stores agree.
	require.True(t, found, "expected the just-saved batch to appear in ListJobs, documenting the PostgreSQL SaveDataset-creates-a-job-row behaviour")
}

// Helper to compute string set intersection
func intersectStringSets(a, b map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for k := range a {
		if b[k] {
			result[k] = true
		}
	}
	return result
}
