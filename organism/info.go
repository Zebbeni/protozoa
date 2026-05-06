package organism

import (
	"github.com/Zebbeni/protozoa/decision"
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
	Age        int
	Children   int
	PhEffect   float64
	// ChemoFailed is true when the organism's most recent
	// chemosynthesis action was outside its pH tolerance range and
	// produced no health gain. Only meaningful when Action ==
	// ActChemosynthesis; stale values on other actions are ignored by
	// the renderer.
	ChemoFailed bool
	// EatFailed is true when the organism's most recent eat action
	// hit an empty cell (no food consumed). Only meaningful when
	// Action == ActEat; stale on other actions and ignored by the
	// renderer in that case.
	EatFailed bool
	// BornThisCycle is true for exactly the cycle on which the
	// organism was spawned. The animation layer uses it to synthesise
	// a birth Frame (2-cell move from the parent's cell into the
	// child's cell) without needing to carry the parent's location.
	BornThisCycle bool
}
