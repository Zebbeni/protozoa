package food

import (
	"github.com/Zebbeni/protozoa/utils"
)

// WorldAPI provides functions to make changes
type WorldAPI interface {
	// AddGridPointToUpdate indicates a point on the grid has been updated
	// and needs to be re-rendered
	AddGridPointToUpdate(point utils.Point)
}
