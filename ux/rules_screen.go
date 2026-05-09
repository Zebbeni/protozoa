package ux

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

// RulesScreen renders a scrollable explanation of how the simulated
// world works — meant to ship the README's rules section in-app so a
// new viewer doesn't have to leave the program to find them.
//
// Returns true from Update once the user clicks Back.
type RulesScreen struct {
	scrollY  float64
	finished bool
}

// rulesParagraphs is the body text, one entry per visual paragraph.
// Section headers start with "# " — drawn larger / accented — body
// paragraphs are plain. Wrapping is done at draw time using a fixed
// character width so layout doesn't drift across windows.
var rulesParagraphs = []string{
	"# THE WORLD",
	"Protozoa is a 2D wraparound grid of cells. Each cell has a pH value between 0 and 10 that diffuses cycle-by-cycle towards a global equilibrium. Acidic cells render green, alkaline cells pink, neutral cells dark. Walls can divide the grid into sealed pools so isolated lineages develop in parallel.",
	"# ORGANISMS",
	"Each organism is a coloured square with a set of inherited traits and a single decision tree. Every cycle it picks one action — chemosynthesise, eat, move, turn, attack, feed, idle, or spawn — by walking the tree against its current surroundings. Most actions cost a small amount of health (\"energy\"); chemosynthesis at a healthy pH gains a little.",
	"An organism's health is capped by its size. When it gains more health than its size allows, it grows. When health reaches zero, it dies and leaves behind a food item with value equal to its size at death.",
	"# TRAITS",
	"Initial organisms are seeded with random traits. Children inherit their parent's traits with small mutations. Surviving lineages tend to converge on traits that suit the local environment.",
	"  Color — purely visual; randomised hue/sat/brightness.",
	"  MaxSize — biggest the organism can grow.",
	"  SpawnHealth — health a child starts with (also subtracted from the parent).",
	"  MinHealthToSpawn — minimum parent health required to spawn.",
	"  MinCyclesBetweenSpawns — cooldown between spawns.",
	"  IdealPh — centre of the organism's preferred pH range.",
	"# pH EFFECTS",
	"pH tolerance is global, not per-organism: every organism takes damage outside IdealPh ± Tolerance. Organisms also shape their environment by acting:",
	"  Successful chemosynthesis pushes the local pH down (more acidic) by a small amount per organism size.",
	"  Successful eating pushes the local pH up (more alkaline) by a small amount per food unit consumed.",
	"Cumulative pH-shift per organism is tracked over its lifetime — colour mode \"pH effect\" tints organisms by net push.",
	"# DECISION TREES",
	"Trees mix conditions (\"is food ahead?\", \"is health above 50%?\") with action leaves. Random trees mean lots of redundant or unreachable branches early on; mutation slowly weeds them out. To reward parsimony, each tree node costs a tiny health drain per cycle, so simpler trees that achieve the same behaviour outpace bloated ones over time.",
	"# DEATH AND FAMILY TREES",
	"Every spawn is recorded against its parent, building a descendant family tree per original ancestor. The replay viewer lets you click into any node, scrub through cycles, and watch how a single lineage rose, branched, and died out. Population graphs colour each lineage by its founder's hue.",
	"# THE REPLAY",
	"Every run writes a deterministic .pzr replay file. The viewer re-runs the simulation against this file — same seed, same RNG, same outcomes. Scrub the timeline, change zoom, swap colour modes, or click an organism to see its traits and decision tree. \"Load Previous\" from the main menu re-opens the most recent run.",
}

const (
	rulesPanelW         = 760
	rulesContentPad     = 20
	rulesLineHeightBody = 16
	rulesLineHeightHead = 22
	rulesParaGap        = 8
	rulesBackBtnW       = 120
	rulesBackBtnH       = 30
)

func NewRulesScreen() *RulesScreen { return &RulesScreen{} }

