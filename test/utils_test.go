package test

import (
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

func TestNewNodeLibrary(t *testing.T) {
	rand.Seed(0)
	nodeLibrary := d.NewNodeLibrary()
	if len(nodeLibrary.Map) != 6 {
		t.Errorf("expected 6 initial nodes in library (one per action), got %d", len(nodeLibrary.Map))
	}
}
