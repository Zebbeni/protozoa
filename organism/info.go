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
	Size       float64
	Action     decision.Action
	AncestorID int
	Color      colorful.Color
	Age        int
	Children   int
	PhEffect   float64
}
