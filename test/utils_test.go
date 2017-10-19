package utils_test

import (
	"math"
	"math/rand"
	"testing"

	d "../decisions"
	u "../utils"
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

func TestGenerateRandomSubSequence(t *testing.T) {
	rand.Seed(0)
	// Test random subsequence creation (with expected result for 0 seed)
	expectedString := "C_FoodLeft-A_Right-A_Left"
	sequence := d.NewRandomSubSequence()
	sequenceString := d.PrintSequence(sequence)
	if sequenceString != expectedString {
		t.Errorf("expected sequence: '%s', got %s", expectedString, sequenceString)
	}

	// Test tree creation
	node := d.TreeFromSequence(sequence, sequence)
	if node.YesNode.NodeType != d.ActTurnRight {
		t.Errorf("expected yes action: '%s', got %s", d.Map[d.ActTurnRight], d.Map[node.YesNode.NodeType])
	}
	if node.NoNode.NodeType != d.ActTurnLeft {
		t.Errorf("expected no action: '%s', got %s", d.Map[d.ActTurnLeft], d.Map[node.NoNode.NodeType])
	}
}
