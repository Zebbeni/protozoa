package physiology

import (
	"math"

	"github.com/Zebbeni/protozoa/config"
)

// CurveID names one of the multiplier curves that turn an ability score
// into how strongly an action works. Most abilities drive one curve;
// Digging drives three, because a better digger pays less to dig, clears
// more terrain ahead, and packs more into the walls it raises — and
// clearing and raising are worth tuning apart, so a small organism can
// tunnel without being able to build.
type CurveID int

const (
	CurveChemosynthesis CurveID = iota
	CurveEating
	CurveMovementCost
	CurveDiggingCost
	CurveDiggingStrength
	CurveAttack
	CurveDamageTaken
	// CurveThorns is the damage a defender deals back per hit taken. A
	// second Defense curve, like Digging's two: armour both absorbs a hit
	// and answers it, and the two are worth tuning apart.
	CurveThorns
	// CurvePhTolerance is how wide a pH band an organism tolerates.
	CurvePhTolerance
	// CurveDiggingCreate is the wall strength a dig packs into the cells
	// either side of it. Appended rather than slotted beside the other
	// Digging curves: the order here is what config files and saved
	// blocks are keyed on.
	CurveDiggingCreate
	// CurveChemoPhEffect and CurveEatingPhEffect scale how hard an
	// action shifts the pH around it, on top of the health it gained.
	// Separating the push from the gain is what lets a hard specialist
	// change its environment faster than it feeds off it — the two used
	// to be locked together at a fixed ratio, so no amount of
	// specialisation could tip the local chemistry.
	//
	// Appended, like CurveDiggingCreate: the order here is what config
	// files and saved blocks are keyed on.
	CurveChemoPhEffect
	CurveEatingPhEffect
	curveCount
)

// AllCurves lists every curve in declaration order.
var AllCurves = []CurveID{
	CurveChemosynthesis,
	CurveEating,
	CurveMovementCost,
	CurveDiggingCost,
	CurveDiggingStrength,
	CurveAttack,
	CurveDamageTaken,
	CurveThorns,
	CurvePhTolerance,
	CurveDiggingCreate,
	CurveChemoPhEffect,
	CurveEatingPhEffect,
}

var curveInfo = [curveCount]struct {
	name    string
	ability Ability
	// costStyle curves run from 1 at score 0 to 0 at score 100: a better
	// score makes something cheaper or hurt less. Effect curves run from 0
	// at score 0 to 1 at score 100.
	costStyle bool
	// defaultShape is the shape used when the configuration doesn't name
	// a valid one.
	defaultShape ShapeKind
}{
	CurveChemosynthesis:  {"Chemosynthesis", AbilityChemosynthesis, false, ShapeSaturating},
	CurveEating:          {"Eating", AbilityEating, false, ShapeCosine},
	CurveMovementCost:    {"Movement cost", AbilityMovement, true, ShapeCosine},
	CurveDiggingCost:     {"Digging cost", AbilityDigging, true, ShapeCosine},
	CurveDiggingStrength: {"Digging removal", AbilityDigging, false, ShapeCosine},
	CurveAttack:          {"Attack", AbilityAttack, false, ShapeCosine},
	CurveDamageTaken:     {"Damage taken", AbilityDefense, true, ShapeCosine},
	CurveThorns:          {"Thorns", AbilityDefense, false, ShapeCosine},
	CurvePhTolerance:     {"pH tolerance", AbilityTolerance, false, ShapeCosine},
	CurveDiggingCreate:   {"Digging creation", AbilityDigging, false, ShapeCosine},
	CurveChemoPhEffect:   {"Chemosynthesis pH push", AbilityChemosynthesis, false, ShapeFlat},
	CurveEatingPhEffect:  {"Eating pH push", AbilityEating, false, ShapeFlat},
}

