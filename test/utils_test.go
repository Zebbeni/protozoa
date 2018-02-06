package test

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

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

func TestGenerateSequence(t *testing.T) {
	rand.Seed(0)
	// Test random subsequence creation (with expected result for 0 seed)
	expectedString := "Turn Left | Eat | Move Ahead | If Can Move Ahead | Turn Left | If Food Right | Move Ahead | Be Idle | Eat | If Food Right | Be Idle | Be Idle | If Can Move Ahead | Be Idle"
	sequence := d.NewRandomSequence()
	sequenceString := d.PrintSequence(sequence)
	if sequenceString != expectedString {
		t.Errorf("expected sequence: '%s', got %s", expectedString, sequenceString)
	}
}

func TestSequenceTreeCreation(t *testing.T) {
	// Verify no errors when generating a ton of sequences and trees
	for i := 0; i < 1000; i++ {
		fmt.Printf("Test sequence %d\n", i+1)
		// Test tree creation
		sequence := d.NewRandomSequence()
		node := d.TreeFromSequence(sequence)
		fmt.Printf("\n%d Node(s): %s", len(sequence), d.PrintSequence(sequence))
		fmt.Printf("\nTree:\n%s\n", d.PrintNode(node, 1))
	}
}
