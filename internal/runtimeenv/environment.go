package runtimeenv

import "strings"

// WithoutHerdr removes context owned by the separate herdr product at a
// process boundary. It leaves unrelated environment entries unchanged.
func WithoutHerdr(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, "HERDR_") {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
