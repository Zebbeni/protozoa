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
	// SecondaryColor tints feature overlays in the high-res renderer
	// (pili / teeth / sensors). Inherited and mutated alongside
	// Color; view-mode overrides (pH, health) leave it unused since
	// those modes paint the whole organism with one derived colour.
	SecondaryColor colorful.Color
	Age            int
	Children       int
	TraveledDist   int // lifetime grid-unit travel count
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
	// what the action did, how it turned out, or whether the
	// organism is in its terminal Dying/Decaying sequence. Replaces the older
	// ChemoFailed / EatFailed / Posture fields.
	Status Status
	// BornThisCycle is true for exactly the cycle on which the
	// organism was spawned. The animation layer uses it to synthesise
	// a birth Frame (2-cell move from the parent's cell into the
	// child's cell) without needing to carry the parent's location.
	BornThisCycle bool
	// IdealPh is the centre of the organism's pH tolerance range, so the
	// renderer can tell how well it tolerates the pH where it sits.
	IdealPh float64
	// Abilities is the organism's ability-score distribution, shown by
	// the panel.
	Abilities physiology.Scores
	// Appearance is the derived sprite composition — body silhouette
	// plus motor / mouth / sensor overlays. Precomputed on the
	// organism, not derived here, so the renderer never walks a
	// decision tree per frame.
	Appearance physiology.Appearance
	// LineageEndCycle is the latest death in this organism's line of
	// descent, or 0 if a descendant survives to the end of the recorded
	// run (always 0 in a live run). See DescendantNode.LineageEndCycle.
	LineageEndCycle int
}