// CurvesFor lists the curves an ability drives, in declaration order.
// Digging has three (cost, what a dig clears, what it raises) and
// Defense has two (armour both absorbs a hit and answers it), so the
// config screen groups them under one ability rather than one per curve.
func CurvesFor(a Ability) []CurveID {
	var out []CurveID
	for _, id := range AllCurves {
		if id.Ability() == a {
			out = append(out, id)
		}
	}
	return out
}

// Name is the curve's human-readable name.
func (id CurveID) Name() string { return curveInfo[id].name }

// Ability is the ability whose score drives the curve.
func (id CurveID) Ability() Ability { return curveInfo[id].ability }

// CostStyle reports whether the curve is fixed at 1 at score 0 (a cost or
// damage that shrinks as the score rises) rather than at score 100 (an
// effect that grows with the score).
func (id CurveID) CostStyle() bool { return curveInfo[id].costStyle }

// Curve is one multiplier curve under a particular configuration. Every
// curve runs between 0 and 1 across scores 0 to 100; its Shape decides
// how it gets there. Effect curves are the shape itself (nothing at score
// 0, the full effect at 100). Cost curves are 1 − shape (the full cost at
// score 0, nothing at 100). The health change or damage setting a curve
// scales defines what 1 means.
type Curve struct {
	Shape Shape
	// Flipped is true for cost curves, which fall from 1 instead of rising.
	Flipped bool
}

// Shape is a curve's 0-to-1 progress along scores 0 to 100: exactly 0 at
// score 0 and 1 at score 100, never falling in between.
type Shape interface {
	Progress(score float64) float64
}

// ShapeKind names one of the shapes a curve can take. Which one suits an
// ability is a balance question, not a fixed property of the ability: a
// curve that pays off early makes its ability the obvious first buy, and
// one that only pays off for specialists makes it a commitment.
type ShapeKind string

const (
	// ShapeFlat is 1 everywhere: the ability's score doesn't scale this
	// effect at all. The one shape that isn't a curve, and the way to say
	// a curve exists but isn't wanted — an effect that was a plain
	// constant before a curve was put behind it keeps its old behaviour
	// under this shape, rather than every such addition forcing a
	// rebalance of whatever the constant was tuned to.
	//
	// On a cost curve it reads the other way round, since cost curves are
	// 1 minus the shape: flat means the cost is 0 at every score. Only
	// worth setting on effect curves.
	ShapeFlat ShapeKind = "flat"
	// ShapeLinear is a straight line: every point buys the same amount.
	ShapeLinear ShapeKind = "linear"
	// ShapeQuadratic starts slow and accelerates, rewarding investment.
	ShapeQuadratic ShapeKind = "quadratic"
	// ShapeCosine is an S-curve with K holding back early scores.
	ShapeCosine ShapeKind = "cosine"
	// ShapeSaturating front-loads the gains, with K setting how early.
	ShapeSaturating ShapeKind = "saturating"
)

// AllShapeKinds lists the shapes in the order the config screen cycles
// through them, gentlest start first.
var AllShapeKinds = []ShapeKind{ShapeFlat, ShapeSaturating, ShapeLinear, ShapeQuadratic, ShapeCosine}

// Valid reports whether k names a shape.
func (k ShapeKind) Valid() bool {
	for _, known := range AllShapeKinds {
		if k == known {
			return true
		}
	}
	return false
}

// KRange is the range the shape's K is held to. Linear and quadratic
// ignore K entirely, so theirs is empty.
func (k ShapeKind) KRange() (float64, float64) {
	switch k {
	case ShapeCosine:
		return 0, 1
	case ShapeSaturating:
		return MinSaturatingK, MaxSaturatingK
	}
	return 0, 0
}

// UsesK reports whether the shape reads its K at all.
func (k ShapeKind) UsesK() bool {
	lo, hi := k.KRange()
	return lo != hi
}

