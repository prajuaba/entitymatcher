package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"entitymatcher/matcher"
	"entitymatcher/store"

	"github.com/stretchr/testify/require"
)

func TestManualLinkWritesOverrideAuditEntry(t *testing.T) {
	st := store.NewStore()
	batchID := "batch-audit-1"

	sources := []matcher.SourceRecord{
		{ID: "src1", BatchID: batchID, NormalizedName: matcher.CleanName{Raw: "John Doe"}},
	}
	dests := []matcher.DestinationRecord{
		{ID: "dst1", BatchID: batchID, NormalizedName: matcher.CleanName{Raw: "John Doe"}},
	}
	require.NoError(t, st.SaveDataset(batchID, sources, dests))

	srv := NewServer(st)

	payload := ManualLinkPayload{
		BatchID:       batchID,
		SourceID:      "src1",
		DestinationID: "dst1",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	claims := &JWTClaims{
		UserID:   "usr-99",
		Username: "reviewer_jane",
		Name:     "Jane Reviewer",
		Role:     RoleAdmin,
	}
	ctx := context.WithValue(context.Background(), claimsKey{}, claims)
	req := httptest.NewRequest("POST", "/api/manual-link", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.HandleManualLink(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	logs := st.GetAuditLogs(batchID, "", "")
	require.Len(t, logs, 1)

	entry := logs[0]
	require.Equal(t, "OVERRIDE", entry.Action)
	require.Equal(t, "src1", entry.SourceID)
	require.Equal(t, "dst1", entry.DestinationID)
	require.Equal(t, "CONFIRMED", entry.NewStatus)
	require.Equal(t, 1.0, entry.ConfidenceScore)
	require.Equal(t, "usr-99", entry.UserID)
	require.Equal(t, "Manually linked by reviewer", entry.ReviewComments)
}

func TestManualLinkStoresSuppliedReviewComments(t *testing.T) {
	st := store.NewStore()
	batchID := "batch-audit-2"

	sources := []matcher.SourceRecord{
		{ID: "src2", BatchID: batchID, NormalizedName: matcher.CleanName{Raw: "John Doe"}},
	}
	dests := []matcher.DestinationRecord{
		{ID: "dst2", BatchID: batchID, NormalizedName: matcher.CleanName{Raw: "John Doe"}},
	}
	require.NoError(t, st.SaveDataset(batchID, sources, dests))

	srv := NewServer(st)

	payload := ManualLinkPayload{
		BatchID:        batchID,
		SourceID:       "src2",
		DestinationID:  "dst2",
		ReviewComments: "Confirmed via phone call with customer",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	claims := &JWTClaims{
		UserID:   "usr-99",
		Username: "reviewer_jane",
		Name:     "Jane Reviewer",
		Role:     RoleAdmin,
	}
	ctx := context.WithValue(context.Background(), claimsKey{}, claims)
	req := httptest.NewRequest("POST", "/api/manual-link", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.HandleManualLink(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	logs := st.GetAuditLogs(batchID, "", "")
	require.Len(t, logs, 1)

	entry := logs[0]
	require.Equal(t, "Confirmed via phone call with customer", entry.ReviewComments)
}

func TestManualLinkFailureWritesNoAuditEntry(t *testing.T) {
	st := store.NewStore()
	batchID := "batch-audit-3"

	srv := NewServer(st)

	payload := ManualLinkPayload{
		BatchID:       batchID,
		SourceID:      "src-nonexistent",
		DestinationID: "dst-nonexistent",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	claims := &JWTClaims{
		UserID:   "usr-99",
		Username: "reviewer_jane",
		Name:     "Jane Reviewer",
		Role:     RoleAdmin,
	}
	ctx := context.WithValue(context.Background(), claimsKey{}, claims)
	req := httptest.NewRequest("POST", "/api/manual-link", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.HandleManualLink(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	logs := st.GetAuditLogs(batchID, "", "")
	require.Len(t, logs, 0)
}
