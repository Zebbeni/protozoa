package ux

import (
	"fmt"
	"strings"

	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// hoverInfoText is the readout beside the cursor: what is at the hovered cell, in the order it is worth knowing.
func hoverInfoText(ph float64, info *organism.Info, wallStrength int, item *food.Item, buried int, point utils.Point) string {
	lines := []string{fmt.Sprintf("PH: %2.1f", ph)}

	if info != nil {
		lines = append(lines,
			fmt.Sprintf("ORG: %d", info.ID),
			fmt.Sprintf("SIZE: %.0f", info.Size),
		)
	}
	if wallStrength > 0 {
		lines = append(lines, fmt.Sprintf("WALL: %d", wallStrength))
	}
	if item != nil {
		lines = append(lines, fmt.Sprintf("FOOD: %d", item.Value))
	}
	if buried > 0 {
		lines = append(lines, fmt.Sprintf("BURIED: %d", buried))
	}

	lines = append(lines, fmt.Sprintf("POINT: %v", point))
	return strings.Join(lines, "\n")
}
