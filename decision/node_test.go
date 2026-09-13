package decision

import (
	"fmt"
	"testing"
)

func TestSerializeRoundTrip(t *testing.T) {
	// Serialize uses %02d per code, so the expected string is derived
	// from the live constant values rather than pinned to specific
	// ints — the iota layout in constants.go is allowed to change,
	// what matters is that Serialize/Deserialize round-trips.
	conditionalTree := &Tree{Node: &Node{
		NodeType: CanMove,
		YesNode:  NodeFromAction(ActAttack),
		NoNode:   NodeFromAction(ActEat),
	}}
	conditionalExpected := fmt.Sprintf("%02d%02d%02d", CanMove, ActAttack, ActEat)

	testCases := []struct {
		name     string
		tree     *Tree
		expected string
	}{
		{"single action", TreeFromAction(ActAttack), fmt.Sprintf("%02d", ActAttack)},
		{"conditional with two action children", conditionalTree, conditionalExpected},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tree.Serialize(); got != tc.expected {
				t.Errorf("Serialize() = %q, want %q", got, tc.expected)
			}
			// Verify the serialized form round-trips back to an
			// equivalent tree through Deserialize.
			node, consumed := Deserialize(tc.tree.Serialize())
			if node == nil {
				t.Fatalf("Deserialize returned nil for %q", tc.tree.Serialize())
			}
			if consumed != len(tc.tree.Serialize()) {
				t.Errorf("Deserialize consumed %d bytes, want %d", consumed, len(tc.tree.Serialize()))
			}
			redo := (&Tree{Node: node}).Serialize()
			if redo != tc.expected {
				t.Errorf("round-trip Serialize() = %q, want %q", redo, tc.expected)
			}
		})
	}
}
