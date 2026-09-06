package matcher

import (
	"strings"
	"sync"
)

type SynonymEntry struct {
	Alias       string `json:"alias"`       // e.g. "KBank" or "กสิกร"
	Canonical   string `json:"canonical"`   // e.g. "Kasikornbank"
	Description string `json:"description"` // e.g. "Bank brand alias"
}

type CustomDictionary struct {
	mu      sync.RWMutex
	entries map[string]string // lowercase alias -> canonical
	// descriptions is a parallel map, kept separate specifically so `entries` keeps its `map[string]string` shape,
	// because `ReplaceSynonymsInText` reads `dict.entries[lower]` directly on the hot normalization path and must not change type.
	descriptions map[string]string // lowercase alias -> description
}

var globalDictionary = NewCustomDictionary()

func NewCustomDictionary() *CustomDictionary {
	d := &CustomDictionary{
		entries:      make(map[string]string),
		descriptions: make(map[string]string),
	}
	// Default enterprise aliases
	d.entries["kbank"] = "kasikornbank"
	d.entries["กสิกรไทย"] = "kasikornbank"
	d.entries["กสิกร"] = "kasikornbank"
	d.entries["scb"] = "siam commercial bank"
	d.entries["ไทยพาณิชย์"] = "siam commercial bank"
	d.entries["bbl"] = "bangkok bank"
	d.entries["กรุงเทพ"] = "bangkok bank"
	d.entries["ais"] = "advanced info service"
	d.entries["เอไอเอส"] = "advanced info service"
	d.entries["cp"] = "charoen pokphand"
	d.entries["ซีพี"] = "charoen pokphand"
	return d
}

func GetGlobalDictionary() *CustomDictionary {
	return globalDictionary
}

func (d *CustomDictionary) Lookup(alias string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	canon, exists := d.entries[strings.ToLower(strings.TrimSpace(alias))]
	return canon, exists
}

func (d *CustomDictionary) Set(alias, canonical string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries[strings.ToLower(strings.TrimSpace(alias))] = strings.TrimSpace(canonical)
}

func (d *CustomDictionary) Delete(alias string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	normalizedAlias := strings.ToLower(strings.TrimSpace(alias))
	delete(d.entries, normalizedAlias)
	delete(d.descriptions, normalizedAlias)
}

func (d *CustomDictionary) ListEntries() []SynonymEntry {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var list []SynonymEntry
	for k, v := range d.entries {
		list = append(list, SynonymEntry{
			Alias:       k,
			Canonical:   v,
			Description: d.descriptions[k],
		})
	}
	return list
}

func (d *CustomDictionary) SetEntry(entry SynonymEntry) {
	d.mu.Lock()
	defer d.mu.Unlock()
	normalizedAlias := strings.ToLower(strings.TrimSpace(entry.Alias))
	d.entries[normalizedAlias] = strings.TrimSpace(entry.Canonical)
	if strings.TrimSpace(entry.Description) == "" {
		delete(d.descriptions, normalizedAlias)
	} else {
		d.descriptions[normalizedAlias] = strings.TrimSpace(entry.Description)
	}
}

func ReplaceSynonymsInText(text string) string {
	dict := GetGlobalDictionary()
	dict.mu.RLock()
	defer dict.mu.RUnlock()

	tokens := strings.Fields(text)
	for i, token := range tokens {
		lower := strings.ToLower(token)
		if canon, exists := dict.entries[lower]; exists {
			tokens[i] = canon
		}
	}
	return strings.Join(tokens, " ")
}
