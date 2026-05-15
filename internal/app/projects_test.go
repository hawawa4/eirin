package app

import "testing"

func TestSanitizeFolderName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Project", "My_Project"},
		{"NGC 1234 Ha 300s", "NGC_1234_Ha_300s"},
		{"valid-name_here", "valid-name_here"},
		{"M42", "M42"},
		// Special characters are collapsed to a single underscore
		{"special!@#chars", "special_chars"},
		{"a  b", "a_b"}, // consecutive spaces → one underscore
		// Underscores at edges are trimmed
		{"!leading", "leading"},
		{"trailing!", "trailing"},
		// Empty / whitespace-only → fallback
		{"", "project"},
		{"   ", "project"},
		// Hyphens are preserved (they are in the allowed set)
		{"my-project-2024", "my-project-2024"},
		// Only invalid chars → fallback after trim
		{"!!!!", "project"},
	}

	for _, tt := range tests {
		got := sanitizeFolderName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFolderName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