// new builds the shape with the given K.
func (k ShapeKind) new(kValue float64) Shape {
	switch k {
	case ShapeFlat:
		return FlatShape{}
	case ShapeLinear:
		return LinearShape{}
	case ShapeQuadratic:
		return QuadraticShape{}
	case ShapeSaturating:
		return newSaturatingShape(kValue)
	default:
		return newCosineShape(kValue)
	}
}

// ShapeFor is the shape curve id takes under configuration g, falling
// back to the curve's default when the configured name isn't one.
func ShapeFor(g *config.Globals, id CurveID) ShapeKind {
	if kind := ShapeKind(curveShapeName(g, id)); kind.Valid() {
		return kind
	}
	return curveInfo[id].defaultShape
}

// CurveFor returns curve id under configuration g.
func CurveFor(g *config.Globals, id CurveID) Curve {
	return Curve{Shape: ShapeFor(g, id).new(curveK(g, id)), Flipped: id.CostStyle()}
}

// curveShapeName reads curve id's configured shape name.
func curveShapeName(g *config.Globals, id CurveID) string {
	switch id {
	case CurveChemosynthesis:
		return g.ChemosynthesisCurveShape
	case CurveEating:
		return g.EatingCurveShape
	case CurveMovementCost:
		return g.MovementCostCurveShape
	case CurveDiggingCost:
		return g.DiggingCostCurveShape
	case CurveDiggingStrength:
		return g.DiggingStrengthCurveShape
	case CurveDiggingCreate:
		return g.DiggingCreationCurveShape
	case CurveAttack:
		return g.AttackCurveShape
	case CurveDamageTaken:
		return g.DamageTakenCurveShape
	case CurveThorns:
		return g.ThornsCurveShape
	case CurvePhTolerance:
		return g.PhToleranceCurveShape
	case CurveChemoPhEffect:
		return g.ChemoPhEffectCurveShape
	case CurveEatingPhEffect:
		return g.EatingPhEffectCurveShape
	}
	return ""
}

// curveK reads the K curve id uses under its current shape. Each shape
// has its own K per curve: the cosine's runs 0-1 and the saturating
// shape's 1-100, so one shared value would have to be mangled every time
// the shape changed, losing whatever the other shape was tuned to.
func curveK(g *config.Globals, id CurveID) float64 {
	switch ShapeFor(g, id) {
	case ShapeCosine:
		return cosineK(g, id)
	case ShapeSaturating:
		return saturatingK(g, id)
	}
	return 0
}

// cosineK reads curve id's cosine K.
func cosineK(g *config.Globals, id CurveID) float64 {
	switch id {
	case CurveChemosynthesis:
		return g.ChemosynthesisCosineK
	case CurveEating:
		return g.EatingCosineK
	case CurveMovementCost:
		return g.MovementCostCosineK
	case CurveDiggingCost:
		return g.DiggingCostCosineK
	case CurveDiggingStrength:
		return g.DiggingStrengthCosineK
	case CurveDiggingCreate:
		return g.DiggingCreationCosineK
	case CurveAttack:
		return g.AttackCosineK
	case CurveDamageTaken:
		return g.DamageTakenCosineK
	case CurveThorns:
		return g.ThornsCosineK
	case CurvePhTolerance:
		return g.PhToleranceCosineK
	case CurveChemoPhEffect:
		return g.ChemoPhEffectCosineK
	case CurveEatingPhEffect:
		return g.EatingPhEffectCosineK
	}
	return 0
}

// saturatingK reads curve id's saturating K.
func saturatingK(g *config.Globals, id CurveID) float64 {
	switch id {
	case CurveChemosynthesis:
		return g.ChemosynthesisSaturatingK
	case CurveEating:
		return g.EatingSaturatingK
	case CurveMovementCost:
		return g.MovementCostSaturatingK
	case CurveDiggingCost:
		return g.DiggingCostSaturatingK
	case CurveDiggingStrength:
		return g.DiggingStrengthSaturatingK
	case CurveDiggingCreate:
		return g.DiggingCreationSaturatingK
	case CurveAttack:
		return g.AttackSaturatingK
	case CurveDamageTaken:
		return g.DamageTakenSaturatingK
	case CurveThorns:
		return g.ThornsSaturatingK
	case CurvePhTolerance:
		return g.PhToleranceSaturatingK
	case CurveChemoPhEffect:
		return g.ChemoPhEffectSaturatingK
	case CurveEatingPhEffect:
		return g.EatingPhEffectSaturatingK
	}
	return 0
}

