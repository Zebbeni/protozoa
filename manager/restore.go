package manager

import (
	"image/color"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/checkpoint"
	"github.com/Zebbeni/protozoa/environment"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// RestoreEnvironmentManager creates an EnvironmentManager with pre-populated pH maps.
func RestoreEnvironmentManager(api environment.API, currentPh, previousPh [][]float64) *EnvironmentManager {
	return &EnvironmentManager{
		api:           api,
		currentPhMap:  currentPh,
		previousPhMap: previousPh,
	}
}

// RestoreFoodManager creates a FoodManager with pre-populated food items.
func RestoreFoodManager(api food.API, rng *simrand.RNG, items []checkpoint.FoodRecord) *FoodManager {
	foodItems := make(map[string]*food.Item)
	for _, rec := range items {
		p := utils.Point{X: rec.X, Y: rec.Y}
		foodItems[p.ToString()] = food.NewItem(p, rec.Value)
	}
	return &FoodManager{
		api:           api,
		rng:           rng,
		Items:         foodItems,
		isInitialized: true,
	}
}

// RestoreOrganismManager creates an OrganismManager with pre-populated state.
func RestoreOrganismManager(
	api organism.API, rng *simrand.RNG,
	organisms map[int]*organism.Organism,
	grid [][]int, totalCreated int,
	ancestorIDs []int, ancestorColors map[int]color.Color,
) *OrganismManager {
	return &OrganismManager{
		api:                    api,
		rng:                    rng,
		requestManager:         RequestManager{},
		organisms:              organisms,
		organismIDGrid:         grid,
		totalOrganismsCreated:  totalCreated,
		organismIds:            make([]int, 0, c.MaxOrganisms()),
		originalAncestors:      ancestorIDs,
		originalAncestorColors: ancestorColors,
		descendantTrees:        make(map[int]*organism.DescendantNode),
		history: map[HistoryType]map[int]map[int]int32{
			HistoryPopulation:     make(map[int]map[int]int32),
			HistoryPhEffect:       make(map[int]map[int]int32),
			HistoryPhDistribution: make(map[int]map[int]int32),
		},
	}
}
