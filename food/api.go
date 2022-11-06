package food

import "github.com/Zebbeni/protozoa/utils"

// API provides functions to look up or update information for the sim state
type API interface {
	AddFoodUpdate(p utils.Point)
}
