package ux

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/instrument"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/replay"
	"github.com/Zebbeni/protozoa/simulation"
	"github.com/Zebbeni/protozoa/utils"
)

const (
	minimapMaxW    = 150
	minimapMaxH    = 120
	minimapPadding = 8
	minimapBorder  = 1
)

type Minimap struct {
	simulation *simulation.Simulation
	grid       *Grid
	camera     *Camera
	width      int
	height     int

	image         *ebiten.Image
	pendingImage  chan *ebiten.Image
	rendering     bool
	lastCycleUsed int
	// lastShowPh / lastShowOrgs / lastOrgColor / lastTheme are the
	// render state the most recent background render used. When the
	// live state differs we invalidate the cached image so the minimap
	// switches appearance without waiting for the 20-cycle refresh
	// interval. Food isn't tracked because the minimap doesn't render
	// food.
	lastShowPh    bool
	lastShowOrgs  bool
	lastOrgColor  mode
	lastTheme     string

	// replayCtrl (optional) lets the minimap detect replay seeks so the
	// cache refreshes as soon as the playhead jumps — even backwards or
	// by less than 20 cycles, where the normal cycle-delta check would
	// miss the change. Nil when the minimap is wired up before a replay
	// controller exists (e.g. live sim) and set later via
	// SetReplayController.
	replayCtrl    *replay.Controller
	lastSeekCount int
}

func NewMinimap(sim *simulation.Simulation, grid *Grid) *Minimap {
	worldW := config.GridUnitsWide()
	worldH := config.GridUnitsHigh()
	scale := min(float64(minimapMaxW)/float64(worldW), float64(minimapMaxH)/float64(worldH))
	w := max(1, int(float64(worldW)*scale))
	h := max(1, int(float64(worldH)*scale))

	return &Minimap{
		simulation:   sim,
		grid:         grid,
		camera:       grid.Camera,
		width:        w,
		height:       h,
		pendingImage: make(chan *ebiten.Image, 1),
		lastShowPh:   grid.ShowPh(),
		lastOrgColor: grid.OrgColor(),
		lastTheme:    config.Theme(),
	}
}

// SetReplayController attaches a replay controller so the minimap can
// invalidate its cached image whenever the user seeks the playhead.
// Safe to call multiple times; nil clears the attachment.
func (m *Minimap) SetReplayController(ctrl *replay.Controller) {
	m.replayCtrl = ctrl
	if ctrl != nil {
		m.lastSeekCount = ctrl.SeekCount
	} else {
		m.lastSeekCount = 0
	}
}

func (m *Minimap) Update() {
	select {
	case img := <-m.pendingImage:
		m.image = img
		m.rendering = false
	default:
	}

	cycle := m.simulation.Cycle()
	curShowPh := m.grid.ShowPh()
	curShowOrgs := m.grid.ShowOrganisms()
	curOrgColor := m.grid.OrgColor()
	curTheme := config.Theme()
	modeChanged := curShowPh != m.lastShowPh ||
		curShowOrgs != m.lastShowOrgs ||
		curOrgColor != m.lastOrgColor ||
		curTheme != m.lastTheme
	seeked := false
	if m.replayCtrl != nil && m.replayCtrl.SeekCount != m.lastSeekCount {
		seeked = true
	}
	stale := m.image == nil || cycle-m.lastCycleUsed >= 20
	if !m.rendering && (stale || modeChanged || seeked) {
		m.rendering = true
		m.lastCycleUsed = cycle
		m.lastShowPh = curShowPh
		m.lastShowOrgs = curShowOrgs
		m.lastOrgColor = curOrgColor
		m.lastTheme = curTheme
		if m.replayCtrl != nil {
			m.lastSeekCount = m.replayCtrl.SeekCount
		}
		// Capture the state by value so the goroutine renders a consistent
		// snapshot even if the user toggles mid-render.
		go m.renderInBackground(curShowPh, curShowOrgs, curOrgColor)
	}
}

