package store

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type AuditLogEntry struct {
	ID              string    `json:"id"`
	BatchID         string    `json:"batch_id"`
	SourceID        string    `json:"source_id"`
	DestinationID   string    `json:"destination_id"`
	UserID          string    `json:"user_id"`         // Reviewer username or ID
	Action          string    `json:"action"`          // "CONFIRM", "REJECT", "OVERRIDE"
	PreviousStatus  string    `json:"previous_status"` // "REVIEW_NEEDED", "AUTO_MATCHED", etc.
	NewStatus       string    `json:"new_status"`      // "CONFIRMED", "REJECTED"
	ConfidenceScore float64   `json:"confidence_score"`
	ReviewComments  string    `json:"review_comments"` // Rationale for compliance audit
	Timestamp       time.Time `json:"timestamp"`
}

type AuditStore struct {
	mu   sync.RWMutex
	logs []AuditLogEntry
}

func NewAuditStore() *AuditStore {
	return &AuditStore{
		logs: make([]AuditLogEntry, 0),
	}
}

func (a *AuditStore) RecordAuditLog(entry AuditLogEntry) AuditLogEntry {
	a.mu.Lock()
	defer a.mu.Unlock()

	if entry.ID == "" {
		entry.ID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.UserID == "" {
		entry.UserID = "reviewer_op"
	}

	a.logs = append(a.logs, entry)
	return entry
}

func (a *AuditStore) GetAuditLogs(batchID, userID, actionFilter string) []AuditLogEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var filtered []AuditLogEntry
	for _, entry := range a.logs {
		if batchID != "" && entry.BatchID != batchID {
			continue
		}
		if userID != "" && entry.UserID != userID {
			continue
		}
		if actionFilter != "" && entry.Action != actionFilter {
			continue
		}
		filtered = append(filtered, entry)
	}

	// Sort descending by timestamp
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	return filtered
}
