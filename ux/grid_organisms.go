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

// minOrganismAnimationUnitSize is the smallest per-cell unit size at which
// organism sprite animations play. Below this (Zoom4), the renderer pins
// to frame 0 of the sheet — at tiny sizes per-frame differences are too
// small to read and the flicker adds more noise than animation.
const minOrganismAnimationUnitSize = 8

// renderOrganisms fully clears and redraws the organism layer every call.
//
// Unlike the other layers (walls, food, env), organisms are animated: we
// need a fresh draw every render tick so sprites at interpolated positions
// don't leave trails and so the animation frame index can advance. Clearing
// unconditionally also means "attack/move animations passing through a
// neighbour cell" requires no special bookkeeping — both cells are empty
// on the organism layer each frame, and food/walls below come from their
// own layers.
func (g *Grid) renderOrganisms(organismsImage *ebiten.Image, refresh bool, organismInfo map[int]*organism.Info) {
	organismsImage.Clear()

	for _, info := range organismInfo {
		g.renderOrganism(info, organismsImage)
	}

	// Dying organisms stay in organismInfo with Status = Dying / Decaying
	// for two cycles before finalizeDeaths removes them, so the renderer
	// doesn't need a separate pass to synthesize dying frames for organisms
	// that vanished from the manager.
}

// renderOrganism draws an organism at its animation-interpolated position
// using the action-specific sprite frame, rotated to face its direction.
//
// If we have an animation.Frame for this organism we use its FromLocation →
// ToLocation pair plus the current Progress() to animate. Without one (newly
// born organism, fresh seek, first frame before any cycle advance) we fall
// back to the organism's static Location.
func (g *Grid) renderOrganism(info *organism.Info, img *ebiten.Image) {
	us := float64(g.unitSize())

	// Size-to-role mapping: thirds of MaximumMaxSize.
	//   small  — below 33%
	//   medium — 33% to below 66%
	//   large  — 66% and above
	maxSize := config.MaximumMaxSize()
	var role resources.ImageRole
	switch {
	case info.Size < maxSize*(1.0/3.0):
		role = resources.RoleOrganismSmall
	case info.Size < maxSize*(2.0/3.0):
		role = resources.RoleOrganismMedium
	default:
		role = resources.RoleOrganismLarge
	}

	// Per-layer colour: by default body uses the primary OrganismColor
	// and overlays (flagellae / teeth / sensors) use the SecondaryColor
	// so two-tone family identities read at a glance. View-mode
	// overrides (pH effect, health) are diagnostic views that should
	// paint the whole organism uniformly — they collapse secondary to
	// the same derived colour as primary.
	bodyColor := info.Color
	overlayColor := info.SecondaryColor
	switch g.orgColor {
	case orgColorPhEffect:
		bodyColor = phEffectColor(info.PhPositive, info.PhNegative)
		overlayColor = bodyColor
	case orgColorHealth:
		bodyColor = healthColor(info.Health, info.Size)
		overlayColor = bodyColor
	}

	// Defaults used when animation state is unavailable or the organism has
	// no frame yet: render statically at the current location facing its
	// current direction.
	gridX := float64(info.Location.X)
	gridY := float64(info.Location.Y)
	direction := info.Direction
	anim := animation.ForStatus(info.Status)
	frameIdx := 0

	if g.animState != nil {
		// Animation is entirely sprite-based — the spritesheet paints any
		// motion between cells. We only gate sprite-frame advancement on
		// playback speed (skip above 2x) and on-screen unit size (skip
		// when sprites are too small to read per-frame differences). When
		// either says "don't animate" we snap to a single frame instead
		// of cycling.
		animate := g.animState.AnimatesPosition() && g.unitSize() >= minOrganismAnimationUnitSize
		if frame, ok := g.animState.Frames[info.ID]; ok {
			gridX, gridY = animatedCellPosition(frame)
			direction = frame.Direction
			// ForFrame, not ForAction, so a move whose position didn't
			// change falls through to AnimBlocked instead of drawing the
			// 2-cell travel sprite in place.
			anim = animation.ForFrame(frame)
		}
		if animate {
			frameIdx = g.animState.SpriteFrameIndex(g.Camera.SpriteFrameCount())
		} else if g.animState.Speed >= 4 && g.Camera.SpriteFrameCount() > 1 {
			// At 4x+ speed with multi-frame sprites (currently only the
			// 16x16 set), the wall-clock window per cycle is too short
			// to play a real animation — pin every organism to frame 1
			// so the action reads as a recognisable mid-action pose
			// instead of either the start state (frame 0) or a frozen
			// end state.
			frameIdx = 1
		} else if isMultiCellAnim(anim) {
			// Multi-cell (_xl) sprites depict per-frame detail that
			// matters for visual consistency at cycle boundaries —
			// move/attack's travel path, eat's crumbs in the adjacent
			// cell, etc. Without frame advancement we'd stay on frame 0
			// for the whole cycle, which typically depicts a mid-action
			// state and snaps backwards when the next cycle begins.
			// Snap to the last frame instead — the authored end state.
			frameIdx = g.Camera.SpriteFrameCount() - 1
			if frameIdx < 0 {
				frameIdx = 0
			}
		}
	}

	// High-res sprite sets are layered: one body variant + a feature
	// overlay per non-defense tree, picked from the organism's
	// physiology. Low-res sets have a single LayerBody layer per role,
	// so OrganismLayersFor's body-variant choice naturally collapses
	// to a single absent-LayerBody lookup that misses and falls back
	// to the default sprite via the Sprite fallback path below.
	layers := resources.OrganismLayersFor(info.Features)
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
		g.drawOrganismSprite(img, gridX*us, gridY*us, sprite, direction, col)
		stampedAny = true
	}
	if !stampedAny {
		// Low-res path (only LayerBody is authored, so the layered
		// lookup above finds nothing) and the missing-art fallback
		// for high-res organisms with no LayerBodyBasic PNG. Both
		// represent "the body of the organism" so they use the
		// primary colour, not secondary.
		sprite := resources.Sprite(role, anim, frameIdx)
		g.drawOrganismSprite(img, gridX*us, gridY*us, sprite, direction, bodyColor)
	}
}

