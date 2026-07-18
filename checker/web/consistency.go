package web

import (
	"fmt"
	"strings"
	"unicode"

	"http-tester/checker"
)

// wordFreq builds a frequency map of words from input bytes
func wordFreq(data []byte) map[string]int {
	freq := make(map[string]int)
	words := strings.FieldsFunc(string(data), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, w := range words {
		w = strings.ToLower(w)
		if len(w) >= 3 {
			freq[w]++
		}
	}
	return freq
}

// compareWordMaps compares two word frequency maps and returns the ratio of matching words
func compareWordMaps(a, b map[string]int) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	// Union of all unique words
	union := make(map[string]bool)
	for w := range a {
		union[w] = true
	}
	for w := range b {
		union[w] = true
	}
	// Count words present in both maps
	match := 0
	for w := range union {
		if a[w] > 0 && b[w] > 0 {
			match++
		}
	}
	return float64(match) / float64(len(union))
}

// CheckConsistency compares response bodies from different protocols and warns on significant differences
func CheckConsistency(results []*checker.Result) []*checker.Result {
	// Collect results with 200 OK status and body
	type entry struct {
		checker string
		body    []byte
	}
	var entries []entry
	for _, r := range results {
		if r.Passed && r.Body != nil && len(r.Body) > 1024 { // skip small responses (under 1KB)
			entries = append(entries, entry{checker: r.Checker, body: r.Body})
		}
	}

	if len(entries) < 2 {
		return nil
	}

	// Compare all pairs
	var warnings []*checker.Result
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			freqA := wordFreq(entries[i].body)
			freqB := wordFreq(entries[j].body)
			similarity := compareWordMaps(freqA, freqB)

			// Check size ratio
			sizeA := len(entries[i].body)
			sizeB := len(entries[j].body)
			sizeRatio := 1.0
			if sizeA > 0 && sizeB > 0 {
				if sizeA > sizeB {
					sizeRatio = float64(sizeA) / float64(sizeB)
				} else {
					sizeRatio = float64(sizeB) / float64(sizeA)
				}
			}

			// Warn if similarity is low or size differs significantly
			// Thresholds are intentionally relaxed to avoid false positives
			// from sites with A/B testing, localization, or minor content variations
			if similarity < 0.60 || sizeRatio > 3.0 {
				warnings = append(warnings, &checker.Result{
					Checker: "consistency",
					Group:   "Consistency",
					Domain:  entries[i].checker,
					Passed:  false,
					Warning: true,
					Details: fmt.Sprintf("different content detected between %s and %s (word overlap: %.0f%%, size ratio: %.1fx)",
						entries[i].checker, entries[j].checker, similarity*100, sizeRatio),
				})
			}
		}
	}

	return warnings
}
