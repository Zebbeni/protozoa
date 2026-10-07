package ux

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/utils"
)

// minOrganismAnimationUnitSize is the smallest per-cell unit size at which organism sprite animations play.
const minOrganismAnimationUnitSize = 8

// renderOrganisms fully clears and redraws the organism layer every call.
func (g *Grid) renderOrganisms(organismsImage *ebiten.Image, refresh bool, organismInfo map[int]*organism.Info) {
	organismsImage.Clear()

	// One pass for the AGE view's denominator, rather than a scan of the population per organism.
	g.oldestAlive = 0
	if g.orgColor == orgColorAge {
		for _, info := range organismInfo {
			g.oldestAlive = max(g.oldestAlive, info.Age)
		}
	}

	tint := g.organismTint()
	g.drawOrder = organismDrawOrder(organismInfo, g.drawOrder[:0])
	for _, info := range g.drawOrder {
		g.renderOrganism(info, organismsImage, tint)
	}

	// Dying organisms stay in organismInfo with Status = Dying / Decaying for two cycles before finalizeDeaths removes them.
}

// Draw layers, stamped deepest first: what an organism is DOING decides
// what it belongs under, regardless of its age or id.
const (
	// A body a killer is standing on. It gives up its grid square in the
	// cycle it dies but keeps rendering so its death animation plays.
	drawLayerDying = iota
	// A newborn, whose frame is anchored at the PARENT's cell and reaches
	// forward into its own, so its art covers the parent for that cycle.
	// Drawn over the parent it reads as an organism that walked onto
	// another rather than one being born out of it.
	drawLayerNewborn
	drawLayerNormal
)

// organismDrawLayer is a switch, not two booleans, so an organism born and
// killed in the same cycle has one answer written down: it is a body, which
// is the layer that puts it under whoever is standing on it.
func organismDrawLayer(info *organism.Info) int {
	switch {
	case info.Status == organism.StatusDying:
		return drawLayerDying
	case info.BornThisCycle:
		return drawLayerNewborn
	}
	return drawLayerNormal
}

// organismDrawOrder is the order to stamp organisms in: by layer, deepest
// first. Within a layer the order is Go's map order, which is arbitrary and
// differs per frame.
//
// Ranging the map for everything left the two cases below to that same
// arbitrary order, so a body flickered against its killer and a newborn
// against its parent. The layers are what fix those; what stays unfixed is
// the overlap between two ORDINARY organisms, which the 2-cell (_xl)
// animations create whenever one reaches into the cell ahead of it.
//
// Appends into buf so the per-frame slice is reused.
func organismDrawOrder(organismInfo map[int]*organism.Info, buf []*organism.Info) []*organism.Info {
	for layer := drawLayerDying; layer <= drawLayerNormal; layer++ {
		for _, info := range organismInfo {
			if organismDrawLayer(info) == layer {
				buf = append(buf, info)
			}
		}
	}
	return buf
}

