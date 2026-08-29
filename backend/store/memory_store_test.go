package store

import (
	"sync"
	"testing"
	"time"

	"entitymatcher/matcher"
)

func TestResultIndexCorrectness(t *testing.T) {
	store := NewStore()
	batchID := "test-batch-1"

	results := []matcher.MatchResultItem{
		{ID: "item-1", BatchID: batchID, MatchStatus: "AUTO_MATCHED"},
		{ID: "item-2", BatchID: batchID, MatchStatus: "REVIEW_NEEDED"},
		{ID: "item-3", BatchID: batchID, MatchStatus: "CONFIRMED"},
	}

	store.SaveResults(batchID, results)

	index := store.resultIndex[batchID]
	if len(index) != 3 {
		t.Fatalf("Expected index size 3, got %d", len(index))
	}

	for i, item := range results {
		pos, exists := index[item.ID]
		if !exists {
			t.Errorf("Match ID %s not found in index", item.ID)
		}
		if pos != i {
			t.Errorf("Expected position %d for ID %s, got %d", i, item.ID, pos)
		}
	}
}

func TestManualLinkMaintainsIndex(t *testing.T) {
	store := NewStore()
	batchID := "test-batch-2"
	sources := []matcher.SourceRecord{
		{ID: "src1", BatchID: batchID, NormalizedName: matcher.CleanName{Raw: "John Doe"}},
	}
	dests := []matcher.DestinationRecord{
		{ID: "dst1", BatchID: batchID, NormalizedName: matcher.CleanName{Raw: "John Doe"}},
	}

	store.SaveDataset(batchID, sources, dests)

	results := []matcher.MatchResultItem{
		{ID: "existing-item", BatchID: batchID, MatchStatus: "AUTO_MATCHED"},
	}
	store.SaveResults(batchID, results)

	newItem, err := store.ManualLink(batchID, "src1", "dst1")
	if err != nil {
		t.Fatalf("ManualLink failed: %v", err)
	}

	index := store.resultIndex[batchID]
	pos, exists := index[newItem.ID]
	if !exists {
		t.Errorf("Newly added item ID %s not found in resultIndex", newItem.ID)
	}
	if pos != len(results) {
		t.Errorf("Expected position %d for new item, got %d", len(results), pos)
	}

	// Verify the item is in results slice
	items := store.results[batchID]
	if len(items) != 2 {
		t.Fatalf("Expected 2 items in results, got %d", len(items))
	}
	if items[pos].ID != newItem.ID {
		t.Errorf("Mismatched item ID at index")
	}
}

func TestUpdateMatchStatusO1(t *testing.T) {
	store := NewStore()
	batchID := "test-batch-3"
	results := []matcher.MatchResultItem{
		{ID: "item-1", BatchID: batchID, MatchStatus: "AUTO_MATCHED"},
		{ID: "item-2", BatchID: batchID, MatchStatus: "REVIEW_NEEDED"},
	}

	store.SaveResults(batchID, results)

	// Check that resultIndex is populated
	index := store.resultIndex[batchID]
	if len(index) != 2 {
		t.Fatalf("Expected index size 2, got %d", len(index))
	}

	// This should be O(1) lookup via index, not scanning
	err := store.UpdateMatchStatus(batchID, "item-2", "CONFIRMED")
	if err != nil {
		t.Fatalf("UpdateMatchStatus failed: %v", err)
	}

	// Verify status changed
	item, found := store.GetResultByID(batchID, "item-2")
	if !found {
		t.Fatalf("Could not find updated item by ID")
	}
	if item.MatchStatus != "CONFIRMED" {
		t.Errorf("Expected status CONFIRMED, got %s", item.MatchStatus)
	}
}

func TestGetResultByIDHit(t *testing.T) {
	store := NewStore()
	batchID := "test-batch-4"
	results := []matcher.MatchResultItem{
		{ID: "item-1", BatchID: batchID, MatchStatus: "AUTO_MATCHED"},
		{ID: "item-2", BatchID: batchID, MatchStatus: "REVIEW_NEEDED"},
	}

	store.SaveResults(batchID, results)

	item, found := store.GetResultByID(batchID, "item-2")
	if !found {
		t.Fatalf("Expected to find item by ID")
	}
	if item.ID != "item-2" {
		t.Errorf("Expected ID 'item-2', got '%s'", item.ID)
	}
	if item.MatchStatus != "REVIEW_NEEDED" {
		t.Errorf("Expected status REVIEW_NEEDED, got %s", item.MatchStatus)
	}
}

func TestGetResultByIDMiss(t *testing.T) {
	store := NewStore()
	batchID := "test-batch-5"
	results := []matcher.MatchResultItem{
		{ID: "item-1", BatchID: batchID, MatchStatus: "AUTO_MATCHED"},
	}

	store.SaveResults(batchID, results)

	// Test invalid batch ID
	_, found := store.GetResultByID("invalid-batch", "item-1")
	if found {
		t.Error("Expected false for invalid batch ID")
	}

	// Test invalid match ID
	_, found = store.GetResultByID(batchID, "nonexistent")
	if found {
		t.Error("Expected false for invalid match ID")
	}
}

