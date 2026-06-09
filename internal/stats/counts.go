package stats

// GetPlatformAccountCounts returns stored per-platform account totals (excludes aggregate _Total).
func GetPlatformAccountCounts() (map[string]int, error) {
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return nil, err
	}
	out := make(map[string]int)
	for k, v := range state.SwitcherStats {
		if k == "_Total" {
			continue
		}
		if v.Accounts > 0 {
			out[k] = v.Accounts
		}
	}
	return out, nil
}
