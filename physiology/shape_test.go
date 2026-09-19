package physiology

import (
	"math"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// TestEveryShapeRunsZeroToOne: whatever shape a curve takes, it starts at
// nothing, ends at the full value and never falls in between — the
// property the whole multiplier system rests on.
//
// ShapeFlat is the deliberate exception and has its own test below: it is
// 1 everywhere precisely so a score buys nothing, which is the point of
// having it.
func TestEveryShapeRunsZeroToOne(t *testing.T) {
	loadGlobals(t)
	for _, kind := range AllShapeKinds {
		if kind == ShapeFlat {
			continue
		}
		for _, k := range []float64{0, 0.5, 1, 25, 100} {
			shape := kind.new(k)
			if got := shape.Progress(0); got != 0 {
				t.Errorf("%s (K %v) at score 0 = %v, want 0", kind, k, got)
			}
			if got := shape.Progress(PointTotal); got != 1 {
				t.Errorf("%s (K %v) at score %d = %v, want 1", kind, k, PointTotal, got)
			}
			prev := 0.0
			for score := 1; score <= PointTotal; score++ {
				v := shape.Progress(float64(score))
				if v < prev-1e-12 {
					t.Fatalf("%s (K %v) falls between scores %d and %d", kind, k, score-1, score)
				}
				prev = v
			}
		}
	}
}

// TestFlatShapeIgnoresTheScore: ShapeFlat is how a curve says it isn't
// wanted. An effect that was a plain constant before a curve was put
// behind it has to keep exactly its old behaviour under this shape, or
// every such addition forces a rebalance of whatever that constant was
// tuned to.
func TestFlatShapeIgnoresTheScore(t *testing.T) {
	loadGlobals(t)
	shape := ShapeFlat.new(0)
	for score := 0; score <= MaxAbilityScore; score++ {
		if got := shape.Progress(float64(score)); got != 1 {
			t.Errorf("flat at score %d = %v, want 1 at every score", score, got)
		}
	}
	// Out of range too: a score restored from a recording made under
	// another scale can land past the cap.
	if got := shape.Progress(float64(MaxAbilityScore) * 10); got != 1 {
		t.Errorf("flat past the cap = %v, want 1", got)
	}
}

// TestShapesDifferWhereItMatters: the point of the choice is that the
// same score buys different amounts, so the shapes must not collapse onto
// each other in the middle of the range.
func TestShapesDifferWhereItMatters(t *testing.T) {
	// Halfway along the score scale, whatever the scale is.
	half := float64(MaxAbilityScore) / 2
	at50 := map[ShapeKind]float64{}
	for _, kind := range AllShapeKinds {
		at50[kind] = kind.new(0.25).Progress(half)
	}
	if math.Abs(at50[ShapeLinear]-0.5) > 1e-12 {
		t.Errorf("linear at half = %v, want 0.5", at50[ShapeLinear])
	}
	if math.Abs(at50[ShapeQuadratic]-0.25) > 1e-12 {
		t.Errorf("quadratic at half = %v, want 0.25", at50[ShapeQuadratic])
	}
	if !(at50[ShapeQuadratic] < at50[ShapeLinear] && at50[ShapeLinear] < at50[ShapeSaturating]) {
		t.Errorf("halfway values should rise quadratic < linear < saturating, got %v", at50)
	}
}

// TestShapesIgnoringKSaySo: linear and quadratic don't read K, and the
// config screen asks them so it can leave the K slider alone.
func TestShapesIgnoringKSaySo(t *testing.T) {
	for _, kind := range AllShapeKinds {
		usesK := kind.UsesK()
		want := kind == ShapeCosine || kind == ShapeSaturating
		if usesK != want {
			t.Errorf("%s UsesK = %v, want %v", kind, usesK, want)
		}
		if !usesK {
			probe := float64(MaxAbilityScore) * 0.37
			a, b := kind.new(0).Progress(probe), kind.new(100).Progress(probe)
			if a != b {
				t.Errorf("%s changed with K: %v vs %v", kind, a, b)
			}
		}
	}
}

// TestCurveShapeComesFromConfig: each curve takes the shape its setting
// names, and an unknown name falls back to the curve's default rather
// than breaking the curve.
func TestCurveShapeComesFromConfig(t *testing.T) {
	loadGlobals(t)
	g := *config.GetCurrentGlobals()

	g.PhToleranceCurveShape = string(ShapeQuadratic)
	if got := ShapeFor(&g, CurvePhTolerance); got != ShapeQuadratic {
		t.Errorf("configured shape = %v, want quadratic", got)
	}
	if got := CurveFor(&g, CurvePhTolerance).At(float64(MaxAbilityScore) / 2); math.Abs(got-0.25) > 1e-12 {
		t.Errorf("quadratic pH tolerance at half = %v, want 0.25", got)
	}

	g.PhToleranceCurveShape = "banana"
	if got := ShapeFor(&g, CurvePhTolerance); got != ShapeCosine {
		t.Errorf("unknown shape fell back to %v, want the curve's default", got)
	}

	// A cost curve takes the same shape, mirrored.
	g.MovementCostCurveShape = string(ShapeLinear)
	if got := CurveFor(&g, CurveMovementCost).At(float64(MaxAbilityScore) / 4); math.Abs(got-0.75) > 1e-12 {
		t.Errorf("linear movement cost at a quarter = %v, want 0.75", got)
	}
}
