package util

import (
	"testing"
)

func TestHasOverlap(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{
			name:     "both empty",
			a:        []string{},
			b:        []string{},
			expected: false,
		},
		{
			name:     "first empty",
			a:        []string{},
			b:        []string{"hello", "world"},
			expected: false,
		},
		{
			name:     "second empty",
			a:        []string{"hello", "world"},
			b:        []string{},
			expected: false,
		},
		{
			name:     "no overlap",
			a:        []string{"hello", "world"},
			b:        []string{"foo", "bar"},
			expected: false,
		},
		{
			name:     "partial overlap",
			a:        []string{"hello", "world"},
			b:        []string{"world", "foo"},
			expected: true,
		},
		{
			name:     "full overlap",
			a:        []string{"hello", "world"},
			b:        []string{"hello", "world"},
			expected: true,
		},
		{
			name:     "single element overlap",
			a:        []string{"hello"},
			b:        []string{"hello"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasOverlap(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("HasOverlap(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}
