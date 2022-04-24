package manager

import (
	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/environment"
	"math"
)

type EnvironmentManager struct {
	api environment.API

	pH float64
}

func NewEnvironmentManager(api environment.API) *EnvironmentManager {
	manager := &EnvironmentManager{
		api: api,
		pH:  c.InitialPh(),
	}
	return manager
}

// GetPh returns the current pH level of the environment
func (e *EnvironmentManager) GetPh() float64 {
	return e.pH
}

// UpdatePh adds a positive or negative value to pH, bounded by the
// minimum and maximum pH values provided by the config
func (e *EnvironmentManager) UpdatePh(change float64) {
	e.pH = math.Min(math.Max(e.pH+change, c.MinPh()), c.MaxPh())
}
