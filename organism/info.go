package organism

import (
	"github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
	"github.com/lucasb-eyer/go-colorful"
)

// Info contains all information relevant to rendering an organism
type Info struct {
	ID         int
	Health     float64
	Location   utils.Point
	Direction  utils.Point
	Size       float64
	Action     decision.Action
	AncestorID int
	Color      colorful.Color
	Age          int
	Children     int
	TraveledDist int // lifetime grid-unit travel count
	// PhPositive / PhNegative are lifetime cumulative magnitudes the
	// organism has pushed pH up (eating) and down (chemosynthesis).
	// Both are non-negative; the renderer derives a tint from the
	// imbalance between them.
	PhPositive float64
	PhNegative float64
	// AttackTotal is the lifetime count of attack-action cycles.
	// AttackHits is the subset that landed on an organism in the target
	// cell at the moment of the attack.
	AttackTotal int
	AttackHits  int
	// Status is the resolved outcome of the most recent cycle —
	// what the action did, how it turned out, and whether the
	// organism is currently in a posture mode (Hunker/Flare/Hide)
	// or its terminal Dying/Decaying sequence. Replaces the older
	// ChemoFailed / EatFailed / Posture fields.
	Status Status
	// BornThisCycle is true for exactly the cycle on which the
	// organism was spawned. The animation layer uses it to synthesise
	// a birth Frame (2-cell move from the parent's cell into the
	// child's cell) without needing to carry the parent's location.
	BornThisCycle bool
	// Features is the organism's evolved physiology bitmask. The
	// high-res sprite renderer reads this to pick the body variant
	// (defense tree) and feature overlays (other trees) to composite.
	Features physiology.Set
}
