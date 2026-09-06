package matcher

import (
	"testing"
)

func TestSetEntryStoresDescriptionAndListEntriesReturnsIt(t *testing.T) {
	d := NewCustomDictionary()
	d.SetEntry(SynonymEntry{
		Alias:       " MyAlias ",
		Canonical:   " MyCanonical ",
		Description: " My description ",
	})

	entries := d.ListEntries()
	var foundEntry *SynonymEntry
	for _, entry := range entries {
		if entry.Alias == "myalias" {
			foundEntry = &entry
			break
		}
	}
	if foundEntry == nil {
		t.Fatalf("Expected entry with alias 'myalias' not found in ListEntries()")
	}

	// Canonical is only trimmed, not lowercased, matching Set's existing behavior.
	if foundEntry.Canonical != "MyCanonical" {
		t.Errorf("Expected Canonical 'MyCanonical', got %q", foundEntry.Canonical)
	}
	if foundEntry.Description != "My description" {
		t.Errorf("Expected Description 'My description', got %q", foundEntry.Description)
	}
}

func TestDeleteRemovesBothCanonicalAndDescription(t *testing.T) {
	d := NewCustomDictionary()
	d.SetEntry(SynonymEntry{
		Alias:       "foo",
		Canonical:   "bar",
		Description: "some description",
	})

	if _, exists := d.Lookup("foo"); !exists {
		t.Fatalf("Expected entry 'foo' to exist before deletion")
	}

	d.Delete("foo")

	if _, exists := d.Lookup("foo"); exists {
		t.Errorf("Expected entry 'foo' to be deleted after calling Delete")
	}

	entries := d.ListEntries()
	for _, entry := range entries {
		if entry.Alias == "foo" {
			t.Errorf("Found entry with alias 'foo' in ListEntries() after deletion")
			break
		}
	}
}

func TestSetEntryWithEmptyDescriptionClearsStaleDescription(t *testing.T) {
	d := NewCustomDictionary()
	d.SetEntry(SynonymEntry{
		Alias:       "baz",
		Canonical:   "qux",
		Description: "stale description",
	})

	entries := d.ListEntries()
	var foundEntry *SynonymEntry
	for _, entry := range entries {
		if entry.Alias == "baz" {
			foundEntry = &entry
			break
		}
	}
	if foundEntry == nil || foundEntry.Description != "stale description" {
		t.Fatalf("Expected stale description 'stale description' for alias 'baz'")
	}

	d.SetEntry(SynonymEntry{
		Alias:       "baz",
		Canonical:   "qux",
		Description: "",
	})

	entries = d.ListEntries()
	for _, entry := range entries {
		if entry.Alias == "baz" {
			if entry.Description != "" {
				t.Errorf("Expected empty description after re-setting with empty description, got %q", entry.Description)
			}
			return
		}
	}
	t.Fatalf("Entry for 'baz' not found after re-setting")
}

func TestReplaceSynonymsInTextStillSubstitutesAfterSetEntry(t *testing.T) {
	testAlias := "zzqtestalias123"
	testCanonical := "zzqtestcanonical123"
	GetGlobalDictionary().SetEntry(SynonymEntry{
		Alias:       testAlias,
		Canonical:   testCanonical,
		Description: "regression guard alias",
	})
	t.Cleanup(func() {
		GetGlobalDictionary().Delete(testAlias)
	})

	result := ReplaceSynonymsInText("payment from ZZQTestAlias123 today")
	expected := "payment from zzqtestcanonical123 today"
	if result != expected {
		t.Errorf("ReplaceSynonymsInText(%q) = %q; want %q", "payment from ZZQTestAlias123 today", result, expected)
	}
}
