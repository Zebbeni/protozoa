package physiology

import (
	"math"

	"github.com/Zebbeni/protozoa/config"
)

// CurveID names one of the multiplier curves that turn an ability score into how strongly an action works.
type CurveID int

const (
	CurveChemosynthesis CurveID = iota
	CurveEating
	CurveMovementCost
	CurveDiggingCost
	CurveDiggingStrength
	CurveAttack
	CurveDamageTaken
	// CurveThorns is the damage a defender deals back per hit taken.
	CurveThorns
	// CurvePhTolerance is how wide a pH band an organism tolerates.
	CurvePhTolerance
	// CurveDiggingCreate is the wall strength a dig packs into the cells either side of it.
	CurveDiggingCreate
	// CurveChemoPhEffect and CurveEatingPhEffect scale how hard an action shifts the pH around it, on top of the health it gained.
	CurveChemoPhEffect
	CurveEatingPhEffect
	// CurveEatingCost is what one eating attempt costs, whether or not it finds anything.
	CurveEatingCost
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
	CurveEatingCost,
}

var curveInfo = [curveCount]struct {
	name    string
	ability Ability
	// costStyle curves run from 1 at score 0 to 0 at score 100: a better score makes something cheaper or hurt less.
	costStyle bool
	// defaultShape is the shape used when the configuration doesn't name a valid one.
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
	CurveEatingCost:      {"Eating cost", AbilityEating, true, ShapeCosine},
}

// CurvesFor lists the curves an ability drives, in declaration order.
func CurvesFor(a Ability) []CurveID {
	var out []CurveID
	for _, id := range AllCurves {
		if id.Ability() == a {
			out = append(out, id)
		}
	}
	return out
}

func (id CurveID) Name() string { return curveInfo[id].name }

func (id CurveID) Ability() Ability { return curveInfo[id].ability }

// CostStyle reports whether the curve is fixed at 1 at score 0 (a cost or damage that shrinks as the score rises).
func (id CurveID) CostStyle() bool { return curveInfo[id].costStyle }

type Curve struct {
	Shape Shape
	// Flipped is true for cost curves, which fall from 1 instead of rising.
	Flipped bool
}

type Shape interface {
	Progress(score float64) float64
}

// ShapeKind names one of the shapes a curve can take.
type ShapeKind string

const (
	// ShapeFlat is 1 everywhere: the ability's score doesn't scale this effect at all.
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

// AllShapeKinds lists the shapes in the order the config screen cycles through them, gentlest start first.
var AllShapeKinds = []ShapeKind{ShapeFlat, ShapeSaturating, ShapeLinear, ShapeQuadratic, ShapeCosine}

func (k ShapeKind) Valid() bool {
	for _, known := range AllShapeKinds {
		if k == known {
			return true
		}
	}
	return false
}

func (k ShapeKind) KRange() (float64, float64) {
	switch k {
	case ShapeCosine:
		return 0, 1
	case ShapeSaturating:
		return MinSaturatingK, MaxSaturatingK
	}
	return 0, 0
}

func (k ShapeKind) UsesK() bool {
	lo, hi := k.KRange()
	return lo != hi
}

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

// ShapeFor is the shape curve id takes under configuration g, falling back to the curve's default when the configured name isn't one.
func ShapeFor(g *config.Globals, id CurveID) ShapeKind {
	if kind := ShapeKind(curveShapeName(g, id)); kind.Valid() {
		return kind
	}
	return curveInfo[id].defaultShape
}

func CurveFor(g *config.Globals, id CurveID) Curve {
	return Curve{Shape: ShapeFor(g, id).new(curveK(g, id)), Flipped: id.CostStyle()}
}

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
	case CurveEatingCost:
		return g.EatingCostCurveShape
	}
	return ""
}

func curveK(g *config.Globals, id CurveID) float64 {
	switch ShapeFor(g, id) {
	case ShapeCosine:
		return cosineK(g, id)
	case ShapeSaturating:
		return saturatingK(g, id)
	}
	return 0
}

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
	case CurveEatingCost:
		return g.EatingCostCosineK
	}
	return 0
}

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
	case CurveEatingCost:
		return g.EatingCostSaturatingK
	}
	return 0
}

// At evaluates the curve's multiplier at score: the shape for effect curves, 1 minus the shape for cost curves.
func (cv Curve) At(score float64) float64 {
	if cv.Flipped {
		return 1 - cv.Shape.Progress(score)
	}
	return cv.Shape.Progress(score)
}

// progressAt clamps score to [0, 100] as p = score/100 and returns the exact endpoints, leaving f to shape the scores in between.
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

// CosineShape is a cosine S-curve with extra dampening of early values.
type CosineShape struct{ K float64 }

// newCosineShape holds K to [0, 1], which keeps the curve moving one way.
func newCosineShape(k float64) Shape { return CosineShape{K: min(1, max(0, k))} }

func (c CosineShape) Progress(score float64) float64 {
	return progressAt(score, func(p float64) float64 {
		t := (1 - math.Cos(math.Pi*p)) / 2
		return t - (1-p)*c.K*t
	})
}

type FlatShape struct{}

func (FlatShape) Progress(float64) float64 { return 1 }

type LinearShape struct{}

func (LinearShape) Progress(score float64) float64 {
	return progressAt(score, func(p float64) float64 { return p })
}

// QuadraticShape is p², so the first points buy little and each later one buys more.
type QuadraticShape struct{}

func (QuadraticShape) Progress(score float64) float64 {
	return progressAt(score, func(p float64) float64 { return p * p })
}

type SaturatingShape struct{ K float64 }

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