// animatedCellPosition returns the organism's grid-unit anchor for the
// current render frame. All motion is painted by the spritesheet itself,
// so we just anchor at FromLocation — the sprite's base-cell origin.
//
// For 2-cell actions (move, attack, eat) this puts the base cell at the
// source and the extending cell in the direction the organism is
// facing; single-cell actions have FromLocation == ToLocation so the
// anchor choice doesn't matter.
func animatedCellPosition(f animation.Frame) (float64, float64) {
	return float64(f.FromLocation.X), float64(f.FromLocation.Y)
}

// isMultiCellAnim reports whether the animation uses a 2-cell (_xl)
// spritesheet that extends into the cell ahead of the organism. Multi-
// cell sprites need frame-index handling that differs from 1-cell
// sprites: without frame advancement we snap to the last frame so the
// authored end state (shape at top of the 2-cell canvas) is what shows
// for the whole cycle, not the pre-action start state.
func isMultiCellAnim(a animation.Animation) bool {
	switch a {
	case animation.AnimMove, animation.AnimAttack, animation.AnimEat, animation.AnimEatFail:
		return true
	}
	return false
}

// drawOrganismSprite draws a sprite rotated to match `direction`, anchored
// to the organism's base cell, using the grid's current camera zoom.
func (g *Grid) drawOrganismSprite(img *ebiten.Image, x, y float64, spriteImg *ebiten.Image, direction utils.Point, col colorful.Color) {
	cellSize := float64(zoomSpriteSizes[g.Camera.SpriteSet()])
	drawAnimatedSprite(img, x, y, spriteImg, direction, col, cellSize, g.Camera.SpriteScale())
}
