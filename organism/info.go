package organism

import (
	"github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/utils"
	"image/color"
)

// Info contains all information relevant to rendering an organism
type Info struct {
	ID         int
	Location   utils.Point
	Size       float64
	Action     decision.Action
	AncestorID int
	Color      color.Color
}
