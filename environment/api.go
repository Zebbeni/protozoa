package environment

import "github.com/Zebbeni/protozoa/utils"

// API provides functions to look up information about the sim state
type API interface {
	Cycle() int
	AddPhUpdate(p utils.Point)
}
