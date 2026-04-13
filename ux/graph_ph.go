package ux

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
)

const avgPhLineHeight = 100 // pixel height of the avg pH line overlay image

// renderPh draws a stacked area chart of pH distribution across grid cells
func (g *Graph) renderPh(barCount int) *ebiten.Image {
	fmt.Printf("\nrenderPh")

	totalCells := c.GridUnitsWide() * c.GridUnitsHigh()
	if totalCells < 1 {
		totalCells = 1
	}
	heightPerCell := realGraphHeight / float64(totalCells)
	barWidth := realGraphWidth / float64(barCount)
	phBucketWidth := 0.5
	numBuckets := int(c.MaxPh() / phBucketWidth)

	img := ebiten.NewImage(int(realGraphWidth), int(realGraphHeight))
	src := whiteSrc()

	g.simulation.LockHistoryForReading()
	phDistMap := g.simulation.GetPhDistributionHistory()

	var vertices []ebiten.Vertex
	var indices []uint16

	// Build the avg pH line pixel buffer (1px per bar, avgPhLineHeight tall)
	lineBuf := make([]byte, 4*barCount*avgPhLineHeight)
	lineThickness := 1 // pixels
	lastAvgPh := g.lastAvgPh

	for barIdx := 0; barIdx < barCount; barIdx++ {
		cycle := barIdx * c.PopulationUpdateInterval()
		prevCycle := cycle - c.PopulationUpdateInterval()
		prevDist := phDistMap[prevCycle]
		newDist := phDistMap[cycle]

		xLeft := float32(barIdx) * float32(barWidth)
		xRight := float32(barIdx+1) * float32(barWidth)

		prevBottom := float32(realGraphHeight)
		newBottom := float32(realGraphHeight)

		for bucket := 0; bucket < numBuckets; bucket++ {
			prevY1, prevY2 := prevBottom, prevBottom
			newY1, newY2 := newBottom, newBottom

			prevCount := prevDist[bucket]
			if prevCount > 0 {
				prevBottom -= float32(prevCount) * float32(heightPerCell)
				prevY2 = prevBottom
			}
			newCount := newDist[bucket]
			if newCount > 0 {
				newBottom -= float32(newCount) * float32(heightPerCell)
				newY2 = newBottom
			}
			if prevCount == 0 && newCount == 0 {
				continue
			}

			phMid := (float64(bucket) + 0.5) * phBucketWidth
			cr, cg, cb, ca := phValueColor(phMid)
			flushAndAppendQuad(&vertices, &indices, img, src,
				xLeft, prevY1, prevY2, xRight, newY1, newY2,
				cr, cg, cb, ca)
		}

		// Compute avg pH for this bar and draw into line buffer
		avgPh := computeAvgPh(newDist, phBucketWidth)
		if avgPh >= 0 {
			lastAvgPh = avgPh
			// Map pH to a Y pixel in the line image: MaxPh=top, MinPh=bottom
			lineY := int(float64(avgPhLineHeight) * (1.0 - (avgPh-c.MinPh())/(c.MaxPh()-c.MinPh())))
			for dy := -lineThickness / 2; dy <= lineThickness/2; dy++ {
				py := lineY + dy
				if py >= 0 && py < avgPhLineHeight {
					idx := (py*barCount + barIdx) * 4
					lineBuf[idx] = 255
					lineBuf[idx+1] = 255
					lineBuf[idx+2] = 255
					lineBuf[idx+3] = 255
				}
			}
		}
	}

	g.simulation.UnlockHistoryForReading()

	if len(vertices) > 0 {
		img.DrawTriangles(vertices, indices, src, nil)
	}

	// Overlay the avg pH line image, scaled to match the main graph
	if barCount > 0 {
		lineImg := ebiten.NewImage(barCount, avgPhLineHeight)
		lineImg.WritePixels(lineBuf)

		lineOpts := &ebiten.DrawImageOptions{}
		lineOpts.GeoM.Scale(
			realGraphWidth/float64(barCount),
			realGraphHeight/float64(avgPhLineHeight),
		)
		img.DrawImage(lineImg, lineOpts)
	}

	g.lastAvgPh = lastAvgPh

	return img
}

// computeAvgPh returns the weighted average pH from a bucket distribution.
// Returns -1 if no data.
func computeAvgPh(dist map[int]int32, bucketWidth float64) float64 {
	totalCount := float64(0)
	weightedSum := float64(0)
	for bucket, count := range dist {
		mid := (float64(bucket) + 0.5) * bucketWidth
		weightedSum += mid * float64(count)
		totalCount += float64(count)
	}
	if totalCount == 0 {
		return -1
	}
	return weightedSum / totalCount
}

// phValueColor maps a pH value to a color using the same spectrum as the grid
// pH rendering (green for acid, pink for base, dark for neutral)
func phValueColor(ph float64) (float32, float32, float32, float32) {
	hue := phMaxHue - (phMaxHue * ph / c.MaxPh())
	sat := math.Abs(ph-((c.MaxPh()+c.MinPh())/2.0)) / (c.MaxPh() - c.MinPh())
	light := 0.5 + (0.5 * math.Sin(math.Pi*(sat-0.5)))
	col := colorful.HSLuv(hue, sat, light)
	return colorToFloat(col)
}
