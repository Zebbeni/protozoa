package manager

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

func noWalls(utils.Point) bool { return false }

// TestWasteLandsBehindEater pins where eating waste goes: behind the
// organism, not on the food cell it just ate from, so a meal doesn't skew
// the pH the eater's own forward sensors read.
func TestWasteLandsBehindEater(t *testing.T) {
	loadDefaultGlobals(t)

	o := &organism.Organism{Location: utils.Point{X: 10, Y: 10}, Direction: utils.Point{X: 1, Y: 0}}
	got := wasteLocation(o, noWalls)
	if want := (utils.Point{X: 9, Y: 10}); got != want {
		t.Errorf("waste at %v, want behind the organism at %v", got, want)
	}
	if ahead := o.Location.Add(o.Direction); got == ahead {
		t.Error("waste landed on the food cell ahead")
	}
}

// TestWasteWrapsAtGridEdge: behind an organism on the edge is the far side
// of the world, same as every other neighbour lookup.
func TestWasteWrapsAtGridEdge(t *testing.T) {
	loadDefaultGlobals(t)

	o := &organism.Organism{Location: utils.Point{X: 0, Y: 5}, Direction: utils.Point{X: 1, Y: 0}}
	want := utils.Point{X: config.GridUnitsWide() - 1, Y: 5}
	if got := wasteLocation(o, noWalls); got != want {
		t.Errorf("waste at %v, want wrapped to %v", got, want)
	}
}

// TestWasteAvoidsWalls: walls freeze pH and don't diffuse, so waste dropped
// into one would vanish from the world's pH balance. It goes into the
// organism's own cell instead.
func TestWasteAvoidsWalls(t *testing.T) {
	loadDefaultGlobals(t)

	o := &organism.Organism{Location: utils.Point{X: 10, Y: 10}, Direction: utils.Point{X: 0, Y: 1}}
	behind := o.Location.Sub(o.Direction)
	wallBehind := func(p utils.Point) bool { return p == behind }

	if got := wasteLocation(o, wallBehind); got != o.Location {
		t.Errorf("with a wall behind, waste at %v, want the organism's own cell %v", got, o.Location)
	}
}
