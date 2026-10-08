package ux

import (
	"fmt"
	d "github.com/Zebbeni/protozoa/decision"
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/lucasb-eyer/go-colorful"
	"golang.org/x/image/font"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

// The colour key: a legend in the bottom-right corner saying what the organism colours currently mean.
const (
	colorKeyPad     = 8
	colorKeyBarH    = 10
	colorKeyGap     = 4
	colorKeyLineH   = 11
	colorKeySamples = 64
)

// colorKey is one mode's legend, in the four parts it is read in.
type colorKey struct {
	// title names the mode, in the panel button's own words.
	title string
	// bars are the scales the mode paints, drawn in order.
	bars []colorBar
	// note is the sentence under the bars: what the measurement means.
	note string
}

type colorBar struct {
	// subtitle sits above the bar and says what the bar is a scale of, with its units.
	subtitle string
	// ramp is the colour at t along the bar, 0 at the left.
	ramp func(t float64) colorful.Color
	// axis labels the bar beneath it: left edge, middle, right edge.
	axis [3]string
}

// phToleranceKeyTop is the health-loss-per-cycle the tolerance bar's right edge stands for.
const phToleranceKeyTop = phToleranceMidpointDamage * 10

func colorKeyFor(m mode, colorAbility physiology.Ability, colorAction d.Action) colorKey {
	switch m {
	case orgColorPhEffect:
		return colorKey{
			title: "Org Color: pH Effect",
			bars: []colorBar{{
				subtitle: "lifetime pH pushed up vs down",
				ramp:     phEffectSpectrumColor,
				axis:     [3]string{fmt.Sprintf("%gx", 1/phEffectMaxRatio), "1x", fmt.Sprintf("%gx", phEffectMaxRatio)},
			}},
			note: "Chemosynthesis lowers the pH around it and eating raises it. This is the ratio over its lifetime",
		}
	case orgColorHealth:
		return colorKey{
			title: "Org Color: Health",
			bars: []colorBar{{
				subtitle: "health / size",
				ramp:     gh.GreenRedColor,
				axis:     [3]string{"0", "0.5", "1"},
			}},
			note: "Organism health relative to its size",
		}
	case orgColorAbility:
		return colorKey{
			// Named for the ability itself rather than "Ability".
			title: "Org Color: " + colorAbility.Name(),
			bars: []colorBar{{
				// The saturation point goes in the subtitle, not on the axis.
				subtitle: fmt.Sprintf("Ability score, full green at %g+", gh.AbilityFullGreenScore),
				// The bar spans the whole 0-10 score range rather than only the part the colour varies over.
				ramp: func(t float64) colorful.Color {
					return gh.AbilityScoreColor(t * float64(physiology.MaxAbilityScore))
				},
				axis: [3]string{
					"0",
					fmt.Sprintf("%d", physiology.MaxAbilityScore/2),
					fmt.Sprintf("%d", physiology.MaxAbilityScore),
				},
			}},
			// The range is on the subtitle and the axis already, so the explanation is only about what the ability does.
			note: abilityKeyNotes[colorAbility],
		}
	case orgColorAction:
		return colorKey{
			// Named for the action, like an ability's key is named for the
			// ability: "Action" alone would make all eight read as one mode.
			title: "Org Color: " + d.Map[colorAction],
			bars: []colorBar{{
				subtitle: "share of the decision tree",
				ramp: func(t float64) colorful.Color {
					// The weight ramp only, to its own ceiling. The brightest
					// green is reserved for acting now and is named in the
					// note rather than drawn, since one bar cannot carry a
					// scale and a separate state at once.
					return gh.GrayGreenColor(t * midActionGreen)
				},
				axis: [3]string{"none", fmt.Sprintf("%.0f%%", actionWeightCeiling*50), fmt.Sprintf("%.0f%%+", actionWeightCeiling*100)},
			}},
			note: "How much of an organism's decision tree is this action, as a share of its action nodes. " +
				"Brightest green is an organism taking the action right now, which is always brighter than any amount of the ramp. " +
				"So a dim organism that flares is one acting on a branch it rarely reaches, and a steady mid-green one is built for the action without currently doing it.",
		}
	case orgColorSuccess:
		return colorKey{
			title: "Org Color: Success",
			bars: []colorBar{{
				subtitle: "Descendant survival time remaining",
				ramp:     gh.GrayGreenColor,
				axis:     [3]string{"0%", "50%", "100%"},
			}},
			note: "Organisms colored by the percent of remaining simulation containing at least one living descendant",
		}
	case orgColorFamily:
		// Two bars, because the colours fan out from the selected organism along more than one axis and one bar can only follow one of them.
		return colorKey{
			title: "Org Color: Family",
			bars: []colorBar{
				{
					subtitle: "generations of descent",
					ramp: func(t float64) colorful.Color {
						return familyColor(kinship{down: int(t*familyFadeGenerations + 0.5), related: true})
					},
					axis: [3]string{
						"selected",
						fmt.Sprintf("%d", familyFadeGenerations/2),
						fmt.Sprintf("%d+", familyFadeGenerations),
					},
				},
				{
					subtitle: "generations to a cousin branch",
					// up 1 throughout, so every sample is a genuine cousin rather than the direct line the bar above already covers.
					ramp: func(t float64) colorful.Color {
						dist := 2 + int(t*float64(familyCousinFade-1)+0.5)
						return familyColor(kinship{up: 1, down: dist - 1, related: true})
					},
					axis: [3]string{"2", fmt.Sprintf("%d", (2+familyCousinFade+1)/2), fmt.Sprintf("%d+", familyCousinFade+1)},
				},
			},
			note: "Descendants run green to blue and ancestors green to yellow. Cousin branches fade through red to the gray everything unrelated wears",
		}
	case orgColorAge:
		// The subtitle says which denominator is in force.
		span := "of max lifespan"
		if c.MaxLifespan() <= 0 {
			span = "of the oldest alive"
		}
		return colorKey{
			title: "Org Color: Age",
			bars: []colorBar{{
				subtitle: "Age " + span,
				ramp:     gh.GrayGreenColor,
				axis:     [3]string{"0%", "50%", "100%"},
			}},
			note: "Organisms colored by how far through their lifespan they are, brightest just before they die of old age",
		}
	case orgColorSize:
		return colorKey{
			title: "Org Color: Size",
			bars: []colorBar{{
				subtitle: fmt.Sprintf("Size (0-%g)", c.MaximumMaxSize()),
				ramp:     gh.GrayGreenColor,
				axis: [3]string{
					"0",
					fmt.Sprintf("%g", c.MaximumMaxSize()/2),
					fmt.Sprintf("%g", c.MaximumMaxSize()),
				},
			}},
			note: "Organisms colored by size against the largest any organism can evolve, so a colour means the same thing between runs",
		}
	case orgColorTolerance:
		return colorKey{
			title: "Org Color: Tolerance",
			bars: []colorBar{{
				subtitle: "health loss per cycle",
				// The grid tints by damage, which has no top end, so the bar walks the tolerance fraction backwards instead.
				ramp: func(t float64) colorful.Color { return gh.GreenRedColor(1 - t) },
				axis: [3]string{
					"0",
					fmt.Sprintf("%g", phToleranceMidpointDamage),
					fmt.Sprintf("%g+", phToleranceKeyTop),
				},
			}},
			note: "Health lost in relation to size due to distance from ideal pH, lessened by pH tolerance",
		}
	}
	// True, and nothing else: its colours are a lineage's inherited identity rather than a measurement.
	return colorKey{}
}

// empty reports a mode that shows no key at all, as against one whose key is merely short.
func (k colorKey) empty() bool { return k.title == "" }

// abilityKeyNotes says in one sentence what each ability actually does, for the second half of the Ability key's explanation.
var abilityKeyNotes = map[physiology.Ability]string{
	physiology.AbilityChemosynthesis: "Chemosynthesis makes health out of the environment and lowers the pH around it. A higher score feeds across a wider band of pH",
	physiology.AbilityEating:         "Eating removes food units from the cell ahead and raises the pH around it. A higher score removes more per attempt",
	physiology.AbilityMovement:       "Movement carries the organism forward and turns it. A higher score makes both cost less health",
	physiology.AbilityDigging:        "Digging creates rock walls beside the organism while removing those ahead, or (if there are none) revealing food",
	physiology.AbilityAttack:         "Attack damages the organism ahead, leaving a corpse of food behind when it kills",
	physiology.AbilityDefense:        "Defense absorbs damage from attacks and returns some of it to the attacker",
	physiology.AbilityTolerance:      "Tolerance widens the band of pH the organism can sit in before the environment costs it health",
}

// keyInnerWidth is the room inside the plate's padding.
func keyInnerWidth(width int) int { return width - colorKeyPad*2 }

// titleLines, subtitleLines and noteLines are the key's blocks of text, wrapped to the width it is being drawn at.
func (k colorKey) titleLines(width int) []string {
	return wrapKeyText(k.title, r.FontSourceCodePro10, keyInnerWidth(width))
}

func (b colorBar) subtitleLines(width int) []string {
	return wrapKeyText(b.subtitle, r.FontSourceCodePro8, keyInnerWidth(width))
}

func (k colorKey) noteLines(width int) []string {
	return wrapKeyText(k.note, r.FontSourceCodePro8, keyInnerWidth(width))
}

func (k colorKey) height(width int) int {
	h := colorKeyPad*2 + len(k.titleLines(width))*colorKeyLineH
	for _, b := range k.bars {
		h += len(b.subtitleLines(width))*colorKeyLineH + colorKeyGap + colorKeyBarH + colorKeyLineH
	}
	return h + len(k.noteLines(width))*colorKeyLineH
}

// wrapKeyText breaks s across as many lines as the plate is wide.
func wrapKeyText(s string, face font.Face, maxW int) []string {
	if s == "" {
		return nil
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if line != "" && boundString(face, candidate).Dx() > maxW {
			lines = append(lines, line)
			line = word
			continue
		}
		line = candidate
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// colorKeyRect is where the key lands for a bottom edge at bottomY, aligned to the same right margin as the minimap.
func colorKeyRect(k colorKey, width, bottomY int) image.Rectangle {
	h := k.height(width)
	x := c.ScreenWidth() - width - minimapPadding
	return image.Rect(x, bottomY-h, x+width, bottomY)
}

// drawColorKey paints the key at the given width, bottom edge at bottomY.
func drawColorKey(screen *ebiten.Image, k colorKey, width, bottomY int) {
	rect := colorKeyRect(k, width, bottomY)
	x, y, h := rect.Min.X, rect.Min.Y, rect.Dy()

	// The same border the minimap sits in, so the two read as one corner.
	ebitenutil.DrawRect(screen, float64(x-minimapBorder), float64(y-minimapBorder),
		float64(width+minimapBorder*2), float64(h+minimapBorder*2),
		color.RGBA{R: 60, G: 60, B: 60, A: 200})
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(width), float64(h),
		colorKeyFill())

	innerX := x + colorKeyPad
	innerW := keyInnerWidth(width)
	ty := y + colorKeyPad + colorKeyLineH - 2
	for i, line := range k.titleLines(width) {
		if i > 0 {
			ty += colorKeyLineH
		}
		text.Draw(screen, line, r.FontSourceCodePro10, innerX, ty, themedValue())
	}

	for _, bar := range k.bars {
		for _, line := range bar.subtitleLines(width) {
			ty += colorKeyLineH
			text.Draw(screen, line, r.FontSourceCodePro8, innerX, ty, themedLabel())
		}

		ty += colorKeyGap
		// Sampled in fixed-width columns rather than per pixel.
		colW := float64(innerW) / colorKeySamples
		for i := 0; i < colorKeySamples; i++ {
			t := float64(i) / (colorKeySamples - 1)
			ebitenutil.DrawRect(screen, float64(innerX)+float64(i)*colW, float64(ty),
				colW+1, colorKeyBarH, colorfulToRGBA(bar.ramp(t)))
		}
		ty += colorKeyBarH + colorKeyLineH - 2

		// The three axis labels share one line under the bar.
		lo, mid, hi := bar.axis[0], bar.axis[1], bar.axis[2]
		text.Draw(screen, lo, r.FontSourceCodePro8, innerX, ty, themedLabel())
		mb := boundString(r.FontSourceCodePro8, mid)
		text.Draw(screen, mid, r.FontSourceCodePro8, innerX+(innerW-mb.Dx())/2, ty, themedLabel())
		hb := boundString(r.FontSourceCodePro8, hi)
		text.Draw(screen, hi, r.FontSourceCodePro8, innerX+innerW-hb.Dx(), ty, themedLabel())
	}

	for _, line := range k.noteLines(width) {
		ty += colorKeyLineH
		text.Draw(screen, line, r.FontSourceCodePro8, innerX, ty, themedMuted())
	}
}

func colorKeyFill() color.RGBA {
	return chrome(
		color.RGBA{A: 255},
		color.RGBA{R: 246, G: 246, B: 249, A: 255},
	)
}

// colorfulToRGBA converts to the 8-bit colour ebiten draws with, clamping rather than wrapping.
func colorfulToRGBA(c colorful.Color) color.RGBA {
	clamp := func(v float64) uint8 {
		return uint8(min(1, max(0, v))*254 + 0.5)
	}
	return color.RGBA{R: clamp(c.R), G: clamp(c.G), B: clamp(c.B), A: 255}
}
