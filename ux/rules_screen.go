package ux

import (
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// RulesScreen renders an explanation of how the simulated world works, split
// across three tabs. It is the About screen; the type and its files keep the
// rules name.
type RulesScreen struct {
	finished bool
	// header is above the tab strip and stays put; tabs are what the strip
	// switches between. Both are built once: assembling them walks the
	// appearance tables and wraps every paragraph, which is not work for a
	// draw call.
	header []rulesBlock
	tabs   []aboutTab
	tab    int
	// scroll is per tab, so switching away and back does not lose the place.
	scroll   []float64
	tabRects []detailTabHitbox
	// started is the clock the sprite animations run against, so every strip
	// on screen shows the same frame.
	started time.Time
	now     func() time.Time
}

const (
	rulesPanelW         = 760
	rulesContentPad     = 20
	rulesLineHeightBody = 16
	rulesLineHeightHead = 22
	rulesParaGap        = 8
	rulesBackBtnW       = 120
	rulesBackBtnH       = 30
	// rulesTabGap is the space above and below the tab strip.
	rulesTabGap = 10
	// rulesTopMargin clears the title, which is anchored to the screen.
	rulesTopMargin = 54
)

func NewRulesScreen() *RulesScreen {
	rs := &RulesScreen{now: time.Now}
	rs.started = rs.now()
	rs.header = aboutHeader()
	rs.tabs = aboutTabs()
	rs.scroll = make([]float64, len(rs.tabs))
	return rs
}

// headerHeight is the summary and the organism crossing the page, which are
// drawn above the tab strip whichever tab is up.
func (rs *RulesScreen) headerHeight() int {
	h := 0
	for _, b := range rs.header {
		h += b.height()
	}
	return h
}

// tabStripY is where the strip sits: under the title and the header.
func (rs *RulesScreen) tabStripY() int {
	return rulesTopMargin + rs.headerHeight() + rulesTabGap
}

// contentTop is the first line of the tab's own blocks.
func (rs *RulesScreen) contentTop() int {
	return rs.tabStripY() + tabStripHeight + rulesTabGap
}

func (rs *RulesScreen) blocks() []rulesBlock {
	if rs.tab < 0 || rs.tab >= len(rs.tabs) {
		return nil
	}
	return rs.tabs[rs.tab].blocks
}

func (rs *RulesScreen) scrollY() float64 {
	if rs.tab < 0 || rs.tab >= len(rs.scroll) {
		return 0
	}
	return rs.scroll[rs.tab]
}

func (rs *RulesScreen) setScrollY(v float64) {
	if rs.tab >= 0 && rs.tab < len(rs.scroll) {
		rs.scroll[rs.tab] = v
	}
}

// rulesFrameHold is how long one sprite frame is held. The grid ties this to
// the simulation speed; here there is no simulation, so it is a plain rate
// slow enough to read.
const rulesFrameHold = 180 * time.Millisecond

// elapsed is how long the screen has been open, which the longer animations
// run against.
func (rs *RulesScreen) elapsed() time.Duration {
	if rs.now == nil {
		return 0
	}
	return rs.now().Sub(rs.started)
}

// spriteFrame is the animation frame every strip shows this draw.
func (rs *RulesScreen) spriteFrame() int {
	if rs.now == nil {
		return 0
	}
	frames := zoomSpriteFrameCounts[r.ZoomHighRes]
	if frames < 1 {
		return 0
	}
	return int(rs.now().Sub(rs.started)/rulesFrameHold) % frames
}

func (rs *RulesScreen) Update() bool {
	if rs.finished {
		return true
	}

	scroll := rs.scrollY()
	_, wy := ebiten.Wheel()
	scroll -= wheelScrollSteps(wy) * 30
	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		scroll += 6
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) {
		scroll -= 6
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		scroll += 200
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
		scroll -= 200
	}
	rs.setScrollY(clampScroll(scroll, rs.maxScroll()))

	// Left and right step through the tabs, the way the strip reads.
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		rs.selectTab(rs.tab + 1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		rs.selectTab(rs.tab - 1)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		rs.finished = true
		return true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		for _, rect := range rs.tabRects {
			if mx >= rect.x && mx < rect.x+rect.w && my >= rect.y && my < rect.y+rect.h {
				rs.selectTab(rect.tab)
				return false
			}
		}
		bx, by := rs.backButtonRect()
		if mx >= bx && mx < bx+rulesBackBtnW && my >= by && my < by+rulesBackBtnH {
			rs.finished = true
			return true
		}
	}
	return false
}

