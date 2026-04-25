package core

import (
	"testing"
)

func TestNewUUID(t *testing.T) {
	id := NewUUID()

	if len(id) != 32 {
		t.Errorf("NewUUID() length = %d, want 32", len(id))
	}

	// Verify uniqueness
	id2 := NewUUID()
	if id == id2 {
		t.Error("Two consecutive NewUUID() calls returned the same value")
	}
}
