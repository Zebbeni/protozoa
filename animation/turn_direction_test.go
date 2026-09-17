package animation

import (
	"testing"

	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// TestTurnsAreDrawnFromTheHeadingTheyLeft: the turn sprites animate the
// rotation themselves, so a turn frame has to be drawn at the heading the
// organism turned *from* — the organism already holds the new one by the
// time the frame is built. Drawing it at the new heading rotated the
// sprite twice and landed the last frame 180° out.
func TestTurnsAreDrawnFromTheHeadingTheyLeft(t *testing.T) {
	for _, start := range []utils.Point{{X: 1, Y: 0}, {X: 0, Y: 1}, {X: -1, Y: 0}, {X: 0, Y: -1}} {
		if got := drawDirection(start.Left(), organism.StatusTurnLeft); got != start {
			t.Errorf("left turn from %v: drawn facing %v, want the heading it left, %v", start, got, start)
		}
		if got := drawDirection(start.Right(), organism.StatusTurnRight); got != start {
			t.Errorf("right turn from %v: drawn facing %v, want the heading it left, %v", start, got, start)
		}

		// The sprite art covers exactly the 90° between the two, so the
		// animation must end where the organism now faces.
		if start.Left() == start || start.Right() == start {
			t.Fatalf("%v: a turn should change the heading", start)
		}
	}

	// Everything else is drawn at the organism's own heading.
	facing := utils.Point{X: 1, Y: 0}
	for _, status := range []organism.Status{
		organism.StatusIdle, organism.StatusMoveSuccess, organism.StatusMoveBlocked,
		organism.StatusEatSuccess, organism.StatusAttacking, organism.StatusDigging,
	} {
		if got := drawDirection(facing, status); got != facing {
			t.Errorf("status %v: drawn facing %v, want %v", status, got, facing)
		}
	}
}
