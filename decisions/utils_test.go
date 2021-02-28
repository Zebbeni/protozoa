package decisions

import (
	"github.com/Zebbeni/protozoa/config"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNodeLibrary(t *testing.T) {
	rand.Seed(0)
	nodeLibrary := NewNodeLibrary()
	if len(nodeLibrary.Map) != 7 {
		t.Errorf("expected 7 initial nodes in library (one per action), got %d", len(nodeLibrary.Map))
	}
	expectedNodeIDs := []string{"00", "01", "02", "03", "04", "05", "06"}
	for _, expectedID := range expectedNodeIDs {
		if _, ok := nodeLibrary.Map[expectedID]; !ok {
			t.Errorf("Node not found with expected ID: %s", expectedID)
		}
	}
}

func TestMutateAndRegisterNode(t *testing.T) {
	globals := config.NewGlobals()
	config.SetGlobals(&globals)
	rand.Seed(2)
	// Test simple decision tree
	node := TreeFromAction(ActEat)
	expectedID := "02"
	expectedPrint := "Eat (0 uses)\n"
	assert.Equal(t, expectedID, node.ID, "Unexpected Node ID")
	assert.Equal(t, expectedPrint, node.PrintTree("", true, false))
	// Test effect of single mutation
	mutated := MutateTree(node)
	expectedID = "200602"
	expectedPrint = "If Organism Right (0 uses)\n├─Turn Right (0 uses)\n└─Eat (0 uses)\n"
	assert.Equal(t, expectedID, mutated.ID, "Unexpected Node ID after first Mutate")
	assert.Equal(t, expectedPrint, mutated.PrintTree("", true, false))
	// Register and verify in node library
	nodeLibrary := NewNodeLibrary()
	nodeLibrary.RegisterAndReturnNewNode(mutated)
	mutatedNode, ok := nodeLibrary.Map[expectedID]
	if !ok {
		t.Errorf("Mutated Node with with expected ID %s not found after registering", expectedID)
	} else {
		assert.Equal(t, expectedPrint, mutatedNode.PrintTree("", true, false))
	}
}
