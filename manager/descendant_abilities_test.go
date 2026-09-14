package manager

import (
	"testing"

	"github.com/Zebbeni/protozoa/checkpoint"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
)

// TestDescendantNodeAbilitiesRoundTrip pins the save path the population
// graph's ability colouring depends on. Nodes outlive their organisms,
// so after a load the graph colours long-dead organisms purely from the
// node record — a restore that dropped the scores would silently paint
// every historical organism with the genesis colour.
func TestDescendantNodeAbilitiesRoundTrip(t *testing.T) {
	loadDefaultGlobals(t)

	parentScores := physiology.Scores{30, 5, 25, 10, 20, 10}
	childScores := physiology.Scores{10, 40, 5, 5, 20, 20}
	for _, s := range []physiology.Scores{parentScores, childScores} {
		if err := s.Validate(); err != nil {
			t.Fatalf("test scores invalid: %v", err)
		}
	}

	parent := &organism.DescendantNode{ID: 1, Abilities: parentScores}
	child := &organism.DescendantNode{ID: 2, Abilities: childScores}
	parent.AddChild(child)

	restored := recordToNode(nodeToRecord(parent), nil)
	if restored.Abilities != parentScores {
		t.Errorf("parent abilities %v after round trip, want %v", restored.Abilities, parentScores)
	}
	if len(restored.Children) != 1 {
		t.Fatalf("restored %d children, want 1", len(restored.Children))
	}
	if got := restored.Children[0].Abilities; got != childScores {
		t.Errorf("child abilities %v after round trip, want %v", got, childScores)
	}
}

// TestPreAbilitiesNodeRecordFallsBack: a node saved before ability scores
// existed has an all-zero record. It must come back as a valid genesis
// distribution, not as zeros that break the budget invariant.
func TestPreAbilitiesNodeRecordFallsBack(t *testing.T) {
	loadDefaultGlobals(t)

	node := recordToNode(checkpoint.DescendantNodeRecord{ID: 7}, nil)
	if err := node.Abilities.Validate(); err != nil {
		t.Errorf("stale node restored with invalid scores: %v", err)
	}
	if node.Abilities != physiology.GenesisScores() {
		t.Errorf("stale node abilities = %v, want genesis %v", node.Abilities, physiology.GenesisScores())
	}
}
