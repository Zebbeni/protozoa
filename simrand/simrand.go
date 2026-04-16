package simrand

import "math/rand/v2"

// RNG is the simulation's random number generator, wrapping a PCG source
// whose state can be serialized for checkpointing.
type RNG struct {
	src *rand.PCG
	r   *rand.Rand
}

// New creates a new RNG seeded with the given value.
func New(seed uint64) *RNG {
	src := rand.NewPCG(seed, 0)
	return &RNG{src: src, r: rand.New(src)}
}

func (rng *RNG) Float64() float64 { return rng.r.Float64() }
func (rng *RNG) Intn(n int) int   { return rng.r.IntN(n) }

// MarshalState returns the serialized RNG state for checkpointing.
func (rng *RNG) MarshalState() ([]byte, error) {
	return rng.src.MarshalBinary()
}

// RestoreFromState creates an RNG restored from a previously serialized state.
func RestoreFromState(data []byte) (*RNG, error) {
	src := rand.NewPCG(0, 0)
	if err := src.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	return &RNG{src: src, r: rand.New(src)}, nil
}
