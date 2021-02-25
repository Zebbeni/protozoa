package food

import (
	"github.com/Zebbeni/protozoa/utils"
)

// API provides functions to make changes
type API interface {
	// AddGridPointToUpdate indicates a point on the grid has been updated
	// and needs to be re-rendered
	AddGridPointToUpdate(point utils.Point)
}
