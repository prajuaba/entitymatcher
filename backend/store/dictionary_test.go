package store

import (
	"testing"

	"entitymatcher/matcher"
)

// TestSaveDictionaryEntryRoundTrips verifies that an alias saved via the
// in-memory store's SaveDictionaryEntry is returned by ListDictionaryEntries,
// which is the round trip main.go's startup hydration depends on.
func TestSaveDictionaryEntryRoundTrips(t *testing.T) {
	s := NewStore()

	entry := matcher.SynonymEntry{
		Alias:       "kbank",
		Canonical:   "kasikornbank",
		Description: "Bank brand alias",
	}

	if err := s.SaveDictionaryEntry(entry); err != nil {
		t.Fatalf("SaveDictionaryEntry failed: %v", err)
	}

	entries, err := s.ListDictionaryEntries()
	if err != nil {
		t.Fatalf("ListDictionaryEntries failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0] != entry {
		t.Errorf("expected %+v, got %+v", entry, entries[0])
	}
}

// TestSaveDictionaryEntryOverwritesRatherThanDuplicates verifies that saving
// the same alias twice with a different canonical overwrites the existing row
// instead of appending a second one, matching the PostgreSQL upsert semantics
// (ON CONFLICT (alias) DO UPDATE).
func TestSaveDictionaryEntryOverwritesRatherThanDuplicates(t *testing.T) {
	s := NewStore()

	first := matcher.SynonymEntry{
		Alias:       "scb",
		Canonical:   "siam commercial bank",
		Description: "first save",
	}
	if err := s.SaveDictionaryEntry(first); err != nil {
		t.Fatalf("SaveDictionaryEntry (first) failed: %v", err)
	}

	second := matcher.SynonymEntry{
		Alias:       "scb",
		Canonical:   "siam commercial bank plc",
		Description: "second save",
	}
	if err := s.SaveDictionaryEntry(second); err != nil {
		t.Fatalf("SaveDictionaryEntry (second) failed: %v", err)
	}

	entries, err := s.ListDictionaryEntries()
	if err != nil {
		t.Fatalf("ListDictionaryEntries failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after overwrite, got %d: %+v", len(entries), entries)
	}
	if entries[0] != second {
		t.Errorf("expected overwritten entry %+v, got %+v", second, entries[0])
	}
}

// TestListDictionaryEntriesOrderedByAlias verifies entries come back sorted
// by alias, matching the PostgreSQL implementation's ORDER BY alias.
func TestListDictionaryEntriesOrderedByAlias(t *testing.T) {
	s := NewStore()

	for _, e := range []matcher.SynonymEntry{
		{Alias: "zzz", Canonical: "last"},
		{Alias: "aaa", Canonical: "first"},
		{Alias: "mmm", Canonical: "middle"},
	} {
		if err := s.SaveDictionaryEntry(e); err != nil {
			t.Fatalf("SaveDictionaryEntry(%q) failed: %v", e.Alias, err)
		}
	}

	entries, err := s.ListDictionaryEntries()
	if err != nil {
		t.Fatalf("ListDictionaryEntries failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	wantOrder := []string{"aaa", "mmm", "zzz"}
	for i, alias := range wantOrder {
		if entries[i].Alias != alias {
			t.Errorf("position %d: expected alias %q, got %q", i, alias, entries[i].Alias)
		}
	}
}

// TestDeleteDictionaryEntryTombstonesAndRevivesOnResave verifies that deleting
// a dictionary entry creates a tombstone which is cleared when the same alias
// is saved again, enabling a tombstone-then-revive round trip pattern.
func TestDeleteDictionaryEntryTombstonesAndRevivesOnResave(t *testing.T) {
	s := NewStore()

	entry := matcher.SynonymEntry{
		Alias:     "kbank",
		Canonical: "kasikornbank",
	}

	if err := s.SaveDictionaryEntry(entry); err != nil {
		t.Fatalf("SaveDictionaryEntry failed: %v", err)
	}

	if err := s.DeleteDictionaryEntry("kbank"); err != nil {
		t.Fatalf("DeleteDictionaryEntry failed: %v", err)
	}

	entries, err := s.ListDictionaryEntries()
	if err != nil {
		t.Fatalf("ListDictionaryEntries failed: %v", err)
	}
	for _, e := range entries {
		if e.Alias == "kbank" {
			t.Errorf("expected no entry with alias 'kbank' after deletion, got %+v", e)
		}
	}

	deleted, err := s.ListDeletedDictionaryAliases()
	if err != nil {
		t.Fatalf("ListDeletedDictionaryAliases failed: %v", err)
	}
	if len(deleted) != 1 {
		t.Fatalf("expected exactly 1 deleted alias, got %d: %+v", len(deleted), deleted)
	}
	if deleted[0] != "kbank" {
		t.Errorf("expected deleted alias 'kbank', got %q", deleted[0])
	}

	if err := s.SaveDictionaryEntry(entry); err != nil {
		t.Fatalf("SaveDictionaryEntry (resave) failed: %v", err)
	}

	entries, err = s.ListDictionaryEntries()
	if err != nil {
		t.Fatalf("ListDictionaryEntries (after resave) failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after resave, got %d: %+v", len(entries), entries)
	}
	if entries[0].Alias != "kbank" {
		t.Errorf("expected entry with alias 'kbank', got %+v", entries[0])
	}

	deleted, err = s.ListDeletedDictionaryAliases()
	if err != nil {
		t.Fatalf("ListDeletedDictionaryAliases (after resave) failed: %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("expected no deleted aliases after resave, got %d: %+v", len(deleted), deleted)
	}
}
