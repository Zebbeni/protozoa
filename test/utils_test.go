package test

import (
	"image/color"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	d "github.com/Zebbeni/protozoa/decisions"
	u "github.com/Zebbeni/protozoa/utils"
)

func TestCalculateDirectionVectors(t *testing.T) {
	angle := 0 * math.Pi // 0 degrees
	dirX := u.CalcDirXForDirection(angle)
	dirY := u.CalcDirYForDirection(angle)
	if dirX != 1 {
		t.Errorf("dirX for 0 degree angle should be 1. Got %d", dirX)
	}
	if dirY != 0 {
		t.Errorf("dirY for 0 degree angle should be 0. Got %d", dirY)
	}
	angle = 0.5 * math.Pi
	dirX = u.CalcDirXForDirection(angle)
	dirY = u.CalcDirYForDirection(angle)
	if dirX != 0 {
		t.Errorf("dirX for 90 degree angle should be 0. Got %d", dirX)
	}
	if dirY != 1 {
		t.Errorf("dirY for 90 degree angle should be 1. Got %d", dirY)
	}
	angle = 1.0 * math.Pi
	dirX = u.CalcDirXForDirection(angle)
	dirY = u.CalcDirYForDirection(angle)
	if dirX != -1 {
		t.Errorf("dirX for 180 degree angle should be -1. Got %d", dirX)
	}
	if dirY != 0 {
		t.Errorf("dirY for 180 degree angle should be 0. Got %d", dirY)
	}
	angle = 1.5 * math.Pi
	dirX = u.CalcDirXForDirection(angle)
	dirY = u.CalcDirYForDirection(angle)
	if dirX != 0 {
		t.Errorf("dirX for 270 degree angle should be 0. Got %d", dirX)
	}
	if dirY != -1 {
		t.Errorf("dirY for 270 degree angle should be -1. Got %d", dirY)
	}
}

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

func TestMutateNode(t *testing.T) {
	rand.Seed(2)
	node := d.TreeFromAction(d.ActEat)
	expectedID := "1"
	expectedPrint := "├─Eat\n"
	assert.Equal(t, expectedID, node.ID, "Unexpected Node ID")
	assert.Equal(t, expectedPrint, node.Print("", false))
	mutated := d.MutateTree(&node)
	expectedID = "10-1-4"
	expectedPrint = "├─If Organism Ahead\n│ ├─Eat\n│ └─Turn Left\n"
	assert.Equal(t, expectedID, mutated.ID, "Unexpected Node ID after first Mutate")
	assert.Equal(t, expectedPrint, mutated.Print("", false))
}

func TestRegisterNewNode(t *testing.T) {
	rand.Seed(2)
	node := d.TreeFromAction(d.ActEat)
	mutated := d.MutateTree(&node)
	expectedID := "10-1-4"
	expectedPrint := "├─If Organism Ahead\n│ ├─Eat\n│ └─Turn Left\n"

	nodeLibrary := d.NewNodeLibrary()
	// mutate the 'Eat' action and register the new decision tree
	nodeLibrary.RegisterAndReturnNewNode(mutated)
	mutatedNode, ok := nodeLibrary.Map[expectedID]
	if !ok {
		t.Errorf("Mutated Node with with expected ID %s not found after registering", expectedID)
	} else {
		assert.Equal(t, expectedPrint, mutatedNode.Print("", false))
	}
}

func TestCloneNodeLibrary(t *testing.T) {
	rand.Seed(0)
	nodeLibrary := d.NewNodeLibrary()
	clonedLibrary := nodeLibrary.Clone()
	for id := range nodeLibrary.Map {
		_, ok := clonedLibrary.Map[id]
		assert.True(t, ok, "Expected NodeID %s not found in cloned NodeLibrary", id)
	}
}

func TestMutateColor(t *testing.T) {
	rand.Seed(0)
	color := color.RGBA{100, 150, 200, 255}
	expectedR, expectedG, expectedB, expectedA := 100, 150, 200, 255
	mutatedR, mutatedG, mutatedB, mutatedA := u.MutateColor(color).RGBA()
	assert.Equal(t, int(expectedR), int(mutatedR))
	assert.Equal(t, int(expectedG), int(mutatedG))
	assert.Equal(t, int(expectedB), int(mutatedB))
	assert.Equal(t, int(expectedA), int(mutatedA))
}
