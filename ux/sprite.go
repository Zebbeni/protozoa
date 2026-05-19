package ux

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/utils"
)

// drawAnimatedSprite draws a sprite rotated to face `direction`, anchored to
// its base cell.
//
// Sprites are authored facing -Y (up) and white-on-transparent (optionally
// with grayscale shading). Colorization is multiplicative via ColorScale:
// each channel of each pixel is multiplied by the target colour. White
// pixels become the full target colour; grey pixels become a dimmer
// version of the same hue (brightness variation in the sheet is
// preserved); black stays black.
//
// Single-cell sprites fill a cellSize x cellSize canvas. Multi-cell
// sprites use a cellSize x (N*cellSize) canvas where the base cell is the
// BOTTOM cellSize-tall region and the extending cell(s) sit above it — so
// the organism travels from bottom to top within the sprite at its
// default (up-facing) orientation.
//
// Positive rotation turns clockwise in ebiten's coordinate system. Up-
// facing (0, -1) maps to 0 rotation; east (1, 0) to +π/2; south (0, 1) to
// π; west (-1, 0) to -π/2. Rotation is centred on the base cell (whose
// centre in sprite-local coords is (cellSize/2, spriteH - cellSize/2)), so
// the base cell stays pinned at (x, y) and any extending cells swing to
// align with the facing direction regardless of how tall the sprite is.
//
// Shared between the main grid renderer, the panel portrait, and the
// standalone animation-test screen so they paint identically.
func drawAnimatedSprite(img *ebiten.Image, x, y float64, spriteImg *ebiten.Image, direction utils.Point, col colorful.Color, cellSize, scale float64) {
	if spriteImg == nil {
		return
	}
	b := spriteImg.Bounds()
	spriteH := float64(b.Dy())

	// Base cell sits at the BOTTOM cellSize x cellSize region for multi-
	// cell (vertical-extending) sprites; for single-cell sprites it's the
	// whole image. The formula below reduces to (cellSize/2, cellSize/2)
	// in the single-cell case.
	anchorX := cellSize / 2
	anchorY := spriteH - cellSize/2

	// Compensate the final translate so the anchor (base-cell centre) lands
	// at world (x + cellSize*scale/2, y + cellSize*scale/2) regardless of
	// sprite height — keeping (x, y) = base-cell top-left.
	compensateY := (cellSize/2 - anchorY) * scale

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-anchorX, -anchorY)
	op.GeoM.Rotate(directionAngle(direction))
	op.GeoM.Translate(anchorX, anchorY)
	if scale != 1 {
		op.GeoM.Scale(scale, scale)
	}
	op.GeoM.Translate(x, y+compensateY)
	op.ColorScale.Scale(float32(col.R), float32(col.G), float32(col.B), 1)
	img.DrawImage(spriteImg, op)
}

// directionAngle converts a cardinal utils.Point direction into the
// rotation angle to apply to an up-facing (-Y) sprite.
//
// atan2(y, x) treats +X as 0; sprites are authored facing -Y, so we offset
// by +π/2 to make (0, -1) = no rotation, (1, 0) = +π/2 CW, etc. Non-
// cardinal points still produce a sensible angle through atan2.
func directionAngle(d utils.Point) float64 {
	if d.X == 0 && d.Y == 0 {
		return 0
	}
	return math.Atan2(float64(d.Y), float64(d.X)) + math.Pi/2
}
