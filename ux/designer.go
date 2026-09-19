package ux

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	c "github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

// DesignerResult is what the designer screen hands back to the runner.
type DesignerResult int

const (
	// DesignerRunning means the user is still editing.
	DesignerRunning DesignerResult = iota
	// DesignerBack means they asked to return to the main menu.
	DesignerBack
)

// Designer is the organism editor: a portrait of the organism being
// built, the knobs that define it, and the decision tree it will run.
//
// Everything on screen is derived from one organism.Design, which is
// also exactly what gets saved — so the portrait can't drift from the
// file, and a design loaded back in rebuilds the same screen. The
// decision tree is held as live nodes rather than as the serialized
// string, since editing it is the point; it's serialized on save.
type Designer struct {
	design organism.Design
	tree   *d.Node

	// Existing designs, so the editor can load one back in and so a
	// save can report what the list now holds.
	saved []organism.Design

	// nameEditing routes typed characters into the name field.
	nameEditing bool

	// picker is the open node dropdown, nil when nothing is being
	// picked. It holds the node it will rewrite, not an index, so the
	// tree can be rebuilt underneath it without the target going stale.
	picker *nodePicker

	// confirmDelete is the saved design whose delete is armed, -1 for
	// none. Deleting takes two clicks: the button sits beside a list the
	// user is clicking through to load things, and a design is work that
	// can't be recovered from anywhere else.
	confirmDelete int

	// Hit rects rebuilt every Draw, so layout lives in one place and
	// clicks are tested against exactly what was painted.
	hits []designerHit

	message string
}

// designerHitKind is what a click on a designer rect does.
type designerHitKind int

const (
	hitNone designerHitKind = iota
	hitName
	hitColor
	hitSecondaryColor
	hitTraitDown
	hitTraitUp
	hitAbilityDown
	hitAbilityUp
	hitTreeNode
	hitPickerOption
	hitSave
	hitBack
	hitLoad
	hitDelete
	hitNew
)

// designerHit is one clickable rect painted this frame.
type designerHit struct {
	x, y, w, h int
	kind       designerHitKind
	// index means: the trait row, the ability, the palette swatch, the
	// design to load, or the option in the open picker.
	index int
	node  *d.Node
}

// nodePicker is the open dropdown: every action and condition the tree
// can hold, anchored under the node that was clicked.
type nodePicker struct {
	node    *d.Node
	x, y    int
	options []pickerOption
}

// pickerOption is one entry in the dropdown. Actions and conditions are
// listed together because that is the choice being made — what this node
// *is* — and splitting them into two menus would hide that converting
// between them is allowed.
type pickerOption struct {
	label     string
	action    d.Action
	condition d.Condition
	isAction  bool
}

// designerTrait is one editable number, described once so the rows draw,
// step and clamp from the same place.
type designerTrait struct {
	label  string
	step   float64
	format string
	get    func(*organism.Design) float64
	set    func(*organism.Design, float64)
	lo     func() float64
	hi     func() float64
}

var designerTraits = []designerTrait{
	{
		label: "Max size", step: 1, format: "%.0f",
		get: func(ds *organism.Design) float64 { return ds.MaxSize },
		set: func(ds *organism.Design, v float64) { ds.MaxSize = v },
		lo:  c.MinimumMaxSize, hi: c.MaximumMaxSize,
	},
	{
		label: "Spawn health", step: 0.5, format: "%.1f",
		get: func(ds *organism.Design) float64 { return ds.SpawnHealth },
		set: func(ds *organism.Design, v float64) { ds.SpawnHealth = v },
		lo:  c.MinSpawnHealth, hi: c.MaximumInitialSpawnHealth,
	},
	{
		label: "Min health to spawn", step: 1, format: "%.1f",
		get: func(ds *organism.Design) float64 { return ds.MinHealthToSpawn },
		set: func(ds *organism.Design, v float64) { ds.MinHealthToSpawn = v },
		lo:  c.MinSpawnHealth, hi: c.MaximumMaxSize,
	},
	{
		label: "Spawn cooldown", step: 1, format: "%.0f",
		get: func(ds *organism.Design) float64 { return float64(ds.MinCyclesBetweenSpawns) },
		set: func(ds *organism.Design, v float64) { ds.MinCyclesBetweenSpawns = int(v) },
		lo:  func() float64 { return 0 },
		hi:  func() float64 { return float64(c.MaxCyclesBetweenSpawns()) },
	},
	{
		label: "Ideal pH", step: 0.25, format: "%.2f",
		get: func(ds *organism.Design) float64 { return ds.IdealPh },
		set: func(ds *organism.Design, v float64) { ds.IdealPh = v },
		lo:  c.MinIdealPh, hi: c.MaxIdealPh,
	},
}

