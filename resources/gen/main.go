// Placeholder sprite sheet generator. Produces one PNG per (role, action)
// combination under resources/images/grid/16x16/.
//
// Each sheet contains animation.BaseFramesPerCycle (4) frames. All sprites
// are authored facing UP (-Y): the base cell — the one the organism is
// currently in — occupies the BOTTOM cellSize x cellSize region of each
// frame, and any extending cell sits above it. The renderer rotates these
// to other facing directions at draw time, so authoring is done once per
// (role, action) with "up" as the canonical orientation.
//
// Colorization is multiplicative (ColorScale), so the generator writes
// white pixels and the renderer tints them with the organism colour.
// Grey pixels become dimmer versions of the same hue — author future
// sheets with grayscale shading to bake in brightness variation.
//
// 1-cell actions produce a 16x16 per-frame canvas; 2-cell actions produce
// a 16x32 canvas (base below, extending above). The renderer skips
// position interpolation for 2-cell actions and relies on the sprite
// itself to depict motion.
//
// Per-action motion:
//
//	idle    — 1 cell; no shift, sprite sits still.
//	move    — 2 cells; shape slides base-cell-center (bottom) → extending-
//	          cell-center (top) across the 4 frames.
//	blocked — 1 cell; shape bumps forward and returns to base (move that
//	          failed because the cell ahead was blocked).
//	turn    — 1 cell; shape orbits one pixel around its center along cardinals.
//	attack  — 2 cells; shape lunges from base cell into extending cell and
//	          partially retreats (the attacker doesn't actually move, so
//	          the sprite depicts the full thrust-and-return).
//	eat     — 1 cell; shape chomps forward on alternating frames.
//	chemo   — 1 cell; shape wobbles diagonally (absorbing from the environment).
//	die     — 1 cell; shape shakes side-to-side (placeholder for a death
//	          animation the user will replace with real art).
//
// Regenerate with:
//
//	go run resources/gen/main.go
//
// (invoke from the project root). The file is excluded from normal builds
// via the `ignore` build tag; it is run manually as a one-shot tool.
//
//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const (
	cellSize  = 16
	numFrames = 4
)

type Role struct {
	Name      string
	InnerSize int
}

type Shift struct{ X, Y int }

// Action describes one animation sheet's layout and per-frame motion.
//
// Cells is the sheet's height in cells — 1 for in-place animations, 2 for
// animations that paint across two vertically-adjacent cells (base at the
// bottom, extending above). Each frame is cellSize wide and cellSize*Cells
// tall, so the full sheet is cellSize*numFrames wide.
//
// Shifts is used when Cells == 1 — per-frame (dx, dy) offsets from the
// centered base shape.
//
// Positions is used when Cells == 2 — each entry is a fraction in [0, 1]
// where 0 = base-cell center (bottom) and 1 = extending-cell center
// (top). The base shape is drawn at the linearly-interpolated y position.
// Lets each 2-cell action pick its own curve (move is monotonic upward,
// attack lunges up and partially settles back).
type Action struct {
	Name      string
	Cells     int
	Shifts    [numFrames]Shift
	Positions [numFrames]float64
}

var roles = []Role{
	{"tiny", 2},
	{"small", 4},
	{"medium", 8},
	{"large", 12},
}

