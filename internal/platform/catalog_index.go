package platform

import (
	"bytes"
	"encoding/json"
	"sync"
)

// catalogIndex holds one catalog's per-platform JSON, split out once.
//
// Pulling a single platform out of Platforms.json meant unmarshalling all of it
// into map[string]json.RawMessage first - roughly 214us of pure JSON scanning
// for the 36KB catalog that ships - and every descriptor, entry and name lookup
// paid that again. Opening an account page did it twice over.
//
// The index is keyed on the catalog bytes rather than on a path or a
// generation counter, so a rewritten or swapped-out Platforms.json rebuilds it
// on the next lookup without anything having to remember to invalidate.
var catalogIndex struct {
	mu      sync.RWMutex
	raw     []byte
	entries map[string]json.RawMessage
	names   []string
}

// indexCatalog returns the per-platform JSON and the sorted platform names for
// raw, splitting the catalog only when it is not the one already indexed.
//
// The returned map must be treated as read-only: it is the cached one, and the
// json.RawMessage values in it hold copies of the catalog bytes, so callers
// unmarshal from them into their own values and nothing shared escapes.
func indexCatalog(raw []byte) (map[string]json.RawMessage, []string, error) {
	catalogIndex.mu.RLock()
	if catalogIndex.entries != nil && bytes.Equal(catalogIndex.raw, raw) {
		entries, names := catalogIndex.entries, catalogIndex.names
		catalogIndex.mu.RUnlock()
		return entries, names, nil
	}
	catalogIndex.mu.RUnlock()

	var top struct {
		Platforms map[string]json.RawMessage `json:"Platforms"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, nil, err
	}
	if top.Platforms == nil {
		return nil, nil, nil
	}

	names := make([]string, 0, len(top.Platforms))
	for k := range top.Platforms {
		names = append(names, k)
	}
	sortStringsFold(names)

	catalogIndex.mu.Lock()
	catalogIndex.raw = bytes.Clone(raw)
	catalogIndex.entries = top.Platforms
	catalogIndex.names = names
	catalogIndex.mu.Unlock()

	return top.Platforms, names, nil
}
