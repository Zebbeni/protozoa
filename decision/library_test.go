package decision

import (
	"math/rand"
	"testing"
)

func TestNewLibrary(t *testing.T) {
	rand.Seed(0)
	lib := NewLibrary()
	if len(lib) != 7 {
		t.Errorf("expected 7 initial nodes in library (one per action), got %d", len(lib))
	}
	expectedNodeIDs := []string{"00", "01", "02", "03", "04", "05", "06"}
	for _, expectedID := range expectedNodeIDs {
		if _, ok := lib[expectedID]; !ok {
			t.Errorf("Tree not found with expected ID: %s", expectedID)
		}
	}
}
