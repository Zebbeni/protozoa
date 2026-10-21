package decision

import (
	"fmt"
	"testing"
)

func TestSerializeRoundTrip(t *testing.T) {
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
			// Verify the serialized form round-trips back to an equivalent tree through Deserialize.
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

// TestADamagedTreeFailsToDecodeRatherThanPanicking: a serialized tree can be
// truncated, or can come from a build whose node codes differ from this one —
// testdata/last.pzr holds both, and seeking it crashed the process on
// yes.size of a child that never parsed. A brain that will not decode has to
// report that, so the caller can drop the organism the way an unreadable
// neural net already is.
func TestADamagedTreeFailsToDecodeRatherThanPanicking(t *testing.T) {
	full := TreeFromAction(ActChemosynthesis).Serialize()
	root := NodeFromCondition(IsFoodAhead)
	root.YesNode = NodeFromAction(ActEat)
	root.NoNode = NodeFromAction(ActMove)
	branching := TreeFromNode(root).Serialize()

	for _, s := range []string{
		"",                           // nothing at all
		"9",                          // half a code
		branching[:len(branching)-2], // a condition missing its No branch
		branching[:2],                // a condition with neither branch
		branching + "99",             // a trailing code this build does not know
		"9999",                       // codes this build does not know at all
	} {
		// Deserialize may legitimately return a node and a short count — it
		// is a recursive descent and reports what it consumed. It is
		// DeserializeTree that has to reject the string.
		if tree := DeserializeTree(s); tree != nil {
			t.Errorf("DeserializeTree(%q) built a tree from a damaged string: %q",
				s, tree.Serialize())
		}
	}

	// And the undamaged ones still round-trip, so the guard did not simply
	// start refusing everything.
	for _, s := range []string{full, branching} {
		tree := DeserializeTree(s)
		if tree == nil {
			t.Fatalf("DeserializeTree(%q) refused a tree this build wrote", s)
		}
		if got := tree.Serialize(); got != s {
			t.Errorf("round trip of %q gave %q", s, got)
		}
	}
}
