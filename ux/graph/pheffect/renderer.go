package pheffect

import (
	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/organism"
	s "github.com/Zebbeni/protozoa/simulation"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

// Renderer renders a stacked area chart of organisms bucketed by phEffect.
type Renderer struct{}

func NewRenderer() *Renderer { return &Renderer{} }

func (r *Renderer) Reset() {}

func (r *Renderer) Render(sim *s.Simulation, oldBarCount, newBarCount int) *ebiten.Image {
	sim.LockHistoryForReading()
	phEffectMap := sim.GetHistory(manager.HistoryPhEffect)

	maxPop := getMaxFromBucketHistory(phEffectMap, newBarCount)
	if maxPop < 1 {
		maxPop = 1
	}
	heightPerOrg := gh.RealGraphHeight / float64(maxPop)
	barWidth := gh.RealGraphWidth / float64(newBarCount)
	numBuckets := 10

	img := ebiten.NewImage(int(gh.RealGraphWidth), int(gh.RealGraphHeight))
	src := gh.WhiteSrc()

	var vertices []ebiten.Vertex
	var indices []uint16

	for barIdx := 0; barIdx < newBarCount; barIdx++ {
		cycle := barIdx * c.PopulationUpdateInterval()
		prevDist := phEffectMap[cycle-c.PopulationUpdateInterval()]
		newDist := phEffectMap[cycle]

		xLeft := float32(barIdx) * float32(barWidth)
		xRight := float32(barIdx+1) * float32(barWidth)

		prevBottom := float32(gh.RealGraphHeight)
		newBottom := float32(gh.RealGraphHeight)

		for bucket := 0; bucket < numBuckets; bucket++ {
			prevY1, prevY2 := prevBottom, prevBottom
			newY1, newY2 := newBottom, newBottom

			prevCount := prevDist[bucket]
			if prevCount > 0 {
				prevBottom -= float32(prevCount) * float32(heightPerOrg)
				prevY2 = prevBottom
			}
			newCount := newDist[bucket]
			if newCount > 0 {
				newBottom -= float32(newCount) * float32(heightPerOrg)
				newY2 = newBottom
			}
			if prevCount == 0 && newCount == 0 {
				continue
			}

			spectrumValue := (float64(bucket) + 0.5) / float64(numBuckets)
			cr, cg, cb, ca := gh.ColorToFloat(organism.ComputePhEffectColor(spectrumValue))
			gh.FlushAndAppendQuad(&vertices, &indices, img, src,
				xLeft, prevY1, prevY2, xRight, newY1, newY2,
				cr, cg, cb, ca)
		}
	}

	sim.UnlockHistoryForReading()

	if len(vertices) > 0 {
		img.DrawTriangles(vertices, indices, src, nil)
	}
	return img
}

func getMaxFromBucketHistory(history map[int]map[int]int32, barCount int) int {
	maxTotal := int32(0)
	for barIdx := 0; barIdx < barCount; barIdx++ {
		cycle := barIdx * c.PopulationUpdateInterval()
		dist := history[cycle]
		total := int32(0)
		for _, count := range dist {
			total += count
		}
		if total > maxTotal {
			maxTotal = total
		}
	}
	return int(maxTotal)
}