func (m *Minimap) Draw(screen *ebiten.Image) {
	if m.image == nil {
		return
	}

	// Hide minimap if both axes fit in the viewport
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

	// Draw minimap with wrapping: the viewport rect stays centered, the world wraps around it.
	worldW := float64(config.GridUnitsWide())
	worldH := float64(config.GridUnitsHigh())
	viewGridW := float64(m.camera.ViewportW) / float64(unitSize)
	viewGridH := float64(m.camera.ViewportH) / float64(unitSize)

	// Compute shift so camera center maps to minimap center
	camCenterX := m.camera.NormalizedX() + viewGridW/2
	camCenterY := m.camera.NormalizedY() + viewGridH/2
	camMiniX := camCenterX / worldW * float64(m.width)
	camMiniY := camCenterY / worldH * float64(m.height)
	shiftX := float64(m.width)/2 - camMiniX
	shiftY := float64(m.height)/2 - camMiniY

	// Render tiled minimap onto a clipped temporary image
	clipped := instrument.NewImage(m.width, m.height)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(shiftX+float64(dx*m.width), shiftY+float64(dy*m.height))
			clipped.DrawImage(m.image, op)
		}
	}

	// Draw the clipped minimap onto the screen
	clipOp := &ebiten.DrawImageOptions{}
	clipOp.GeoM.Translate(float64(drawX), float64(drawY))
	screen.DrawImage(clipped, clipOp)

	// Viewport rectangle centered in the minimap
	rw := min(viewGridW/worldW*float64(m.width), float64(m.width))
	rh := min(viewGridH/worldH*float64(m.height), float64(m.height))
	rx := float64(drawX) + (float64(m.width)-rw)/2
	ry := float64(drawY) + (float64(m.height)-rh)/2

	ebitenutil.DrawLine(screen, rx, ry, rx+rw, ry, themedForeground())
	ebitenutil.DrawLine(screen, rx+rw, ry, rx+rw, ry+rh, themedForeground())
	ebitenutil.DrawLine(screen, rx, ry+rh, rx+rw, ry+rh, themedForeground())
	ebitenutil.DrawLine(screen, rx, ry, rx, ry+rh, themedForeground())
}

func (m *Minimap) HandleClick(screenX, screenY int) bool {
	screenW := config.ScreenWidth()
	screenH := config.ScreenHeight()
	drawX := screenW - m.width - minimapPadding
	drawY := screenH - m.height - minimapPadding

	if screenX < drawX || screenX >= drawX+m.width || screenY < drawY || screenY >= drawY+m.height {
		return false
	}

	// Click position relative to minimap center
	relX := float64(screenX-drawX) - float64(m.width)/2
	relY := float64(screenY-drawY) - float64(m.height)/2

	worldW := float64(config.GridUnitsWide())
	worldH := float64(config.GridUnitsHigh())

	// Convert offset from minimap center to grid units, relative to current camera center
	us := m.camera.GridUnitSize()
	viewGridW := float64(m.camera.ViewportW) / float64(us)
	viewGridH := float64(m.camera.ViewportH) / float64(us)
	curCenterX := m.camera.NormalizedX() + viewGridW/2
	curCenterY := m.camera.NormalizedY() + viewGridH/2

	gridX := int(curCenterX + relX/float64(m.width)*worldW)
	gridY := int(curCenterY + relY/float64(m.height)*worldH)

	// Wrap
	gridX = int(math.Mod(math.Mod(float64(gridX), worldW)+worldW, worldW))
	gridY = int(math.Mod(math.Mod(float64(gridY), worldH)+worldH, worldH))

	m.camera.CenterOn(gridX, gridY)
	return true
}

// renderInBackground paints the minimap to match the grid's current
// pH-toggle, organism-toggle, and organism-colour state.
//
//	showPh    — pH heatmap behind organisms when true; theme bg otherwise
//	showOrgs  — when false organisms are skipped entirely
//	orgColor  — orgColorPhEffect tints by PhEffect; other modes use the
//	            organism's natural colour (the minimap doesn't bother
//	            with health colouring, which is a per-organism gradient
//	            that reads poorly at minimap resolution).
//
// State is captured by the caller (Update) and passed in so that a user
// mid-switch doesn't tear: the goroutine renders one consistent
// snapshot, and the cache is invalidated on the next Update pass.
func (m *Minimap) renderInBackground(showPh, showOrgs bool, orgColor mode) {
	worldW := config.GridUnitsWide()
	worldH := config.GridUnitsHigh()

	phEffectTint := orgColor == orgColorPhEffect

	// Theme background used wherever pH isn't painted.
	tbR, tbG, tbB := config.ThemeBackgroundRGB()
	bgR := byte(tbR * 255)
	bgG := byte(tbG * 255)
	bgB := byte(tbB * 255)

	maxPhEffect := config.MaxOrganismPhGrowthEffect()

	buf := make([]byte, 4*m.width*m.height)

	for py := 0; py < m.height; py++ {
		for px := 0; px < m.width; px++ {
			gx := min(px*worldW/m.width, worldW-1)
			gy := min(py*worldH/m.height, worldH-1)

			point := utils.Point{X: gx, Y: gy}

			var r, g, b byte
			if showPh {
				ph := m.simulation.GetPhAtPoint(point)
				pr, pg, pb, _ := PhValueColor(ph)
				r = byte(pr * 255)
				g = byte(pg * 255)
				b = byte(pb * 255)
			} else {
				r, g, b = bgR, bgG, bgB
			}

			if showOrgs {
				if info := m.simulation.GetOrganismInfoAtPoint(point); info != nil {
					if phEffectTint {
						col := PhEffectColor(organism.PhEffectSpectrumValue(info.PhEffect, maxPhEffect))
						r = byte(col.R * 255)
						g = byte(col.G * 255)
						b = byte(col.B * 255)
					} else {
						cr, cg, cb, _ := info.Color.RGBA()
						r, g, b = byte(cr>>8), byte(cg>>8), byte(cb>>8)
					}
				}
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
