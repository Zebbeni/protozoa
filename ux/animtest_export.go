package ux

import (
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/resources"
)

// gifExportDir is the directory (relative to the working dir) the animation-test writes click-exported GIFs into.
const gifExportDir = "gif_exports"

// exportCellGif renders every frame of the clicked matrix cell into an animated GIF, applying the current physiology/colour state.
func (a *AnimationTest) exportCellGif(h cellHit) (string, error) {
	nativeCell := float64(h.nativeCell)
	scale := float64(GridDisplayScale)
	cellPx := nativeCell * scale

	// East direction puts a 2-cell sprite's extension to the right of the base cell.
	canvasW := int(cellPx * 2)
	canvasH := int(cellPx)
	if canvasW < 1 {
		canvasW = 1
	}
	if canvasH < 1 {
		canvasH = 1
	}

	framesInSet := zoomSpriteFrameCounts[h.spriteSet]
	if framesInSet < 1 {
		framesInSet = 1
	}

	// Make sure the global zoom matches the row we're exporting from so SpriteLayer lookups inside drawDemoSprite hit this set's images.
	resources.SelectZoom(h.spriteSet)

	frames := make([]*image.RGBA, framesInSet)
	for f := 0; f < framesInSet; f++ {
		canvas := ebiten.NewImage(canvasW, canvasH)
		a.drawDemoSprite(canvas, 0, 0, h.role, demoDirection, h.anim, f, nativeCell, scale, h.nativeCell)

		pix := make([]byte, canvasW*canvasH*4)
		canvas.ReadPixels(pix)
		frames[f] = &image.RGBA{
			Pix:    pix,
			Stride: canvasW * 4,
			Rect:   image.Rect(0, 0, canvasW, canvasH),
		}
	}

	bbox := unionNonTransparentBounds(frames)
	if bbox.Empty() {
		return "", fmt.Errorf("all frames are fully transparent")
	}

	cropped := make([]*image.RGBA, len(frames))
	for i, f := range frames {
		c := image.NewRGBA(image.Rect(0, 0, bbox.Dx(), bbox.Dy()))
		draw.Draw(c, c.Bounds(), f, bbox.Min, draw.Src)
		cropped[i] = c
	}

	pal := buildGifPalette(cropped)

	perFrameDelay := centisecondsPerFrame(framesInSet)
	palettedFrames := make([]*image.Paletted, len(cropped))
	disposals := make([]byte, len(cropped))
	delays := make([]int, len(cropped))
	for i, rgba := range cropped {
		p := image.NewPaletted(rgba.Bounds(), pal)
		// draw.Src with our exact-match palette is effectively a lookup.
		draw.Draw(p, p.Bounds(), rgba, image.Point{}, draw.Src)
		palettedFrames[i] = p
		delays[i] = perFrameDelay
		// DisposalBackground clears each frame's region back to transparent before the next frame draws.
		disposals[i] = gif.DisposalBackground
	}

	g := &gif.GIF{
		Image:     palettedFrames,
		Delay:     delays,
		Disposal:  disposals,
		LoopCount: 0,
		Config: image.Config{
			ColorModel: pal,
			Width:      bbox.Dx(),
			Height:     bbox.Dy(),
		},
	}

	if err := os.MkdirAll(gifExportDir, 0o755); err != nil {
		return "", err
	}
	fpath := filepath.Join(gifExportDir, gifFilename(h, a.appearance))
	out, err := os.Create(fpath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if err := gif.EncodeAll(out, g); err != nil {
		return "", err
	}
	return fpath, nil
}

// unionNonTransparentBounds returns the smallest rectangle containing every non-transparent pixel across all frames.
func unionNonTransparentBounds(frames []*image.RGBA) image.Rectangle {
	minX, minY := 1<<30, 1<<30
	maxX, maxY := -1, -1
	for _, f := range frames {
		stride := f.Stride
		for y := f.Rect.Min.Y; y < f.Rect.Max.Y; y++ {
			row := f.Pix[y*stride : y*stride+f.Rect.Max.X*4]
			for x := f.Rect.Min.X; x < f.Rect.Max.X; x++ {
				if row[x*4+3] == 0 {
					continue
				}
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < minX || maxY < minY {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// buildGifPalette collects the unique RGBA values across every frame (plus a transparent entry at index 0).
func buildGifPalette(frames []*image.RGBA) color.Palette {
	pal := color.Palette{color.RGBA{0, 0, 0, 0}}
	seen := map[uint32]struct{}{0: {}}
	overflow := false
	for _, f := range frames {
		if overflow {
			break
		}
		for i := 0; i+3 < len(f.Pix); i += 4 {
			r, g, b, a := f.Pix[i], f.Pix[i+1], f.Pix[i+2], f.Pix[i+3]
			if a == 0 {
				continue
			}
			key := uint32(r)<<24 | uint32(g)<<16 | uint32(b)<<8 | uint32(a)
			if _, ok := seen[key]; ok {
				continue
			}
			if len(pal) >= 256 {
				overflow = true
				break
			}
			seen[key] = struct{}{}
			pal = append(pal, color.RGBA{r, g, b, a})
		}
	}
	if overflow {
		pal = color.Palette{color.RGBA{0, 0, 0, 0}}
		pal = append(pal, palette.Plan9[:255]...)
	}
	return pal
}

// centisecondsPerFrame returns the per-frame delay for the GIF, in 1/100 second units (the gif package's delay unit).
func centisecondsPerFrame(framesInSet int) int {
	if framesInSet < 1 {
		framesInSet = 1
	}
	// 100 * BaseFramesPerCycle / (AnimationFPS * framesInSet)
	d := 100 * animation.BaseFramesPerCycle / (animation.AnimationFPS * framesInSet)
	if d < 2 {
		// Most browsers/viewers clamp delays under 2cs to 10cs.
		d = 2
	}
	return d
}

// gifFilename builds a descriptive, filesystem-safe name for the exported GIF.
func gifFilename(h cellHit, app physiology.Appearance) string {
	zoom := fmt.Sprintf("%dx%d", h.nativeCell, h.nativeCell)
	role := strings.ToLower(h.roleLabel)
	anim := strings.ReplaceAll(strings.ToLower(h.animLabel), " ", "_")
	return fmt.Sprintf("%s_%s_%s_%s.gif", zoom, role, anim, appearanceSlug(app))
}

// appearanceSlug renders an appearance as a kebab-case string for the export filename, listing the overlays in render order and skipping the "none" classes.
func appearanceSlug(app physiology.Appearance) string {
	parts := make([]string, 0, 4)
	for _, row := range appearanceRows {
		value := row.get(app)
		label := strings.ToLower(row.options[value])
		if label == "none" {
			continue
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, "-")
}
