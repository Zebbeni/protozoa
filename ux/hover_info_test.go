package ux

import (
	"strings"
	"testing"

	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

var hoverPoint = utils.Point{X: 34, Y: 78}

func TestHoverInfoListsEveryOccupant(t *testing.T) {
	info := &organism.Info{ID: 267, Size: 12}
	item := &food.Item{Value: 40}

	got := hoverInfoText(5.5, info, 63, item, 0, hoverPoint)

	for _, want := range []string{"ORG: 267", "SIZE: 12", "WALL: 63", "FOOD: 40"} {
		if !strings.Contains(got, want) {
			t.Errorf("readout %q is missing %q", got, want)
		}
	}
}

func TestHoverInfoOmitsWhatIsNotThere(t *testing.T) {
	got := hoverInfoText(5.5, nil, 0, nil, 0, hoverPoint)

	for _, absent := range []string{"ORG", "SIZE", "WALL", "FOOD"} {
		if strings.Contains(got, absent) {
			t.Errorf("empty cell reported %q: %q", absent, got)
		}
	}
	if !strings.Contains(got, "PH:") || !strings.Contains(got, "POINT:") {
		t.Errorf("empty cell lost its pH or point: %q", got)
	}
	if lines := strings.Split(got, "\n"); len(lines) != 2 {
		t.Errorf("empty cell drew %d lines, want 2: %q", len(lines), got)
	}
}

// TestHoverInfoKeepsItsOrder: pH first and the point last, with whatever is in the cell between them.
func TestHoverInfoKeepsItsOrder(t *testing.T) {
	got := hoverInfoText(5.5, &organism.Info{ID: 1, Size: 2}, 63, &food.Item{Value: 40}, 0, hoverPoint)
	lines := strings.Split(got, "\n")

	if !strings.HasPrefix(lines[0], "PH:") {
		t.Errorf("first line is %q, want the pH", lines[0])
	}
	if last := lines[len(lines)-1]; !strings.HasPrefix(last, "POINT:") {
		t.Errorf("last line is %q, want the point", last)
	}

	// An organism is the thing being looked at; the terrain it is on is context for it.
	org := strings.Index(got, "ORG:")
	wall := strings.Index(got, "WALL:")
	fd := strings.Index(got, "FOOD:")
	if !(org < wall && wall < fd) {
		t.Errorf("occupants out of order (org %d, wall %d, food %d): %q", org, wall, fd, got)
	}
}

func TestHoverInfoShowsAWallUnderAnOrganism(t *testing.T) {
	got := hoverInfoText(5.5, &organism.Info{ID: 9, Size: 4}, 100, nil, 0, hoverPoint)

	if !strings.Contains(got, "WALL: 100") {
		t.Errorf("an organism hid the wall it is standing on: %q", got)
	}
	if !strings.Contains(got, "ORG: 9") {
		t.Errorf("the wall hid the organism: %q", got)
	}
}
