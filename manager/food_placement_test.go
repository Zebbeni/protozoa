package manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// foodAPIStub is a food.API whose world is whatever the test says.
type foodAPIStub struct {
	walls     map[utils.Point]bool
	organisms map[utils.Point]bool
	updates   []utils.Point
}

func (s *foodAPIStub) AddFoodUpdate(p utils.Point)          { s.updates = append(s.updates, p) }
func (s *foodAPIStub) IsWallAtPoint(p utils.Point) bool     { return s.walls[p] }
func (s *foodAPIStub) IsOrganismAtPoint(p utils.Point) bool { return s.organisms[p] }

func foodTestManager(t *testing.T, api *foodAPIStub) *FoodManager {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	g.InitialFood = 0
	config.SetGlobals(&g)
	return NewFoodManager(api, simrand.New(1))
}

// TestFoodIsNotPlacedOnAnOrganism is the rule the reported bug was
// missing. addFood checked for a wall and nothing else, so a random
// spawn could drop food straight onto a living organism — where it then
// sat, rendering underneath it, until the organism moved or ate it.
//
// Organisms can never step onto food (food blocks movement), so this was
// the only way the two ever shared a cell.
func TestFoodIsNotPlacedOnAnOrganism(t *testing.T) {
	occupied := utils.Point{X: 3, Y: 4}
	empty := utils.Point{X: 5, Y: 6}
	walled := utils.Point{X: 7, Y: 8}

	api := &foodAPIStub{
		organisms: map[utils.Point]bool{occupied: true},
		walls:     map[utils.Point]bool{walled: true},
	}
	m := foodTestManager(t, api)

	m.AddFoodAtPoint(occupied, 10)
	if _, ok := m.GetFoodAtPoint(occupied); ok {
		t.Error("food was placed on a cell holding a living organism")
	}

	m.AddFoodAtPoint(walled, 10)
	if _, ok := m.GetFoodAtPoint(walled); ok {
		t.Error("food was placed inside a wall")
	}

	// And an empty cell still works, so the guard hasn't stopped food
	// entering the world at all.
	m.AddFoodAtPoint(empty, 10)
	if _, ok := m.GetFoodAtPoint(empty); !ok {
		t.Error("food wasn't placed on an empty cell")
	}
}

// TestFoodIsNotAddedToAPileUnderAnOrganism: the same cell can already
// hold food when an organism ends up on it — a snapshot restored from a
// run recorded before the rule existed. Topping that pile up would keep
// the overlap alive rather than letting it drain away as the organism
// eats or moves.
func TestFoodIsNotAddedToAPileUnderAnOrganism(t *testing.T) {
	p := utils.Point{X: 2, Y: 2}
	api := &foodAPIStub{}
	m := foodTestManager(t, api)

	m.AddFoodAtPoint(p, 10)
	before, ok := m.GetFoodAtPoint(p)
	if !ok {
		t.Fatal("setup: food wasn't placed")
	}

	// An organism arrives on the pile.
	api.organisms = map[utils.Point]bool{p: true}
	m.AddFoodAtPoint(p, 40)

	after, _ := m.GetFoodAtPoint(p)
	if after.Value != before.Value {
		t.Errorf("food under an organism grew from %d to %d", before.Value, after.Value)
	}
}

// TestRandomFoodAvoidsOrganisms: the random spawn is the path that
// actually caused the reported bug, and it goes through addFood rather
// than choosing its own cell, so it inherits the rule.
func TestRandomFoodAvoidsOrganisms(t *testing.T) {
	api := &foodAPIStub{organisms: map[utils.Point]bool{}}
	m := foodTestManager(t, api)

	// Every cell occupied: nothing can be placed anywhere.
	for x := 0; x < config.GridUnitsWide(); x++ {
		for y := 0; y < config.GridUnitsHigh(); y++ {
			api.organisms[utils.Point{X: x, Y: y}] = true
		}
	}
	for i := 0; i < 500; i++ {
		m.AddRandomFoodItem()
	}
	if n := len(m.GetFoodItems()); n != 0 {
		t.Errorf("%d food items landed on a fully occupied grid", n)
	}
}
