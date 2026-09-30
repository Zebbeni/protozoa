package effects

import (
	"math"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

// TestChemoBandFollowsItsSetting: the band an organism can feed in is the
// setting at a full score, not a fixed 1. Before the setting existed the
// width was the bare curve multiplier, which is normalised to 1 at the top
// of the range, so no curve shape or constant could widen it.
func TestChemoBandFollowsItsSetting(t *testing.T) {
	for _, width := range []float64{0.25, 1, 2, 4.5} {
		g := bandGlobals(t, width)
		got := ChemoWidth(g, physiology.MaxAbilityScore)
		if math.Abs(got-width) > 1e-9 {
			t.Errorf("width %.2f: a full score feeds over %.4f pH, want %.4f", width, got, width)
		}
	}
}

// TestChemoBandIsZeroAtZeroScore: the ability buys the width, so no score
// means no band — an organism breaks even only at exactly its ideal pH.
// Widening the setting must not hand a free band to an organism that has
// not invested, which is what an endpoint pair would have done.
func TestChemoBandIsZeroAtZeroScore(t *testing.T) {
	g := bandGlobals(t, 5)
	if got := ChemoWidth(g, 0); got != 0 {
		t.Errorf("a zero Chemosynthesis score feeds over %.4f pH, want 0", got)
	}
}

// TestChemoGainCrossesZeroAtTheBandEdge ties the width to what it means:
// inside the band an attempt gains, at the edge it breaks even, past it the
// attempt costs health.
func TestChemoGainCrossesZeroAtTheBandEdge(t *testing.T) {
	const size = 10
	for _, width := range []float64{1, 3} {
		g := bandGlobals(t, width)
		score := physiology.MaxAbilityScore
		if in := ChemosynthesisGain(g, score, size, width*0.5); in <= 0 {
			t.Errorf("width %.1f: inside the band the gain is %.4f, want positive", width, in)
		}
		if edge := ChemosynthesisGain(g, score, size, width); math.Abs(edge) > 1e-9 {
			t.Errorf("width %.1f: at the edge the gain is %.4f, want 0", width, edge)
		}
		if out := ChemosynthesisGain(g, score, size, width*1.5); out >= 0 {
			t.Errorf("width %.1f: past the band the gain is %.4f, want a cost", width, out)
		}
	}
}

// TestDefaultChemoBandIsBitIdenticalToTheOldFormula: the shipped 1.0 has to
// reproduce the bare multiplier exactly, not merely closely, or every replay
// diverges. Multiplying by 1.0 is exact in IEEE-754, which is why the
// default is expressed as a multiplier rather than as an added offset.
func TestDefaultChemoBandIsBitIdenticalToTheOldFormula(t *testing.T) {
	g := bandGlobals(t, config.DefaultMaxChemosynthesisPhWidth)
	for score := 0; score <= physiology.MaxAbilityScore; score++ {
		want := Multiplier(g, physiology.CurveChemosynthesis, score)
		if got := ChemoWidth(g, score); got != want {
			t.Errorf("score %d: %v, want exactly %v", score, got, want)
		}
	}
}

// TestAbsentChemoBandIsRepaired: absent JSON decodes a float to 0, and a
// band of 0 would mean nothing could chemosynthesize anywhere but at its
// exact ideal pH — silently starving every settings file and recording
// written before the setting.
func TestAbsentChemoBandIsRepaired(t *testing.T) {
	var g config.Globals
	config.SetGlobals(&g)
	if config.MaxChemosynthesisPhWidth() != config.DefaultMaxChemosynthesisPhWidth {
		t.Errorf("an absent band decoded to %v, want %v",
			config.MaxChemosynthesisPhWidth(), config.DefaultMaxChemosynthesisPhWidth)
	}
}

func bandGlobals(t *testing.T, width float64) *config.Globals {
	t.Helper()
	g := digGlobals(t)
	g.MaxChemosynthesisPhWidth = width
	config.SetGlobals(g)
	return g
}
