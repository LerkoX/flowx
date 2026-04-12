package flowx

import (
	"strings"
	"testing"
)

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid UUID with hyphens",
			input: "550e8400-e29b-41d4-a716-446655440000",
			want:  true,
		},
		{
			name:  "valid UUID without hyphens",
			input: "550e8400e29b41d4a716446655440000",
			want:  true,
		},
		{
			name:  "valid UUID all zeros",
			input: "00000000-0000-0000-0000-000000000000",
			want:  true,
		},
		{
			name:  "invalid random string",
			input: "not-a-uuid",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "too short",
			input: "550e8400",
			want:  false,
		},
		{
			name:  "32 chars but invalid hex",
			input: "550e8400g29b41d4a716446655440000",
			want:  false,
		},
		{
			name:  "uppercase without hyphens",
			input: "550E8400E29B41D4A716446655440000",
			want:  true,
		},
		{
			name:  "36 chars but invalid groups",
			input: "550e8400-e29b-41d4-a716-44665544",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateUUID(tt.input)
			if got != tt.want {
				t.Errorf("ValidateUUID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "32 chars adds hyphens",
			input: "550e8400e29b41d4a716446655440000",
			want:  "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:  "already formatted unchanged",
			input: "550e8400-e29b-41d4-a716-446655440000",
			want:  "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:  "short string unchanged",
			input: "abc",
			want:  "abc",
		},
		{
			name:  "empty string unchanged",
			input: "",
			want:  "",
		},
		{
			name:  "36 chars unchanged",
			input: "550e8400-e29b-41d4-a716-446655440000",
			want:  "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatUUID(tt.input)
			if got != tt.want {
				t.Errorf("FormatUUID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewUUID(t *testing.T) {
	id := NewUUID()

	if len(id) != 32 {
		t.Errorf("NewUUID() length = %d, want 32", len(id))
	}

	if strings.Contains(id, "-") {
		t.Error("NewUUID() should not contain hyphens")
	}

	if !ValidateUUID(id) {
		t.Errorf("NewUUID() = %q, not a valid UUID", id)
	}

	// Verify uniqueness
	id2 := NewUUID()
	if id == id2 {
		t.Error("Two consecutive NewUUID() calls returned the same value")
	}
}
