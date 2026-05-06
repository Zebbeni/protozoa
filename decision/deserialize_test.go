package decision

import "testing"

func TestDeserializeRoundTrip(t *testing.T) {
	// Simple action-only tree
	tree1 := TreeFromAction(ActChemosynthesis)
	s1 := tree1.Serialize()
	rt1 := DeserializeTree(s1)
	if rt1 == nil {
		t.Fatal("DeserializeTree returned nil for simple action tree")
	}
	if rt1.Serialize() != s1 {
		t.Errorf("Round-trip failed: got %q, want %q", rt1.Serialize(), s1)
	}

	// Build a more complex tree manually
	tree2 := &Tree{
		Node: &Node{
			NodeType: CanMove,
			YesNode: &Node{
				NodeType: IsFoodAhead,
				YesNode:  NodeFromAction(ActEat),
				NoNode:   NodeFromAction(ActMove),
				size:     3,
			},
			NoNode: NodeFromAction(ActChemosynthesis),
			size:   5,
		},
	}
	tree2.ID = tree2.Serialize()
	s2 := tree2.Serialize()
	rt2 := DeserializeTree(s2)
	if rt2 == nil {
		t.Fatal("DeserializeTree returned nil for complex tree")
	}
	if rt2.Serialize() != s2 {
		t.Errorf("Round-trip failed: got %q, want %q", rt2.Serialize(), s2)
	}
	if rt2.size != 5 {
		t.Errorf("Size mismatch: got %d, want 5", rt2.size)
	}
}