// renderOrganism draws an organism at its animation-interpolated position using the action-specific sprite frame, rotated to face its direction.
func (g *Grid) renderOrganism(info *organism.Info, img *ebiten.Image, tint orgTint) {
	us := float64(g.unitSize())

	maxSize := config.MaximumMaxSize()
	var role resources.ImageRole
	switch {
	case info.Size < maxSize*0.25:
		role = resources.RoleOrganismTiny
	case info.Size < maxSize*0.5:
		role = resources.RoleOrganismSmall
	case info.Size < maxSize*0.75:
		role = resources.RoleOrganismMedium
	default:
		role = resources.RoleOrganismLarge
	}

	// Per-layer colour: by default body uses the primary OrganismColor and overlays (pili / teeth / sensors) use the SecondaryColor so two-tone family identities read at a glance.
	bodyColor := info.Color
	overlayColor := info.SecondaryColor
	if col, flat := tint.bodyColor(info); flat {
		bodyColor, overlayColor = col, col
	}

	// Defaults used when animation state is unavailable or the organism has no frame yet.
	gridX := float64(info.Location.X)
	gridY := float64(info.Location.Y)
	direction := info.Direction
	anim := animation.ForStatus(info.Status)
	frameIdx := 0

	if g.animState != nil {
		// Animation is entirely sprite-based — the spritesheet paints any motion between cells.
		animate := g.animState.AnimatesPosition() && g.unitSize() >= minOrganismAnimationUnitSize
		if frame, ok := g.animState.Frames[info.ID]; ok {
			gridX, gridY = animatedCellPosition(frame)
			direction = frame.Direction
			// ForFrame, not ForAction, so a move whose position didn't change falls through to AnimBlocked instead of drawing the 2-cell travel sprite in place.
			anim = animation.ForFrame(frame)
		}
		if animate {
			frameIdx = g.animState.SpriteFrameIndex(g.Camera.SpriteFrameCount())
		} else if g.animState.Speed >= 4 && g.Camera.SpriteFrameCount() > 1 {
			// At 4x+ speed with multi-frame sprites (currently only the 16x16 set), the wall-clock window per cycle is too short to play a real animation.
			frameIdx = 1
		} else if isMultiCellAnim(anim) {
			// Multi-cell (_xl) sprites depict per-frame detail that matters for visual consistency at cycle boundaries.
			frameIdx = g.Camera.SpriteFrameCount() - 1
			if frameIdx < 0 {
				frameIdx = 0
			}
		}
	}

	drawX, drawY := gridX, gridY

	layers := resources.OrganismLayersFor(info.Appearance)
	stampedAny := false
	for _, layer := range layers {
		sprite := resources.SpriteLayer(role, layer, anim, frameIdx)
		if sprite == nil {
			continue
		}
		col := overlayColor
		if resources.UsesPrimaryColor(layer) {
			col = bodyColor
		}
		g.drawOrganismSprite(img, drawX*us, drawY*us, sprite, direction, col)
		stampedAny = true
	}
	if !stampedAny {
		sprite := resources.Sprite(role, anim, frameIdx)
		g.drawOrganismSprite(img, drawX*us, drawY*us, sprite, direction, bodyColor)
	}
}

// animatedCellPosition returns the organism's grid-unit anchor for the current render frame.
func animatedCellPosition(f animation.Frame) (float64, float64) {
	return float64(f.FromLocation.X), float64(f.FromLocation.Y)
}

// isMultiCellAnim reports whether the animation uses a 2-cell (_xl) spritesheet that extends into the cell ahead of the organism.
func isMultiCellAnim(a animation.Animation) bool {
	switch a {
	case animation.AnimMove, animation.AnimAttack, animation.AnimAttackMove,
		animation.AnimEat, animation.AnimEatFail, animation.AnimDig:
		return true
	}
	return false
}

// drawOrganismSprite draws a sprite rotated to match `direction`, anchored to the organism's base cell, using the grid's current camera zoom.
func (g *Grid) drawOrganismSprite(img *ebiten.Image, x, y float64, spriteImg *ebiten.Image, direction utils.Point, col colorful.Color) {
	cellSize := float64(zoomSpriteSizes[g.Camera.SpriteSet()])
	drawAnimatedSprite(img, x, y, spriteImg, direction, col, cellSize, g.Camera.SpriteScale())
}

func (g *Grid) familyColorFor(id int) colorful.Color {
	selID := g.simulation.GetSelected()
	if selID < 0 {
		return familyUnrelatedColor()
	}
	treesGen := g.simulation.TreesGeneration()
	if !g.familyTint.valid(selID, treesGen) {
		sel := g.simulation.GetTreeNodeByID(selID)
		if sel == nil {
			return familyUnrelatedColor()
		}
		g.familyTint = newFamilyTinter(sel, selID, treesGen)
	}
	// Answered from the memo for every organism seen since the selection changed.
	if k, ok := g.familyTint.cached(id); ok {
		return familyColor(k)
	}
	return familyColor(g.familyTint.kinshipOf(g.simulation.GetTreeNodeByID(id)))
}

// ageFraction is how far through its life an organism is, for the AGE view.
func ageFraction(age, oldestAlive int) float64 {
	span := config.MaxLifespan()
	if span <= 0 {
		span = oldestAlive
	}
	if span <= 0 {
		return 0
	}
	return min(1, float64(age)/float64(span))
}

// sizeFraction is an organism's size against the largest any organism can evolve.
func sizeFraction(size float64) float64 {
	maxSize := config.MaximumMaxSize()
	if maxSize <= 0 {
		return 0
	}
	return min(1, max(0, size/maxSize))
}
