package ph

import (
	"github.com/hajimehoshi/ebiten/v2"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/instrument"
	"github.com/Zebbeni/protozoa/manager"
	s "github.com/Zebbeni/protozoa/simulation"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

const avgPhLineHeight = 100

// maxLineWidth caps the average-pH line image's width. It was one pixel per
// bar with no limit, so a long enough run would ask ebiten for a texture
// wider than the GPU allows (~16384px) and panic. The line is drawn into
// RealGraphWidth (1000px), so past this width bars share a column.
const maxLineWidth = 4096

// Renderer renders a stacked area chart of pH distribution with an avg pH line overlay.
type Renderer struct {
	LastAvgPh float64
}

func NewRenderer() *Renderer { return &Renderer{} }

func (r *Renderer) Reset() {}

// Render ignores progress: a single pass over the recorded buckets is
// fast enough that a progress bar would only flash.
func (r *Renderer) Render(sim *s.Simulation, oldBarCount, newBarCount int, _ *gh.Progress) *ebiten.Image {
	totalCells := c.GridUnitsWide() * c.GridUnitsHigh()
	if totalCells < 1 {
		totalCells = 1
	}
	heightPerCell := gh.RealGraphHeight / float64(totalCells)
	width := float64(gh.GraphImageWidth(newBarCount))
	barWidth := width / float64(newBarCount)
	phBucketWidth := 0.5
	numBuckets := int(c.MaxPh() / phBucketWidth)

	img := instrument.NewImage(int(width), int(gh.RealGraphHeight))
	src := gh.WhiteSrc()

	sim.LockHistoryForReading()
	phDistMap := sim.GetHistory(manager.HistoryPhDistribution)

	var vertices []ebiten.Vertex
	var indices []uint16

	lineCols := min(newBarCount, maxLineWidth)
	lineBuf := make([]byte, 4*lineCols*avgPhLineHeight)
	lineThickness := 1
	lastAvgPh := r.LastAvgPh

	for barIdx := 0; barIdx < newBarCount; barIdx++ {
		cycle := barIdx * c.PopulationUpdateInterval()
		prevCycle := cycle - c.PopulationUpdateInterval()
		prevDist := phDistMap[prevCycle]
		newDist := phDistMap[cycle]

		xLeft := float32(barIdx) * float32(barWidth)
		xRight := float32(barIdx+1) * float32(barWidth)

		prevBottom := float32(gh.RealGraphHeight)
		newBottom := float32(gh.RealGraphHeight)

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
			cr, cg, cb, ca := gh.PhValueColor(phMid)
			gh.FlushAndAppendQuad(&vertices, &indices, img, src,
				xLeft, prevY1, prevY2, xRight, newY1, newY2,
				cr, cg, cb, ca)
		}

		avgPh := computeAvgPh(newDist, phBucketWidth)
		if avgPh >= 0 {
			lastAvgPh = avgPh
			lineY := int(float64(avgPhLineHeight) * (1.0 - (avgPh-c.MinPh())/(c.MaxPh()-c.MinPh())))
			for dy := -lineThickness / 2; dy <= lineThickness/2; dy++ {
				py := lineY + dy
				if py >= 0 && py < avgPhLineHeight {
					idx := (py*lineCols + barIdx*lineCols/newBarCount) * 4
					lineBuf[idx] = 255
					lineBuf[idx+1] = 255
					lineBuf[idx+2] = 255
					lineBuf[idx+3] = 255
				}
			}
		}
	}

	sim.UnlockHistoryForReading()

	if len(vertices) > 0 {
		img.DrawTriangles(vertices, indices, src, nil)
	}

	if newBarCount > 0 {
		lineImg := instrument.NewImage(lineCols, avgPhLineHeight)
		lineImg.WritePixels(lineBuf)

		lineOpts := &ebiten.DrawImageOptions{}
		lineOpts.GeoM.Scale(
			width/float64(lineCols),
			gh.RealGraphHeight/float64(avgPhLineHeight),
		)
		img.DrawImage(lineImg, lineOpts)
	}

	r.LastAvgPh = lastAvgPh
	return img
}

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
