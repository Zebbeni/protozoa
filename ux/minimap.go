package ux

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/utils"
)

const (
	minimapMaxW    = 150
	minimapMaxH    = 120
	minimapPadding = 8
	minimapBorder  = 1
)

// Minimap renders a small overview of the full world with a viewport rectangle.
type Minimap struct {
	simulation *simulation.Simulation
	camera     *Camera
	width      int // actual pixel dimensions (proportional to world)
	height     int

	image         *ebiten.Image
	pendingImage  chan *ebiten.Image
	rendering     bool
	lastCycleUsed int
}

// NewMinimap creates a proportionally-sized minimap.
func NewMinimap(sim *simulation.Simulation, cam *Camera) *Minimap {
	worldW := config.GridUnitsWide()
	worldH := config.GridUnitsHigh()
	scale := min(float64(minimapMaxW)/float64(worldW), float64(minimapMaxH)/float64(worldH))
	w := int(float64(worldW) * scale)
	h := int(float64(worldH) * scale)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	return &Minimap{
		simulation:   sim,
		camera:       cam,
		width:        w,
		height:       h,
		pendingImage: make(chan *ebiten.Image, 1),
	}
}

// Update checks for async render results and triggers new renders periodically.
func (m *Minimap) Update() {
	// Check for completed render
	select {
	case img := <-m.pendingImage:
		m.image = img
		m.rendering = false
	default:
	}

	// Trigger new render every 20 cycles
	cycle := m.simulation.Cycle()
	if !m.rendering && (m.image == nil || cycle-m.lastCycleUsed >= 20) {
		m.rendering = true
		m.lastCycleUsed = cycle
		go m.renderInBackground()
	}
}

// Draw composites the minimap onto the screen at the bottom-right of the grid area.
func (m *Minimap) Draw(screen *ebiten.Image) {
	if m.image == nil {
		return
	}

	// Hide minimap if the viewport can see the entire world
	unitSize := m.camera.GridUnitSize()
	if m.camera.ViewportW >= config.GridUnitsWide()*unitSize &&
		m.camera.ViewportH >= config.GridUnitsHigh()*unitSize {
		return
	}

	screenW := config.ScreenWidth()
	screenH := config.ScreenHeight()
	drawX := screenW - m.width - minimapPadding
	drawY := screenH - m.height - minimapPadding

	// Background border
	ebitenutil.DrawRect(screen,
		float64(drawX-minimapBorder), float64(drawY-minimapBorder),
		float64(m.width+minimapBorder*2), float64(m.height+minimapBorder*2),
		color.RGBA{R: 60, G: 60, B: 60, A: 200})

	// Draw minimap image
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(drawX), float64(drawY))
	screen.DrawImage(m.image, op)

	// Draw viewport rectangle
	worldW := float64(config.GridUnitsWide())
	worldH := float64(config.GridUnitsHigh())

	viewGridW := float64(m.camera.ViewportW) / float64(unitSize)
	viewGridH := float64(m.camera.ViewportH) / float64(unitSize)

	rx := float64(drawX) + m.camera.X/worldW*float64(m.width)
	ry := float64(drawY) + m.camera.Y/worldH*float64(m.height)
	rw := viewGridW / worldW * float64(m.width)
	rh := viewGridH / worldH * float64(m.height)

	// Clamp viewport rectangle to minimap bounds
	rx = max(rx, float64(drawX))
	ry = max(ry, float64(drawY))
	rw = min(rw, float64(drawX+m.width)-rx)
	rh = min(rh, float64(drawY+m.height)-ry)

	ebitenutil.DrawLine(screen, rx, ry, rx+rw, ry, color.White)
	ebitenutil.DrawLine(screen, rx+rw, ry, rx+rw, ry+rh, color.White)
	ebitenutil.DrawLine(screen, rx, ry+rh, rx+rw, ry+rh, color.White)
	ebitenutil.DrawLine(screen, rx, ry, rx, ry+rh, color.White)
}

// HandleClick checks if a click is on the minimap and centers the camera there.
// Returns true if the click was consumed.
func (m *Minimap) HandleClick(screenX, screenY int) bool {
	screenW := config.ScreenWidth()
	screenH := config.ScreenHeight()
	drawX := screenW - m.width - minimapPadding
	drawY := screenH - m.height - minimapPadding

	if screenX < drawX || screenX >= drawX+m.width || screenY < drawY || screenY >= drawY+m.height {
		return false
	}

	// Convert minimap pixel to grid coordinate
	relX := float64(screenX - drawX)
	relY := float64(screenY - drawY)
	gridX := int(relX / float64(m.width) * float64(config.GridUnitsWide()))
	gridY := int(relY / float64(m.height) * float64(config.GridUnitsHigh()))

	m.camera.CenterOn(gridX, gridY)
	return true
}

func (m *Minimap) renderInBackground() {
	worldW := config.GridUnitsWide()
	worldH := config.GridUnitsHigh()

	buf := make([]byte, 4*m.width*m.height)

	for py := 0; py < m.height; py++ {
		for px := 0; px < m.width; px++ {
			// Map minimap pixel to grid cell
			gx := px * worldW / m.width
			gy := py * worldH / m.height
			if gx >= worldW {
				gx = worldW - 1
			}
			if gy >= worldH {
				gy = worldH - 1
			}

			point := utils.Point{X: gx, Y: gy}

			var r, g, b byte

			// Check for organism first (most visible)
			if info := m.simulation.GetOrganismInfoAtPoint(point); info != nil {
				cr, cg, cb, _ := info.Color.RGBA()
				r, g, b = byte(cr>>8), byte(cg>>8), byte(cb>>8)
			} else {
				// Fall back to pH coloring
				ph := m.simulation.GetPhAtPoint(point)
				pr, pg, pb, _ := PhValueColor(ph)
				r = byte(pr * 255)
				g = byte(pg * 255)
				b = byte(pb * 255)
			}

			idx := (py*m.width + px) * 4
			buf[idx] = r
			buf[idx+1] = g
			buf[idx+2] = b
			buf[idx+3] = 255
		}
	}

	img := ebiten.NewImage(m.width, m.height)
	img.WritePixels(buf)
	m.pendingImage <- img
}