// designerPalette is the colour choice offered for body and features.
// The same eight hues the animation preview uses, so an organism
// designed here looks like one the sprite work was checked against.
var designerPalette = []colorful.Color{
	colorful.HSLuv(0, 0.7, 0.55),
	colorful.HSLuv(30, 0.85, 0.55),
	colorful.HSLuv(60, 0.9, 0.6),
	colorful.HSLuv(120, 0.6, 0.5),
	colorful.HSLuv(180, 0.7, 0.55),
	colorful.HSLuv(240, 0.7, 0.55),
	colorful.HSLuv(290, 0.7, 0.55),
	colorful.HSLuv(0, 0, 0.85),
}

// Designer layout. Three columns: the organism on the left, its numbers
// in the middle, its behaviour on the right.
const (
	designerPad       = 32
	designerRowH      = 22
	designerColGap    = 28
	designerPortraitW = 260
	designerBtnW      = 120
	designerBtnH      = 30
	designerStepW     = 22
	designerSwatch    = 22
	designerDeleteW   = 22
)

// NewDesigner opens the editor on a fresh design.
func NewDesigner() *Designer {
	ds := organism.NewDesign("")
	dz := &Designer{design: ds, saved: organism.LoadDesigns(organism.DesignsDir), confirmDelete: -1}
	dz.tree = organism.StarterTree().Node
	dz.nameEditing = true
	return dz
}

// Update handles input for one frame and reports whether the user is
// done with the screen.
func (dz *Designer) Update() DesignerResult {
	if dz.nameEditing {
		dz.typeName()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if dz.picker != nil {
			dz.picker = nil
		} else if dz.nameEditing {
			dz.nameEditing = false
		} else {
			return DesignerBack
		}
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return DesignerRunning
	}

	mx, my := ebiten.CursorPosition()
	hit, ok := dz.hitAt(mx, my)
	if !ok {
		// A click anywhere else closes the dropdown and commits the
		// name, so neither can be left open by accident.
		dz.picker = nil
		dz.nameEditing = false
		return DesignerRunning
	}
	// Any click that isn't the name field ends naming, so keystrokes
	// can't keep landing in a field the user has visibly left.
	if hit.kind != hitName {
		dz.nameEditing = false
	}
	// An armed delete only survives a click on the same button, so it
	// can't be left waiting to catch a later misclick.
	if hit.kind != hitDelete || hit.index != dz.confirmDelete {
		dz.confirmDelete = -1
	}
	switch hit.kind {
	case hitName:
		dz.nameEditing = true
	case hitColor:
		dz.design.Color = designerPalette[hit.index].Hex()
	case hitSecondaryColor:
		dz.design.SecondaryColor = designerPalette[hit.index].Hex()
	case hitTraitDown:
		dz.stepTrait(hit.index, -1)
	case hitTraitUp:
		dz.stepTrait(hit.index, +1)
	case hitAbilityDown:
		dz.stepAbility(hit.index, -1)
	case hitAbilityUp:
		dz.stepAbility(hit.index, +1)
	case hitTreeNode:
		dz.openPicker(hit.node, hit.x, hit.y)
	case hitPickerOption:
		dz.applyPickerOption(hit.index)
	case hitSave:
		dz.save()
	case hitLoad:
		dz.load(hit.index)
	case hitDelete:
		dz.deleteSaved(hit.index)
	case hitNew:
		*dz = *NewDesigner()
	case hitBack:
		return DesignerBack
	}
	return DesignerRunning
}

