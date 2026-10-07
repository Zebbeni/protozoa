package organism

import (
	"github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
	"github.com/lucasb-eyer/go-colorful"
)

type Info struct {
	ID         int
	Health     float64
	Location   utils.Point
	Direction  utils.Point
	Size       float64
	Action     decision.Action
	AncestorID int
	Color      colorful.Color
	// SecondaryColor tints feature overlays in the high-res renderer (pili / teeth / sensors).
	SecondaryColor colorful.Color
	Age            int
	Children       int
	TraveledDist   int // lifetime grid-unit travel count
	// PhPositive / PhNegative are lifetime cumulative magnitudes the organism has pushed pH up (eating) and down (chemosynthesis).
	PhPositive float64
	PhNegative float64
	// AttackTotal is the lifetime count of attack-action cycles.
	AttackTotal int
	AttackHits  int
	// Status is the resolved outcome of the most recent cycle.
	Status Status
	// BornThisCycle is true for exactly the cycle on which the organism was spawned.
	BornThisCycle bool
	// IdealPh is the centre of the organism's pH tolerance range.
	IdealPh float64
	// Abilities is the organism's ability-score distribution, shown by the panel.
	Abilities physiology.Scores
	// Appearance is the derived sprite composition — body silhouette plus motor / mouth / sensor overlays.
	Appearance physiology.Appearance
	// ActionWeights is each action's share of the decision tree, indexed by the action's code. Shared read-only; never written through.
	ActionWeights []float64
	// HealthLedger is the most recent resolved cycle's health changes by source.
	HealthLedger HealthLedger
	// LineageEndCycle is the latest death in this organism's line of descent, or 0 if a descendant survives to the end of the recorded run (always 0 in a live run).
	LineageEndCycle int
}