// selectTab switches tabs, ignoring an index off either end so the arrow keys
// stop rather than wrap.
func (rs *RulesScreen) selectTab(i int) {
	if i < 0 || i >= len(rs.tabs) {
		return
	}
	rs.tab = i
}

func clampScroll(v float64, max int) float64 {
	if v < 0 {
		return 0
	}
	if v > float64(max) {
		return float64(max)
	}
	return v
}

// maxScroll is how far the tab on show can be scrolled: what it does not fit
// on screen under the header.
func (rs *RulesScreen) maxScroll() int {
	max := rs.contentHeight() + rs.contentTop() - c.ScreenHeight() + rulesContentPad + rulesBackBtnH
	if max < 0 {
		return 0
	}
	return max
}

// contentHeight is the height of the tab on show, not of the whole document.
func (rs *RulesScreen) contentHeight() int {
	h := 0
	for _, b := range rs.blocks() {
		h += b.height()
	}
	return h
}

func (rs *RulesScreen) Draw(screen *ebiten.Image) {
	fillThemeBackground(screen)

	panelX := (c.ScreenWidth() - rulesPanelW) / 2
	sh := c.ScreenHeight()
	tick := rulesTick{frame: rs.spriteFrame(), elapsed: rs.elapsed()}

	// The tab's own blocks first, so anything scrolled up under the header is
	// painted over by it rather than showing through.
	y := rs.contentTop() - int(rs.scrollY())
	for _, b := range rs.blocks() {
		h := b.height()
		// Culled by its own height, so a tall block is drawn while any part
		// of it is on screen rather than only when its top edge is.
		if y+h > 0 && y < sh {
			b.draw(screen, panelX, y, tick)
		}
		y += h
	}

	// The title, the summary and the tab strip are anchored to the screen, so
	// they need the same cover the Back button has or scrolled text runs
	// straight through them.
	ebitenutil.DrawRect(screen, 0, 0, float64(c.ScreenWidth()),
		float64(rs.tabStripY()+tabStripHeight+rulesTabGap/2), themeBackgroundColor())

	title := "ABOUT"
	tb := boundString(r.FontSourceCodePro12, title)
	text.Draw(screen, title, r.FontSourceCodePro12, (c.ScreenWidth()-tb.Dx())/2, 30, themedForeground())

	hy := rulesTopMargin
	for _, b := range rs.header {
		b.draw(screen, panelX, hy, tick)
		hy += b.height()
	}

	labels := make([]string, 0, len(rs.tabs))
	for _, tab := range rs.tabs {
		labels = append(labels, tab.label)
	}
	rs.tabRects = drawTabStrip(screen, panelX, rs.tabStripY(), rulesPanelW, labels, rs.tab)

	// Back button (anchored to screen, not content — always reachable)
	bx, by := rs.backButtonRect()
	mx, my := ebiten.CursorPosition()
	hovered := mx >= bx && mx < bx+rulesBackBtnW && my >= by && my < by+rulesBackBtnH
	pressed := hovered && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	// Cover any text that scrolled under the button area before drawing it, so the button always reads clearly.
	ebitenutil.DrawRect(screen, float64(bx-10), float64(by-5),
		float64(rulesBackBtnW+20), float64(rulesBackBtnH+10),
		themeBackgroundColor())
	drawMenuButton(screen, bx, by, rulesBackBtnW, rulesBackBtnH, "Back", hovered, pressed, false)
}

func (rs *RulesScreen) backButtonRect() (int, int) {
	bx := (c.ScreenWidth() - rulesBackBtnW) / 2
	by := c.ScreenHeight() - rulesBackBtnH - 15
	return bx, by
}

// wrapParagraph splits s into lines whose rendered width fits panelW.
//
// strings.Fields, not a split that keeps the whitespace it broke on: the
// previous version handed each word back with its leading space attached and
// then joined with another, which put two spaces between every word and one
// in front of every line after the first.
func wrapParagraph(s string, panelW int) []string {
	face := r.FontSourceCodePro10
	if boundString(face, s).Dx() <= panelW {
		return []string{s}
	}
	var lines []string
	var current string
	for _, w := range strings.Fields(s) {
		trial := w
		if current != "" {
			trial = current + " " + w
		}
		if boundString(face, trial).Dx() > panelW && current != "" {
			lines = append(lines, current)
			current = w
			continue
		}
		current = trial
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