// 1-cell shifts are authored in sprite-local space (Y down). Because the
// default sprite orientation is "facing up", a "forward" shift for an
// action like blocked or eat lives along -Y, not +X.
var actions = []Action{
	{"idle", 1, [numFrames]Shift{{0, 0}, {0, 0}, {0, 0}, {0, 0}}, [numFrames]float64{}},
	{"move", 2, [numFrames]Shift{}, [numFrames]float64{0, 1.0 / 3, 2.0 / 3, 1}},
	{"blocked", 1, [numFrames]Shift{{0, 0}, {0, -1}, {0, -1}, {0, 0}}, [numFrames]float64{}},
	// turn: shape orbits CW around centre (up → right → down → left in sprite-local).
	{"turn", 1, [numFrames]Shift{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}, [numFrames]float64{}},
	// attack: start at base, wind up halfway, full lunge, settle back near base.
	// F3 isn't exactly 0 so there's a small jump to the next cycle's base-cell
	// sprite, traded against a more readable thrust-and-recoil motion.
	{"attack", 2, [numFrames]Shift{}, [numFrames]float64{0, 0.5, 1, 0.3}},
	{"eat", 1, [numFrames]Shift{{0, 0}, {0, -1}, {0, 0}, {0, -1}}, [numFrames]float64{}},
	{"chemo", 1, [numFrames]Shift{{-1, 1}, {1, 1}, {1, -1}, {-1, -1}}, [numFrames]float64{}},
	{"die", 1, [numFrames]Shift{{-1, 0}, {1, 0}, {-1, 0}, {1, 0}}, [numFrames]float64{}},
}

func main() {
	outDir := filepath.Join("resources", "images", "grid", "16x16")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}

	// White-on-transparent: the renderer colorizes sprites via multiplicative
	// ColorScale, so white pixels become the full target colour while greys
	// become dimmer versions of the same hue. Author future sheets with
	// grayscale shading if you want built-in brightness variation.
	white := color.RGBA{255, 255, 255, 255}

	for _, role := range roles {
		for _, action := range actions {
			cells := action.Cells
			if cells < 1 {
				cells = 1
			}
			frameW := cellSize
			frameH := cellSize * cells
			sheet := image.NewRGBA(image.Rect(0, 0, frameW*numFrames, frameH))
			baseOffset := (cellSize - role.InnerSize) / 2

			for f := 0; f < numFrames; f++ {
				xStart, yStart := frameOrigin(action, f, role.InnerSize, baseOffset)
				frameX := f * frameW

				for y := 0; y < role.InnerSize; y++ {
					for x := 0; x < role.InnerSize; x++ {
						sheet.Set(frameX+xStart+x, yStart+y, white)
					}
				}
			}

			out := filepath.Join(outDir, role.Name+"_"+action.Name+".png")
			f, err := os.Create(out)
			if err != nil {
				panic(err)
			}
			if err := png.Encode(f, sheet); err != nil {
				f.Close()
				panic(err)
			}
			f.Close()
			fmt.Println("wrote", out)
		}
	}

	// Clean up obsolete per-direction placeholder PNGs from an earlier
	// experiment — any <role>_<action>_<up|right|down|left>.png files we
	// didn't just write are removed here so the dir stays tidy.
	cleanupPerDirection(outDir)
}

// frameOrigin returns the top-left pixel of the base shape within one frame
// of the sheet.
//
// For 1-cell actions the position comes from the action's per-frame shift
// applied to the centered base square within its 16x16 frame.
//
// For 2-cell actions the frame is 16x32 with base cell on the bottom and
// extending cell on top. Positions[frame] in [0, 1] linearly interpolates
// the shape's Y from base-cell-center (bottom) to extending-cell-center
// (top); X stays centred.
func frameOrigin(action Action, frame, innerSize, baseOffset int) (int, int) {
	if action.Cells <= 1 {
		shift := action.Shifts[frame]
		return clamp(baseOffset+shift.X, 0, cellSize-innerSize),
			clamp(baseOffset+shift.Y, 0, cellSize-innerSize)
	}
	bottomY := cellSize + baseOffset
	topY := baseOffset
	t := action.Positions[frame]
	y := int(math.Round(float64(bottomY) + t*float64(topY-bottomY)))
	return baseOffset, y
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func cleanupPerDirection(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".png") {
			continue
		}
		for _, suffix := range []string{"_up.png", "_right.png", "_down.png", "_left.png"} {
			if strings.HasSuffix(name, suffix) {
				_ = os.Remove(filepath.Join(dir, name))
				fmt.Println("removed", filepath.Join(dir, name))
				break
			}
		}
	}
}
