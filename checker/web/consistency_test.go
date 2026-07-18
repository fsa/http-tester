package web

import (
	"strings"
	"testing"

	"http-tester/checker"
)

func TestWordFreq(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected map[string]int
	}{
		{
			name:     "empty input",
			input:    []byte{},
			expected: map[string]int{},
		},
		{
			name:  "simple words",
			input: []byte("hello world hello"),
			expected: map[string]int{
				"hello": 2,
				"world": 1,
			},
		},
		{
			name:  "short words filtered",
			input: []byte("a an the cat dog"),
			expected: map[string]int{
				"the": 1,
				"cat": 1,
				"dog": 1,
			},
		},
		{
			name:  "case insensitive",
			input: []byte("Hello HELLO hello"),
			expected: map[string]int{
				"hello": 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wordFreq(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("wordFreq() returned %d words, expected %d", len(result), len(tt.expected))
				return
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("wordFreq()[%q] = %d, want %d", k, result[k], v)
				}
			}
		})
	}
}

func TestCompareWordMaps(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]int
		b        map[string]int
		expected float64
	}{
		{
			name:     "both empty",
			a:        map[string]int{},
			b:        map[string]int{},
			expected: 1.0,
		},
		{
			name:     "identical maps",
			a:        map[string]int{"hello": 1, "world": 2},
			b:        map[string]int{"hello": 1, "world": 2},
			expected: 1.0,
		},
		{
			name:     "completely different",
			a:        map[string]int{"hello": 1},
			b:        map[string]int{"world": 1},
			expected: 0.0,
		},
		{
			name:     "partial overlap",
			a:        map[string]int{"hello": 1, "world": 1},
			b:        map[string]int{"world": 1, "test": 1},
			expected: 1.0 / 3.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareWordMaps(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("compareWordMaps() = %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestCheckConsistency(t *testing.T) {
	tests := []struct {
		name            string
		results         []*checker.Result
		expectedWarning bool
	}{
		{
			name:            "no results",
			results:         []*checker.Result{},
			expectedWarning: false,
		},
		{
			name: "single result",
			results: []*checker.Result{
				{
					Checker: "http-ipv4",
					Passed:  true,
					Body:    []byte("test content here"),
				},
			},
			expectedWarning: false,
		},
		{
			name: "similar content",
			results: []*checker.Result{
				{
					Checker: "http-ipv4",
					Passed:  true,
					Body:    []byte("test content here with enough words to pass the size threshold"),
				},
				{
					Checker: "https-http2-ipv4",
					Passed:  true,
					Body:    []byte("test content here with enough words to pass the size threshold"),
				},
			},
			expectedWarning: false,
		},
		{
			name: "different content",
			results: []*checker.Result{
				{
					Checker: "http-ipv4",
					Passed:  true,
					Body:    []byte("completely different content with no overlap whatsoever at all here. The quick brown fox jumps over the lazy dog. Pack my box with five dozen liquor jugs. How vexingly quick daft zebras jump. The five boxing wizards jump quickly. Jackdaws love my big sphinx of quartz. " + strings.Repeat("XYZ", 500)),
				},
				{
					Checker: "https-http2-ipv4",
					Passed:  true,
					Body:    []byte("entirely other content with nothing in common together whatsoever. A mad boxer shot a quick gloved jab to the jaw of his dizzy opponent. Sixty zippers were quickly picked from the woven jute bag. The job requires extra pluck and zeal from every young wage earner. " + strings.Repeat("ABC", 500)),
				},
			},
			expectedWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := CheckConsistency(tt.results)
			hasWarning := len(warnings) > 0
			if hasWarning != tt.expectedWarning {
				t.Errorf("CheckConsistency() returned %d warnings, expected warning=%v", len(warnings), tt.expectedWarning)
			}
		})
	}
}
