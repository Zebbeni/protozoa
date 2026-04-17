// Placeholder sprite sheet generator. Produces one PNG per (role, action)
// combination under resources/images/grid/16x16/.
//
// Each sheet contains animation.BaseFramesPerCycle (4) frames. Most actions
// use a 1-cell (16x16) canvas per frame; "move" uses a 2-cell (32x16)
// canvas so the sprite itself can depict the organism transiting from its
// origin cell to its destination cell — the renderer skips position
// interpolation for move, relying on the sprite to show the journey.
//
// Per-action motion:
//
//	idle    — 1 cell; no shift, sprite sits still.
//	move    — 2 cells; shape interpolates left-cell-center → right-cell-center
//	          across the 4 frames.
//	blocked — 1 cell; shape bumps forward and returns to base (move that
//	          failed because the cell ahead was blocked).
//	turn    — 1 cell; shape orbits one pixel around its center along cardinals.
//	attack  — 1 cell; shape extends progressively forward (lunge) then retracts.
//	eat     — 1 cell; shape chomps forward on alternating frames.
//	chemo   — 1 cell; shape wobbles diagonally (absorbing from the environment).
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
// CanvasW is the sheet's width in cells — 1 for in-place animations, 2 for
// animations like "move" that depict travel across two adjacent cells. Each
// frame is cellSize*CanvasW wide, so the full sheet is that width times
// numFrames.
//
// Shifts is used only when CanvasW == 1 — it supplies per-frame (dx, dy)
// offsets from the centered base shape. For CanvasW == 2 the generator
// ignores Shifts and interpolates the base shape from the left cell center
// to the right cell center across frames (see generateFrames).
type Action struct {
	Name    string
	CanvasW int
	Shifts  [numFrames]Shift
}

var roles = []Role{
	{"small", 4},
	{"medium", 8},
	{"large", 12},
}

var actions = []Action{
	{"idle", 1, [numFrames]Shift{{0, 0}, {0, 0}, {0, 0}, {0, 0}}},
	{"move", 2, [numFrames]Shift{}},
	{"blocked", 1, [numFrames]Shift{{0, 0}, {1, 0}, {1, 0}, {0, 0}}},
	{"turn", 1, [numFrames]Shift{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}},
	{"attack", 1, [numFrames]Shift{{0, 0}, {1, 0}, {2, 0}, {1, 0}}},
	{"eat", 1, [numFrames]Shift{{0, 0}, {1, 0}, {0, 0}, {1, 0}}},
	{"chemo", 1, [numFrames]Shift{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}},
}

func main() {
	outDir := filepath.Join("resources", "images", "grid", "16x16")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}

	black := color.RGBA{0, 0, 0, 255}

	for _, role := range roles {
		for _, action := range actions {
			canvasW := action.CanvasW
			if canvasW < 1 {
				canvasW = 1
			}
			frameW := cellSize * canvasW
			sheet := image.NewRGBA(image.Rect(0, 0, frameW*numFrames, cellSize))
			baseOffset := (cellSize - role.InnerSize) / 2

			for f := 0; f < numFrames; f++ {
				xStart, yStart := frameOrigin(action, f, role.InnerSize, baseOffset)
				frameX := f * frameW

				for y := 0; y < role.InnerSize; y++ {
					for x := 0; x < role.InnerSize; x++ {
						sheet.Set(frameX+xStart+x, yStart+y, black)
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
}

// frameOrigin returns the top-left pixel of the base shape within one frame
// of the sheet. For 1-cell actions the position comes from the action's
// per-frame shift; for 2-cell actions (move) it linearly interpolates from
// the left-cell center to the right-cell center across numFrames.
func frameOrigin(action Action, frame, innerSize, baseOffset int) (int, int) {
	if action.CanvasW <= 1 {
		shift := action.Shifts[frame]
		return clamp(baseOffset+shift.X, 0, cellSize-innerSize),
			clamp(baseOffset+shift.Y, 0, cellSize-innerSize)
	}
	leftCenter := baseOffset
	rightCenter := cellSize + baseOffset
	t := float64(frame) / float64(numFrames-1)
	x := int(math.Round(float64(leftCenter) + t*float64(rightCenter-leftCenter)))
	return x, baseOffset
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
