package decision

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Zebbeni/protozoa/config"
)

func TestMutateAndRegisterTree(t *testing.T) {
	globals := config.NewGlobals()
	config.SetGlobals(&globals)
	rand.Seed(2)
	// Test simple decision tree
	node := TreeFromAction(ActEat)
	expectedID := "02"
	expectedPrint := "Eat\n"
	assert.Equal(t, expectedID, node.ID, "Unexpected Tree ID")
	assert.Equal(t, expectedPrint, node.Print())
	// Test effect of single mutation
	mutated := MutateTree(node)
	expectedID = "200602"
	expectedPrint = "If Organism Right\n├─Turn Right\n└─Eat\n"
	assert.Equal(t, expectedID, mutated.ID, "Unexpected Tree ID after first mutate")
	assert.Equal(t, expectedPrint, mutated.Print())
	// Register and verify in tree library
	lib := NewLibrary()
	lib.RegisterAndReturnTree(mutated)
	libTree, ok := lib[expectedID]
	if !ok {
		t.Errorf("Mutated Tree with with expected ID %s not found after registering", expectedID)
	} else {
		assert.Equal(t, expectedPrint, libTree.Print())
	}
}
