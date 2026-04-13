package ux

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
)

// renderPhEffect draws a stacked area chart of organisms bucketed by phEffect
func (g *Graph) renderPhEffect(barCount int) *ebiten.Image {
	fmt.Printf("\nrenderPhEffect")

	g.simulation.LockHistoryForReading()
	phEffectMap := g.simulation.GetPhEffectHistory()

	maxPop := getMaxFromBucketHistory(phEffectMap, barCount)
	if maxPop < 1 {
		maxPop = 1
	}
	heightPerOrg := realGraphHeight / float64(maxPop)
	barWidth := realGraphWidth / float64(barCount)
	numBuckets := 10

	img := ebiten.NewImage(int(realGraphWidth), int(realGraphHeight))
	src := whiteSrc()

	var vertices []ebiten.Vertex
	var indices []uint16

	for barIdx := 0; barIdx < barCount; barIdx++ {
		cycle := barIdx * c.PopulationUpdateInterval()
		prevDist := phEffectMap[cycle-c.PopulationUpdateInterval()]
		newDist := phEffectMap[cycle]

		xLeft := float32(barIdx) * float32(barWidth)
		xRight := float32(barIdx+1) * float32(barWidth)

		prevBottom := float32(realGraphHeight)
		newBottom := float32(realGraphHeight)

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
			cr, cg, cb, ca := colorToFloat(PhEffectColor(spectrumValue))
			flushAndAppendQuad(&vertices, &indices, img, src,
				xLeft, prevY1, prevY2, xRight, newY1, newY2,
				cr, cg, cb, ca)
		}
	}

	g.simulation.UnlockHistoryForReading()

	if len(vertices) > 0 {
		img.DrawTriangles(vertices, indices, src, nil)
	}
	return img
}

// getMaxFromBucketHistory returns the max total count across all cycles in a bucket history.
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
