package manager

import (
	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
	"github.com/hajimehoshi/ebiten/v2"
)

// EnvironmentManager contains an image
type EnvironmentManager struct {
	phMap *ebiten.Image
}

func NewEnvironmentManager() *EnvironmentManager {
	manager := &EnvironmentManager{
		phMap: initializePhMap(),
	}

	return manager
}

func initializePhMap() *ebiten.Image {
	phMap := ebiten.NewImage(c.GridUnitsWide(), c.GridUnitsHigh())
	patternMap := resources.PhPatternMap

	patternW, patternH := patternMap.Size()
	scaleW := float64(c.GridUnitsWide()) / float64(patternW)
	scaleH := float64(c.GridUnitsHigh()) / float64(patternH)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scaleW, scaleH)

	phMap.DrawImage(patternMap, op)
	return phMap
}

func (m *EnvironmentManager) GetPhMap() *ebiten.Image {
	return m.phMap
}

// GetPhAtPoint returns the current pH level of the environment at a given point
func (m *EnvironmentManager) GetPhAtPoint(point utils.Point) float64 {
	color := m.phMap.At(point.X, point.Y)
	_, _, b, _ := color.RGBA()
	return float64(b) / 25.5
}

// AddPhChange adds a positive or negative value to pH, bounded by the
// minimum and maximum pH values provided by the config
func (m *EnvironmentManager) AddPhChange(change float64) {

}