func (r *RulesScreen) Update() bool {
	if r.finished {
		return true
	}

	_, wy := ebiten.Wheel()
	r.scrollY -= wy * 30
	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		r.scrollY += 6
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) {
		r.scrollY -= 6
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		r.scrollY += 200
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
		r.scrollY -= 200
	}
	if r.scrollY < 0 {
		r.scrollY = 0
	}
	maxScroll := r.contentHeight() - c.ScreenHeight() + rulesContentPad*2 + rulesBackBtnH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if r.scrollY > float64(maxScroll) {
		r.scrollY = float64(maxScroll)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		r.finished = true
		return true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		bx, by := r.backButtonRect()
		if mx >= bx && mx < bx+rulesBackBtnW && my >= by && my < by+rulesBackBtnH {
			r.finished = true
			return true
		}
	}
	return false
}

// contentHeight pre-counts how tall the body will render, used to clamp
// scrollY so the user can't scroll past the last paragraph.
func (rs *RulesScreen) contentHeight() int {
	h := rulesContentPad
	for _, p := range rulesParagraphs {
		if isHeading(p) {
			h += rulesLineHeightHead + rulesParaGap
			continue
		}
		h += len(wrapParagraph(p, rulesPanelW)) * rulesLineHeightBody
		h += rulesParaGap
	}
	return h
}

func (rs *RulesScreen) Draw(screen *ebiten.Image) {
	fillThemeBackground(screen)

	// Title
	title := "RULES"
	tb := boundString(r.FontSourceCodePro12, title)
	tx := (c.ScreenWidth() - tb.Dx()) / 2
	text.Draw(screen, title, r.FontSourceCodePro12, tx, 30, themedForeground())

	panelX := (c.ScreenWidth() - rulesPanelW) / 2
	y := 60 - int(rs.scrollY)
	sh := c.ScreenHeight()

	for _, p := range rulesParagraphs {
		if isHeading(p) {
			label := p[2:]
			if y > -rulesLineHeightHead && y < sh {
				text.Draw(screen, label, r.FontSourceCodePro12, panelX, y+14,
					color.RGBA{R: 180, G: 180, B: 255, A: 255})
			}
			y += rulesLineHeightHead + rulesParaGap
			continue
		}
		for _, line := range wrapParagraph(p, rulesPanelW) {
			if y > -rulesLineHeightBody && y < sh {
				text.Draw(screen, line, r.FontSourceCodePro10, panelX, y+12, themedForegroundDim())
			}
			y += rulesLineHeightBody
		}
		y += rulesParaGap
	}

	// Back button (anchored to screen, not content — always reachable)
	bx, by := rs.backButtonRect()
	mx, my := ebiten.CursorPosition()
	hovered := mx >= bx && mx < bx+rulesBackBtnW && my >= by && my < by+rulesBackBtnH
	pressed := hovered && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	// Cover any text that scrolled under the button area before drawing
	// it, so the button always reads clearly.
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

func isHeading(s string) bool { return len(s) >= 2 && s[0] == '#' && s[1] == ' ' }

// wrapParagraph splits s into lines whose rendered width fits panelW.
// Word-wraps at spaces; never splits a word. Uses the body font's
// advance for measurement so the wrap matches what Draw will paint.
func wrapParagraph(s string, panelW int) []string {
	face := r.FontSourceCodePro10
	if boundString(face, s).Dx() <= panelW {
		return []string{s}
	}
	var lines []string
	var current string
	words := splitWords(s)
	for _, w := range words {
		var trial string
		if current == "" {
			trial = w
		} else {
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

// splitWords splits s on spaces, preserving leading whitespace runs as
// part of the next word so indented bullets like "  Color — ..." keep
// their indent on the first line.
func splitWords(s string) []string {
	var out []string
	i := 0
	for i < len(s) {
		j := i
		for j < len(s) && s[j] == ' ' {
			j++
		}
		k := j
		for k < len(s) && s[k] != ' ' {
			k++
		}
		if i < j {
			out = append(out, s[i:k])
		} else if j < k {
			out = append(out, s[j:k])
		}
		i = k
	}
	return out
}