// At evaluates the curve's multiplier at score: the shape for effect
// curves, 1 minus the shape for cost curves.
func (cv Curve) At(score float64) float64 {
	if cv.Flipped {
		return 1 - cv.Shape.Progress(score)
	}
	return cv.Shape.Progress(score)
}

// progressAt clamps score to [0, 100] as p = score/100 and returns the
// exact endpoints, leaving f to shape the scores in between.
func progressAt(score float64, f func(p float64) float64) float64 {
	p := min(1, max(0, score/MaxAbilityScore))
	switch p {
	case 0:
		return 0
	case 1:
		return 1
	}
	return f(p)
}

// CosineShape is a cosine S-curve with extra dampening of early values,
// so an ability pays off slowly at first and accelerates for organisms
// that truly specialise. With p = score/100:
//
//	T = (1 − cos(π·p)) / 2          slow start, fast middle
//	D = (1 − p) · K · T             dampening, strongest early
//	s = T − D                       0 at score 0, 1 at score 100
//
// K = 0 is a pure cosine; K = 1 suppresses early values the most.
type CosineShape struct{ K float64 }

// newCosineShape holds K to [0, 1], which keeps the curve moving one way.
func newCosineShape(k float64) Shape { return CosineShape{K: min(1, max(0, k))} }

func (c CosineShape) Progress(score float64) float64 {
	return progressAt(score, func(p float64) float64 {
		t := (1 - math.Cos(math.Pi*p)) / 2
		return t - (1-p)*c.K*t
	})
}

// LinearShape is a straight line from 0 to 1: every point of the ability
// buys the same amount, whatever the score already is. Ignores K.
// FlatShape is 1 at every score: the effect doesn't scale with the
// ability at all. Not a curve so much as the absence of one.
type FlatShape struct{}

func (FlatShape) Progress(float64) float64 { return 1 }

type LinearShape struct{}

func (LinearShape) Progress(score float64) float64 {
	return progressAt(score, func(p float64) float64 { return p })
}

// QuadraticShape is p², so the first points buy little and each later one
// buys more: an ability worth committing to rather than dabbling in.
// Ignores K.
type QuadraticShape struct{}

func (QuadraticShape) Progress(score float64) float64 {
	return progressAt(score, func(p float64) float64 { return p * p })
}

// SaturatingShape front-loads the gains: y = 1 − K/(K + x²) with x the
// score, scaled so score 100 reaches exactly 1. Half the benefit arrives
// by score √K, so K = 1 hands over almost everything with the first few
// points and K = 100 by about score 10.
//
// Used for chemosynthesis. On an accelerating curve, feeding yourself paid
// best for full specialists, so lineages kept pouring points into it; a
// saturating one gives most of the gain early and leaves points to spend
// on other abilities.
type SaturatingShape struct{ K float64 }

// Saturating K bounds; see SaturatingShape.
const (
	MinSaturatingK = 1
	MaxSaturatingK = 100
)

func newSaturatingShape(k float64) Shape {
	return SaturatingShape{K: min(MaxSaturatingK, max(MinSaturatingK, k))}
}

func (s SaturatingShape) Progress(score float64) float64 {
	return progressAt(score, func(p float64) float64 {
		x := p * MaxAbilityScore
		atMax := 1 - s.K/(s.K+MaxAbilityScore*MaxAbilityScore)
		return (1 - s.K/(s.K+x*x)) / atMax
	})
}