// hitAt finds the rect under the cursor. Painted later wins, so the open
// dropdown takes clicks from the rows it covers.
func (dz *Designer) hitAt(mx, my int) (designerHit, bool) {
	for i := len(dz.hits) - 1; i >= 0; i-- {
		h := dz.hits[i]
		if mx >= h.x && mx < h.x+h.w && my >= h.y && my < h.y+h.h {
			return h, true
		}
	}
	return designerHit{}, false
}

// typeName routes keystrokes into the name. Letters, digits, spaces and
// dashes only: the name becomes a filename.
func (dz *Designer) typeName() {
	for _, ch := range ebiten.AppendInputChars(nil) {
		if ch == ' ' || ch == '-' || ch == '_' ||
			(ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			if len(dz.design.Name) < 24 {
				dz.design.Name += string(ch)
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && dz.design.Name != "" {
		dz.design.Name = dz.design.Name[:len(dz.design.Name)-1]
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		dz.nameEditing = false
	}
}

func (dz *Designer) stepTrait(i, dir int) {
	if i < 0 || i >= len(designerTraits) {
		return
	}
	t := designerTraits[i]
	v := t.get(&dz.design) + float64(dir)*t.step
	t.set(&dz.design, min(t.hi(), max(t.lo(), v)))
}

// stepAbility moves one point into or out of an ability. The budget is
// fixed, so the screen shows the running total and refuses to save until
// it adds up — the same contract the config screen's initial scores use,
// rather than silently taking the point from somewhere else.
func (dz *Designer) stepAbility(i, dir int) {
	if i < 0 || i >= len(dz.design.Abilities) {
		return
	}
	v := dz.design.Abilities[i] + dir
	dz.design.Abilities[i] = min(physiology.MaxAbilityScore, max(0, v))
}

// openPicker opens the dropdown for a node.
func (dz *Designer) openPicker(node *d.Node, x, y int) {
	if node == nil {
		return
	}
	options := make([]pickerOption, 0, len(d.MutableActions)+len(d.MutableConditions))
	for _, a := range d.MutableActions {
		options = append(options, pickerOption{label: d.Map[a], action: a, isAction: true})
	}
	for _, cond := range d.MutableConditions {
		options = append(options, pickerOption{label: d.Map[cond], condition: cond})
	}
	dz.picker = &nodePicker{node: node, x: x, y: y, options: options}
}

// applyPickerOption rewrites the picked node.
//
// An action becoming a condition grows two blank branches, because a
// condition with nothing to choose between isn't a decision. A condition
// becoming an action drops its branches — the subtree it was choosing
// between has nowhere left to hang.
func (dz *Designer) applyPickerOption(i int) {
	if dz.picker == nil || i < 0 || i >= len(dz.picker.options) {
		return
	}
	opt := dz.picker.options[i]
	node := dz.picker.node
	dz.picker = nil

	switch {
	case opt.isAction:
		node.NodeType = opt.action
		node.YesNode, node.NoNode = nil, nil
	case node.IsCondition():
		// Condition to condition: the branches still mean something, so
		// they stay.
		node.NodeType = opt.condition
	default:
		if dz.treeSize()+2 > c.MaxDecisionTreeSize() {
			dz.message = fmt.Sprintf("tree is at the %d-node limit", c.MaxDecisionTreeSize())
			return
		}
		node.NodeType = opt.condition
		node.YesNode = d.NodeFromAction(d.ActIdle)
		node.NoNode = d.NodeFromAction(d.ActIdle)
	}
	dz.tree.CalcAndUpdateSize()
	dz.message = ""
}

func (dz *Designer) treeSize() int {
	if dz.tree == nil {
		return 0
	}
	return dz.tree.CalcAndUpdateSize()
}

// currentTree wraps the edited nodes as a Tree for anything that needs
// a whole one — the portrait's appearance, and saving.
func (dz *Designer) currentTree() *d.Tree {
	return d.TreeFromNode(dz.tree)
}

// scores reads the ability rows, which may not add up yet.
func (dz *Designer) scores() physiology.Scores {
	var s physiology.Scores
	for i, v := range dz.design.Abilities {
		if i < len(s) {
			s[i] = v
		}
	}
	return s
}

func (dz *Designer) abilityTotal() int {
	total := 0
	for _, v := range dz.design.Abilities {
		total += v
	}
	return total
}

// save writes the design, then refreshes the saved list so the load row
// shows it immediately.
func (dz *Designer) save() {
	if reason := dz.saveBlockedReason(); reason != "" {
		dz.message = reason
		return
	}
	dz.design.DecisionTree = dz.currentTree().Serialize()
	path, err := organism.SaveDesign(organism.DesignsDir, dz.design)
	if err != nil {
		dz.message = err.Error()
		return
	}
	dz.saved = organism.LoadDesigns(organism.DesignsDir)
	dz.message = "saved to " + path
}

// deleteSaved removes a saved design, after arming. The first click
// arms it and the second does it; any other click disarms, so the
// confirm can't be left waiting to catch a later misclick.
func (dz *Designer) deleteSaved(i int) {
	if i < 0 || i >= len(dz.saved) {
		return
	}
	if dz.confirmDelete != i {
		dz.confirmDelete = i
		dz.message = "click again to delete " + dz.saved[i].Name
		return
	}
	name := dz.saved[i].Name
	dz.confirmDelete = -1
	if err := organism.DeleteDesign(organism.DesignsDir, name); err != nil {
		dz.message = err.Error()
		return
	}
	dz.saved = organism.LoadDesigns(organism.DesignsDir)
	dz.message = "deleted " + name
}

// load replaces the editor's contents with a saved design.
func (dz *Designer) load(i int) {
	if i < 0 || i >= len(dz.saved) {
		return
	}
	ds := dz.saved[i]
	tree, err := ds.Tree()
	if err != nil {
		dz.message = err.Error()
		return
	}
	dz.design = ds
	dz.tree = tree.Node
	dz.picker = nil
	dz.nameEditing = false
	dz.message = "loaded " + ds.Name
	if dz.treeSize() > c.MaxDecisionTreeSize() {
		// Saved when the limit was higher, or hand-edited. It loads so it
		// can be cut down, but it can't be saved or run until it is.
		dz.message = fmt.Sprintf("%s has %d nodes, over the %d-node limit — trim it to save",
			ds.Name, dz.treeSize(), c.MaxDecisionTreeSize())
	}
}

// Draw paints the screen and records this frame's hit rects.
func (dz *Designer) Draw(screen *ebiten.Image) {
	fillThemeBackground(screen)
	dz.hits = dz.hits[:0]

	title := "ORGANISM DESIGNER"
	text.Draw(screen, title, r.FontSourceCodePro12, designerPad, designerPad, themedForeground())

	top := designerPad + 20
	dz.drawPortraitColumn(screen, designerPad, top)
	midX := designerPad + designerPortraitW + designerColGap
	dz.drawNumbersColumn(screen, midX, top)
	treeX := midX + 300 + designerColGap
	dz.drawTreeColumn(screen, treeX, top, c.ScreenWidth()-treeX-designerPad)

	dz.drawFooter(screen)
	if dz.picker != nil {
		dz.drawPicker(screen)
	}
}

// drawPortraitColumn paints the organism and the two colour rows.
func (dz *Designer) drawPortraitColumn(screen *ebiten.Image, x, y int) {
	box := designerPortraitW
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(box), float64(box),
		chrome(color.RGBA{R: 24, G: 24, B: 32, A: 255}, color.RGBA{R: 235, G: 235, B: 242, A: 255}))
	dz.drawPortrait(screen, x, y, box)

	rowY := y + box + 12
	text.Draw(screen, "NAME", r.FontSourceCodePro10, x, rowY+12, themedForegroundDim())
	nameX, nameW := x+70, box-70
	nameFill := chrome(color.RGBA{R: 40, G: 40, B: 55, A: 255}, color.RGBA{R: 225, G: 225, B: 235, A: 255})
	if dz.nameEditing {
		nameFill = chrome(color.RGBA{R: 55, G: 55, B: 90, A: 255}, color.RGBA{R: 210, G: 215, B: 240, A: 255})
	}
	ebitenutil.DrawRect(screen, float64(nameX), float64(rowY), float64(nameW), float64(designerRowH), nameFill)
	label := dz.design.Name
	if dz.nameEditing {
		label += "_"
	} else if label == "" {
		label = "click to name"
	}
	text.Draw(screen, label, r.FontSourceCodePro10, nameX+6, rowY+15, themedForeground())
	dz.hits = append(dz.hits, designerHit{x: nameX, y: rowY, w: nameW, h: designerRowH, kind: hitName})

	rowY += designerRowH + 10
	rowY = dz.drawSwatchRow(screen, x, rowY, "BODY", dz.design.Color, hitColor)
	rowY = dz.drawSwatchRow(screen, x, rowY, "FEATURES", dz.design.SecondaryColor, hitSecondaryColor)

	// Saved designs, so a design can be reopened without leaving.
	if len(dz.saved) > 0 {
		rowY += 8
		text.Draw(screen, "SAVED", r.FontSourceCodePro10, x, rowY+12, themedForegroundDim())
		rowY += designerRowH
		for i, ds := range dz.saved {
			if rowY > c.ScreenHeight()-100 {
				break
			}
			loadW := designerPortraitW - designerDeleteW - 4
			label := ds.Name
			if ds.ExceedsTreeLimit(c.MaxDecisionTreeSize()) {
				// Named on the row it belongs to: the user finds out when
				// looking at the design, not when a simulation won't start.
				label += fmt.Sprintf("  (%d > %d nodes)", ds.TreeSize(), c.MaxDecisionTreeSize())
			}
			drawMenuButton(screen, x, rowY, loadW, designerRowH, label, false, false,
				ds.ExceedsTreeLimit(c.MaxDecisionTreeSize()))
			dz.hits = append(dz.hits, designerHit{x: x, y: rowY, w: loadW, h: designerRowH,
				kind: hitLoad, index: i})

			// Delete sits apart from the name, and asks twice.
			dx := x + loadW + 4
			mark := "x"
			if dz.confirmDelete == i {
				mark = "!"
			}
			drawMenuButton(screen, dx, rowY, designerDeleteW, designerRowH, mark, false, false, false)
			dz.hits = append(dz.hits, designerHit{x: dx, y: rowY, w: designerDeleteW, h: designerRowH,
				kind: hitDelete, index: i})
			rowY += designerRowH + 4
		}
	}
}

// drawSwatchRow paints one labelled palette row and returns the next y.
func (dz *Designer) drawSwatchRow(screen *ebiten.Image, x, y int, label, current string, kind designerHitKind) int {
	text.Draw(screen, label, r.FontSourceCodePro10, x, y+14, themedForegroundDim())
	sx := x + 70
	for i, col := range designerPalette {
		ebitenutil.DrawRect(screen, float64(sx), float64(y), designerSwatch, designerSwatch, col)
		if strings.EqualFold(col.Hex(), current) {
			// A ring rather than a fill change, so the selected colour is
			// still shown as itself.
			ebitenutil.DrawRect(screen, float64(sx-2), float64(y-2), designerSwatch+4, 2, themedForeground())
			ebitenutil.DrawRect(screen, float64(sx-2), float64(y+designerSwatch), designerSwatch+4, 2, themedForeground())
		}
		dz.hits = append(dz.hits, designerHit{x: sx, y: y, w: designerSwatch, h: designerSwatch, kind: kind, index: i})
		sx += designerSwatch + 2
	}
	return y + designerSwatch + 8
}

// drawPortrait composites the organism the way the grid does, at the
// 16x16 sprites and scaled up. The appearance comes from the scores and
// tree being edited, so the picture answers "what will this look like"
// without the user having to run anything.
//
// The high-res set is selected explicitly: only 16x16 carries the
// layered overlays, and at 4x4 — the set active at startup — every layer
// lookup misses and the portrait falls back to a four-pixel body. The
// grid re-selects from its camera every frame, but the previous set is
// put back anyway rather than leave a global changed from here.
func (dz *Designer) drawPortrait(screen *ebiten.Image, x, y, box int) {
	defer withHighResSprites()()

	primary, err := colorful.Hex(dz.design.Color)
	if err != nil {
		primary = designerPalette[0]
	}
	secondary, err := colorful.Hex(dz.design.SecondaryColor)
	if err != nil {
		secondary = designerPalette[2]
	}

	appearance := physiology.AppearanceFor(dz.scores(), dz.currentTree())
	// The size class the design will actually be, so the portrait shows
	// the sprite the world will draw rather than always the large one.
	role := portraitRole(dz.design.MaxSize)
	facing := utils.Point{X: 0, Y: -1}

	const cell = portraitCellSize
	// Whole-number scale, measured against the tallest thing this role
	// draws: pixel art upscaled by a fraction lands some source pixels on
	// two screen pixels and some on one, which reads as a wobble.
	spriteH := cell
	if base := r.Sprite(role, animation.AnimIdle, 0); base != nil {
		spriteH = float64(base.Bounds().Dy())
	}
	scale := math.Max(1, math.Floor(float64(box)*0.8/spriteH))

	// drawAnimatedSprite anchors the sprite's base cell at (px, py), and
	// a multi-cell sprite extends upward from there, so centring means
	// placing the base cell below the middle by the overhang.
	px := float64(x) + (float64(box)-cell*scale)/2
	py := float64(y) + (float64(box)-spriteH*scale)/2 + (spriteH-cell)*scale

	drew := false
	for _, layer := range r.OrganismLayersFor(appearance) {
		sprite := r.SpriteLayer(role, layer, animation.AnimIdle, 0)
		if sprite == nil {
			continue
		}
		col := secondary
		if r.UsesPrimaryColor(layer) {
			col = primary
		}
		drawAnimatedSprite(screen, px, py, sprite, facing, col, cell, scale)
		drew = true
	}
	if !drew {
		drawAnimatedSprite(screen, px, py, r.Sprite(role, animation.AnimIdle, 0), facing, primary, cell, scale)
	}
}

// portraitCellSize is the native cell of the high-res sprite set the
// portrait draws with.
const portraitCellSize = 16.0

// withHighResSprites switches the active sprite set to the layered
// 16x16 art and returns the function that puts back whatever was
// selected. Used as `defer withHighResSprites()()`.
//
// The selected set is global and the grid re-selects it from its camera
// every frame, so this is restored rather than left changed: a screen
// that draws an organism shouldn't decide what zoom the next one gets.
func withHighResSprites() func() {
	prev := r.CurrentZoom()
	r.SelectZoom(r.ZoomHighRes)
	return func() { r.SelectZoom(prev) }
}

// portraitRole is the sprite role for a design's size, using the same
// size-class split the simulation does.
func portraitRole(maxSize float64) r.ImageRole {
	switch effects.SizeBracket(c.GetCurrentGlobals(), maxSize) {
	case 0:
		return r.RoleOrganismSmall
	case 1:
		return r.RoleOrganismMedium
	default:
		return r.RoleOrganismLarge
	}
}

// drawNumbersColumn paints the trait and ability rows.
func (dz *Designer) drawNumbersColumn(screen *ebiten.Image, x, y int) {
	text.Draw(screen, "TRAITS", r.FontSourceCodePro10, x, y+12, themedForegroundDim())
	y += designerRowH
	for i, t := range designerTraits {
		dz.drawStepRow(screen, x, y, t.label, fmt.Sprintf(t.format, t.get(&dz.design)), i, hitTraitDown, hitTraitUp, false)
		y += designerRowH + 2
	}

	y += 12
	total := dz.abilityTotal()
	header := fmt.Sprintf("ABILITIES   %d / %d", total, physiology.PointTotal)
	headerCol := themedForegroundDim()
	if total != physiology.PointTotal {
		headerCol = themedBad()
	}
	text.Draw(screen, header, r.FontSourceCodePro10, x, y+12, headerCol)
	y += designerRowH
	for _, a := range physiology.AllAbilities {
		score := 0
		if int(a) < len(dz.design.Abilities) {
			score = dz.design.Abilities[a]
		}
		dz.drawStepRow(screen, x, y, a.Name(), fmt.Sprintf("%d", score), int(a), hitAbilityDown, hitAbilityUp, true)
		y += designerRowH + 2
	}
}

// drawStepRow paints "label  - value +" and records the two buttons.
func (dz *Designer) drawStepRow(screen *ebiten.Image, x, y int, label, value string, index int,
	down, up designerHitKind, ability bool) {

	const rowW = 300
	text.Draw(screen, label, r.FontSourceCodePro10, x, y+15, themedForeground())

	bx := x + rowW - designerStepW*2 - 60
	drawMenuButton(screen, bx, y, designerStepW, designerRowH, "-", false, false, false)
	dz.hits = append(dz.hits, designerHit{x: bx, y: y, w: designerStepW, h: designerRowH, kind: down, index: index})

	vx := bx + designerStepW + 6
	valueCol := themedForeground()
	if ability {
		valueCol = gh.AbilityScoreColor(float64(atoiSafe(value)))
	}
	text.Draw(screen, value, r.FontSourceCodePro10, vx, y+15, valueCol)

	ux := bx + designerStepW + 54
	drawMenuButton(screen, ux, y, designerStepW, designerRowH, "+", false, false, false)
	dz.hits = append(dz.hits, designerHit{x: ux, y: y, w: designerStepW, h: designerRowH, kind: up, index: index})
}

// drawTreeColumn paints the decision tree, one clickable row per node.
func (dz *Designer) drawTreeColumn(screen *ebiten.Image, x, y, w int) {
	text.Draw(screen, fmt.Sprintf("DECISION TREE   %d / %d nodes", dz.treeSize(), c.MaxDecisionTreeSize()),
		r.FontSourceCodePro10, x, y+12, themedForegroundDim())
	y += designerRowH
	dz.drawTreeNode(screen, dz.tree, x, &y, w, 0, "")
}

// drawTreeNode paints one node and its branches, depth-first, so the
// shape on screen is the shape of the tree.
func (dz *Designer) drawTreeNode(screen *ebiten.Image, node *d.Node, x int, y *int, w, depth int, branch string) {
	if node == nil || *y > c.ScreenHeight()-110 {
		return
	}
	indent := depth * 16
	label := branch + d.Map[node.NodeType]
	fill := chrome(color.RGBA{R: 44, G: 44, B: 60, A: 255}, color.RGBA{R: 224, G: 224, B: 234, A: 255})
	if node.IsCondition() {
		fill = chrome(color.RGBA{R: 54, G: 54, B: 84, A: 255}, color.RGBA{R: 210, G: 214, B: 238, A: 255})
	}
	rowX, rowW := x+indent, w-indent
	ebitenutil.DrawRect(screen, float64(rowX), float64(*y), float64(rowW), float64(designerRowH), fill)
	text.Draw(screen, label, r.FontSourceCodePro10, rowX+6, *y+15, themedForeground())
	dz.hits = append(dz.hits, designerHit{x: rowX, y: *y, w: rowW, h: designerRowH, kind: hitTreeNode, node: node})
	*y += designerRowH + 3

	if node.IsCondition() {
		dz.drawTreeNode(screen, node.YesNode, x, y, w, depth+1, "yes: ")
		dz.drawTreeNode(screen, node.NoNode, x, y, w, depth+1, "no: ")
	}
}

// drawPicker paints the open dropdown over everything else.
func (dz *Designer) drawPicker(screen *ebiten.Image) {
	const colW = 210
	rows := (len(dz.picker.options) + 1) / 2
	h := rows*designerRowH + 8
	x := min(dz.picker.x, c.ScreenWidth()-colW*2-designerPad)
	y := min(dz.picker.y, c.ScreenHeight()-h-designerPad)

	ebitenutil.DrawRect(screen, float64(x-4), float64(y-4), float64(colW*2+8), float64(h+8),
		chrome(color.RGBA{R: 18, G: 18, B: 26, A: 255}, color.RGBA{R: 245, G: 245, B: 250, A: 255}))
	for i, opt := range dz.picker.options {
		ox := x + (i/rows)*colW
		oy := y + (i%rows)*designerRowH
		fill := chrome(color.RGBA{R: 40, G: 40, B: 52, A: 255}, color.RGBA{R: 228, G: 228, B: 236, A: 255})
		if !opt.isAction {
			fill = chrome(color.RGBA{R: 52, G: 52, B: 78, A: 255}, color.RGBA{R: 214, G: 218, B: 240, A: 255})
		}
		ebitenutil.DrawRect(screen, float64(ox), float64(oy), float64(colW-2), float64(designerRowH-1), fill)
		text.Draw(screen, opt.label, r.FontSourceCodePro10, ox+6, oy+15, themedForeground())
		dz.hits = append(dz.hits, designerHit{x: ox, y: oy, w: colW - 2, h: designerRowH - 1,
			kind: hitPickerOption, index: i})
	}
}

// drawFooter paints the save / new / back buttons and any message.
func (dz *Designer) drawFooter(screen *ebiten.Image) {
	y := c.ScreenHeight() - designerPad - designerBtnH
	x := designerPad

	blocked := dz.saveBlockedReason()
	drawMenuButton(screen, x, y, designerBtnW, designerBtnH, "Save", false, false, blocked != "")
	dz.hits = append(dz.hits, designerHit{x: x, y: y, w: designerBtnW, h: designerBtnH, kind: hitSave})

	x += designerBtnW + 10
	drawMenuButton(screen, x, y, designerBtnW, designerBtnH, "New", false, false, false)
	dz.hits = append(dz.hits, designerHit{x: x, y: y, w: designerBtnW, h: designerBtnH, kind: hitNew})

	x += designerBtnW + 10
	drawMenuButton(screen, x, y, designerBtnW, designerBtnH, "Back", false, false, false)
	dz.hits = append(dz.hits, designerHit{x: x, y: y, w: designerBtnW, h: designerBtnH, kind: hitBack})

	msg, col := dz.message, themedForegroundDim()
	if blocked != "" {
		msg, col = blocked, themedBad()
	}
	if msg != "" {
		text.Draw(screen, msg, r.FontSourceCodePro10, x+designerBtnW+16, y+20, col)
	}
}

// saveBlockedReason explains why the design can't be saved yet, or "".
func (dz *Designer) saveBlockedReason() string {
	if strings.TrimSpace(dz.design.Name) == "" {
		return "name the organism to save it"
	}
	if total := dz.abilityTotal(); total != physiology.PointTotal {
		return fmt.Sprintf("abilities total %d; they must add up to %d", total, physiology.PointTotal)
	}
	if limit := c.MaxDecisionTreeSize(); limit > 0 && dz.treeSize() > limit {
		// Reachable by loading a design saved under a higher limit: the
		// editor won't grow one past it, but it will show one.
		return fmt.Sprintf("tree has %d nodes, over the %d-node limit", dz.treeSize(), limit)
	}
	return ""
}

func atoiSafe(s string) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return n
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
