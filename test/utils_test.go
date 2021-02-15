package test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	d "github.com/Zebbeni/protozoa/decisions"
)

func TestNewNodeLibrary(t *testing.T) {
	rand.Seed(0)
	nodeLibrary := d.NewNodeLibrary()
	if len(nodeLibrary.Map) != 6 {
		t.Errorf("expected 6 initial nodes in library (one per action), got %d", len(nodeLibrary.Map))
	}
	expectedNodeIDs := []string{"0", "1", "2", "3", "4", "5"}
	for _, expectedID := range expectedNodeIDs {
		if _, ok := nodeLibrary.Map[expectedID]; !ok {
			t.Errorf("Node not found with expected ID: %s", expectedID)
		}
	}
}

func TestMutateAndRegisterNode(t *testing.T) {
	rand.Seed(2)
	// Test simple decision tree
	node := d.TreeFromAction(d.ActEat)
	expectedID := "1"
	expectedPrint := "├─Eat (0.00 uses)\n"
	assert.Equal(t, expectedID, node.ID, "Unexpected Node ID")
	assert.Equal(t, expectedPrint, node.Print("", true, false))
	// Test effect of single mutation
	mutated := d.MutateTree(node)
	expectedID = "12-2-1"
	expectedPrint = "├─If Organism Right (0.00 uses)\n│ ├─Be Idle (0.00 uses)\n│ └─Eat (0.00 uses)\n"
	assert.Equal(t, expectedID, mutated.ID, "Unexpected Node ID after first Mutate")
	assert.Equal(t, expectedPrint, mutated.Print("", true, false))
	// Register and verify in node library
	nodeLibrary := d.NewNodeLibrary()
	nodeLibrary.RegisterAndReturnNewNode(mutated)
	mutatedNode, ok := nodeLibrary.Map[expectedID]
	if !ok {
		t.Errorf("Mutated Node with with expected ID %s not found after registering", expectedID)
	} else {
		assert.Equal(t, expectedPrint, mutatedNode.Print("", true, false))
	}
}
