package utils

import "math"

// Vector is a float64 2-D direction-and-magnitude pair, used for the
// environment's flow field ("currents"). Deliberately distinct from
// Point, which is the integer grid-coordinate / cardinal-direction
// type: a Vector is continuous and can point between the cardinals,
// which is what lets many organisms' pushes average into a diagonal
// current.
//
// Magnitude convention: flow vectors are kept in [0, 1]. 0 means
// still water (isotropic diffusion, the pre-currents behaviour) and
// 1 is the strongest current the field represents. ClampUnit enforces
// the ceiling; nothing enforces it implicitly, so callers that
// accumulate pushes must clamp.
type Vector struct {
	X, Y float64
}

// VectorFromPoint converts an integer cardinal direction (one of
// utils.Directions) into the equivalent unit Vector. Non-unit Points
// convert component-wise, which is only meaningful for the cardinals.
func VectorFromPoint(p Point) Vector {
	return Vector{X: float64(p.X), Y: float64(p.Y)}
}

// Add returns the component-wise sum. Used to accumulate one
// organism's push onto the cell's existing flow.
func (v Vector) Add(o Vector) Vector {
	return Vector{X: v.X + o.X, Y: v.Y + o.Y}
}

// Scale returns v multiplied by f. Used for per-cycle decay and for
// scaling a push by its strength config.
func (v Vector) Scale(f float64) Vector {
	return Vector{X: v.X * f, Y: v.Y * f}
}

// Dot returns the scalar product. The diffusion weighting uses it to
// measure how well a neighbour direction lines up with the flow, and
// the IsCurrentAligned condition uses it to compare the local flow
// against an organism's facing.
func (v Vector) Dot(o Vector) float64 {
	return v.X*o.X + v.Y*o.Y
}

// Length returns the magnitude.
func (v Vector) Length() float64 {
	return math.Hypot(v.X, v.Y)
}

// ClampUnit scales v down to magnitude 1 if it is longer, and returns
// it unchanged otherwise. Repeated pushes in the same direction
// therefore saturate rather than growing without bound, which keeps
// the diffusion weights well-behaved.
func (v Vector) ClampUnit() Vector {
	l := v.Length()
	if l <= 1 {
		return v
	}
	return v.Scale(1 / l)
}

// IsZero reports whether the vector is exactly still. Used to skip
// the anisotropic weighting entirely on the (common) still cells, so
// a sim with no Fimbriae organisms pays almost nothing for the flow
// field.
func (v Vector) IsZero() bool {
	return v.X == 0 && v.Y == 0
}
