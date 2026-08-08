package basic

import (
	"encoding/json"
	"fmt"
)

// A game definition may declare Fallbacks: alternative stat sources tried in order when the
// primary one fails. Each entry is merged over the parent, so it only needs to restate what
// differs (usually Url, Attribution and the Collect paths).
//
// Merge rules, applied at any depth:
//   - objects merge key by key, so a fallback can change Collect.Premiere.Path while
//     inheriting that metric's DisplayAs, DisplayFormat and NoDisplayIf
//   - arrays and scalars replace outright
//   - an explicit null clears the parent's value
//
// Variant indices are 0 for the primary definition and 1..n for Fallbacks[0..n-1].

// deepMergeJSON returns override applied over base. Non-object values (and mismatched
// kinds) mean override wins wholesale.
func deepMergeJSON(base, override json.RawMessage) (json.RawMessage, error) {
	var baseObj, overrideObj map[string]json.RawMessage
	baseErr := json.Unmarshal(base, &baseObj)
	overrideErr := json.Unmarshal(override, &overrideObj)
	if baseErr != nil || overrideErr != nil || baseObj == nil || overrideObj == nil {
		return append(json.RawMessage(nil), override...), nil
	}

	out := make(map[string]json.RawMessage, len(baseObj)+len(overrideObj))
	for k, v := range baseObj {
		out[k] = v
	}
	for k, ov := range overrideObj {
		bv, ok := out[k]
		if !ok {
			out[k] = ov
			continue
		}
		merged, err := deepMergeJSON(bv, ov)
		if err != nil {
			return nil, err
		}
		out[k] = merged
	}
	return json.Marshal(out)
}

// jsonObjectWithoutKey drops top-level keys, so a parent's own Fallbacks list is not
// inherited by the variants derived from it.
func jsonObjectWithoutKey(raw json.RawMessage, keys ...string) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return append(json.RawMessage(nil), raw...), nil
	}
	for _, key := range keys {
		delete(obj, key)
	}
	return json.Marshal(obj)
}

// resolveFallbacks expands Fallbacks into fully merged definitions. Callers must apply
// normalizeGameDefinition to each result, since merging happens on the raw JSON.
func (d *gameDefinition) resolveFallbacks() error {
	d.resolved = nil
	if len(d.Fallbacks) == 0 {
		return nil
	}
	if len(d.raw) == 0 {
		return fmt.Errorf("game definition has Fallbacks but no source JSON")
	}
	// Fetch is dropped alongside Fallbacks: it names the fetcher for one specific
	// source, so inheriting it into a fallback with a different Url would point
	// every fallback at the same collector and quietly collapse the chain to one
	// source. A fallback that genuinely wants a collector states its own.
	base, err := jsonObjectWithoutKey(d.raw, "Fallbacks", "Fetch")
	if err != nil {
		return err
	}
	for i, fbRaw := range d.Fallbacks {
		merged, err := deepMergeJSON(base, fbRaw)
		if err != nil {
			return fmt.Errorf("merge fallback %d: %w", i, err)
		}
		var fb gameDefinition
		if err := json.Unmarshal(merged, &fb); err != nil {
			return fmt.Errorf("parse fallback %d: %w", i, err)
		}
		// Fallbacks are one level deep; a merged variant never carries its own chain.
		fb.Fallbacks = nil
		fb.resolved = nil
		d.resolved = append(d.resolved, fb)
	}
	return nil
}

// variantCount is 1 (the primary definition) plus any resolved fallbacks.
func (d gameDefinition) variantCount() int {
	return 1 + len(d.resolved)
}

// variantAt returns variant i, or the primary definition when i is out of range —
// which is what keeps a stale cached index safe after GameStats.json changes.
func (d gameDefinition) variantAt(i int) gameDefinition {
	if i <= 0 || i > len(d.resolved) {
		return d
	}
	return d.resolved[i-1]
}

// clampFallbackIndex maps a remembered index onto the current chain length.
func clampFallbackIndex(idx, count int) int {
	if idx <= 0 || count <= 0 || idx >= count {
		return 0
	}
	return idx
}

// fallbackAttemptOrder tries the remembered variant first, then every other variant from
// the start of the chain, so a source that stops working falls through to the rest.
func fallbackAttemptOrder(startIdx, count int) []int {
	if count <= 0 {
		return nil
	}
	startIdx = clampFallbackIndex(startIdx, count)
	order := make([]int, 0, count)
	order = append(order, startIdx)
	for i := 0; i < count; i++ {
		if i != startIdx {
			order = append(order, i)
		}
	}
	return order
}