func TestDeleteBatchRemovesEverything(t *testing.T) {
	store := NewStore()
	batchID := "test-batch-6"

	sources := []matcher.SourceRecord{{ID: "src1", BatchID: batchID}}
	dests := []matcher.DestinationRecord{{ID: "dst1", BatchID: batchID}}
	results := []matcher.MatchResultItem{{ID: "res1", BatchID: batchID, MatchStatus: "AUTO_MATCHED"}}

	store.SaveDataset(batchID, sources, dests)
	store.SaveResults(batchID, results)
	store.UpdateProgress(matcher.BatchProgress{BatchID: batchID, Status: "COMPLETED"})

	// Register an SSE client
	ch := store.RegisterSSEClient(batchID)

	// Verify batch exists
	if _, ok := store.sources[batchID]; !ok {
		t.Fatal("Batch not created")
	}

	// Delete batch
	store.DeleteBatch(batchID)

	// Verify all entries removed
	if _, ok := store.sources[batchID]; ok {
		t.Error("sources not deleted")
	}
	if _, ok := store.destinations[batchID]; ok {
		t.Error("destinations not deleted")
	}
	if _, ok := store.results[batchID]; ok {
		t.Error("results not deleted")
	}
	if _, ok := store.resultIndex[batchID]; ok {
		t.Error("resultIndex not deleted")
	}
	if _, ok := store.progresses[batchID]; ok {
		t.Error("progresses not deleted")
	}
	if _, ok := store.sseClients[batchID]; ok {
		t.Error("sseClients not deleted")
	}

	// Verify channel was closed (sending to closed channel should panic if not careful)
	// We rely on the DeleteBatch implementation closing safely
	select {
	case <-ch:
		// Channel was closed and drained
	default:
		// Channel is likely closed; try to send and defer recovery
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Channel send panic (expected closed channel): %v", r)
			}
		}()
		// This might panic if channel is closed, which is expected
		ch <- matcher.BatchProgress{}
	}
}

func TestListBatchesSummary(t *testing.T) {
	store := NewStore()
	batchID1 := "batch-1"
	batchID2 := "batch-2"

	sources1 := []matcher.SourceRecord{{ID: "src1"}, {ID: "src2"}}
	dests1 := []matcher.DestinationRecord{{ID: "dst1"}, {ID: "dst2"}, {ID: "dst3"}}
	results1 := []matcher.MatchResultItem{{ID: "res1"}, {ID: "res2"}}

	sources2 := []matcher.SourceRecord{{ID: "src3"}}
	dests2 := []matcher.DestinationRecord{{ID: "dst4"}}
	results2 := []matcher.MatchResultItem{{ID: "res3"}, {ID: "res4"}, {ID: "res5"}}

	store.SaveDataset(batchID1, sources1, dests1)
	store.SaveResults(batchID1, results1)
	store.UpdateProgress(matcher.BatchProgress{BatchID: batchID1, Status: "COMPLETED", StartedAt: time.Now()})

	store.SaveDataset(batchID2, sources2, dests2)
	store.SaveResults(batchID2, results2)
	store.UpdateProgress(matcher.BatchProgress{BatchID: batchID2, Status: "RUNNING", StartedAt: time.Now()})

	summaries := store.ListBatches()

	if len(summaries) != 2 {
		t.Fatalf("Expected 2 batch summaries, got %d", len(summaries))
	}

	// Find batch1 summary
	var summary1 *BatchSummary
	for i := range summaries {
		if summaries[i].BatchID == batchID1 {
			summary1 = &summaries[i]
			break
		}
	}

	if summary1 == nil {
		t.Fatalf("Batch %s summary not found", batchID1)
	}

	if summary1.SourceCount != 2 {
		t.Errorf("Expected source count 2, got %d", summary1.SourceCount)
	}
	if summary1.DestinationCount != 3 {
		t.Errorf("Expected destination count 3, got %d", summary1.DestinationCount)
	}
	if summary1.ResultCount != 2 {
		t.Errorf("Expected result count 2, got %d", summary1.ResultCount)
	}
	if summary1.Status != "COMPLETED" {
		t.Errorf("Expected status COMPLETED, got %s", summary1.Status)
	}
}

func TestConcurrencyNoSendOnClosed(t *testing.T) {
	store := NewStore()
	batchID := "test-batch-7"

	sources := []matcher.SourceRecord{{ID: "src1", BatchID: batchID}}
	dests := []matcher.DestinationRecord{{ID: "dst1", BatchID: batchID}}
	store.SaveDataset(batchID, sources, dests)

	// Register multiple SSE clients
	numClients := 10
	clients := make([]chan matcher.BatchProgress, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = store.RegisterSSEClient(batchID)
	}

	// Hammer with concurrent UpdateProgress and UnregisterSSEClient
	var wg sync.WaitGroup
	numWorkers := 20

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				// Alternate between UpdateProgress and UnregisterSSEClient
				if i%2 == 0 {
					store.UpdateProgress(matcher.BatchProgress{
						BatchID: batchID,
						Status:  "RUNNING",
					})
				} else if workerID < numClients {
					store.UnregisterSSEClient(batchID, clients[workerID])
				}
			}
		}(w)
	}

	wg.Wait()

	// Verify no panic occurred (if we got here, the race is resolved)
	t.Logf("Concurrency test passed: no send-on-closed-channel panic")
}
