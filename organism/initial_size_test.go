package organism

import (
	"testing"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

func sizeGlobals(t *testing.T, fraction, maxInitial float64) {
	t.Helper()
	spawnGlobals(t, 0.25)
	g := *c.GetCurrentGlobals()
	g.InitialOrganismSizeFraction = fraction
	g.MaximumInitialSize = maxInitial
	c.SetGlobals(&g)
}

// TestFoundersStartAtTheConfiguredFraction: the point of the setting is a
// founder that can reproduce on its first cycle instead of growing for
// hundreds of them, so at a fraction of 1 it starts at its own max size and
// already clears its spawn threshold.
func TestFoundersStartAtTheConfiguredFraction(t *testing.T) {
	sizeGlobals(t, 1, 40)
	rng := simrand.New(1)
	canSpawnAtOnce := 0
	for i := 0; i < 500; i++ {
		traits := newRandomTraits(rng)
		got := initialSize(traits)
		if got != traits.MaxSize {
			t.Fatalf("founder %d starts at %.3f, want its max size %.3f", i, got, traits.MaxSize)
		}
		if got >= traits.MinHealthToSpawn {
			canSpawnAtOnce++
		}
	}
	if canSpawnAtOnce != 500 {
		t.Errorf("only %d of 500 founders could spawn immediately at a full fraction", canSpawnAtOnce)
	}
}

// TestHalfFractionIsHalfMaxSize pins that it is a fraction of MAX SIZE
// rather than of anything else.
func TestHalfFractionIsHalfMaxSize(t *testing.T) {
	sizeGlobals(t, 0.5, 40)
	rng := simrand.New(2)
	for i := 0; i < 200; i++ {
		traits := newRandomTraits(rng)
		want := traits.MaxSize * 0.5
		if want < traits.SpawnHealth {
			continue // floored at spawn health; covered below
		}
		if got := initialSize(traits); got != want {
			t.Fatalf("founder %d starts at %.4f, want %.4f", i, got, want)
		}
	}
}

// TestZeroFractionKeepsTheOldStart: 0 is what a settings file written before
// the setting decodes to, and it has to mean the start founders always had.
func TestZeroFractionKeepsTheOldStart(t *testing.T) {
	sizeGlobals(t, 0, 1)
	rng := simrand.New(3)
	for i := 0; i < 500; i++ {
		traits := newRandomTraits(rng)
		if got := initialSize(traits); got != traits.SpawnHealth {
			t.Fatalf("founder %d starts at %.4f, want its spawn health %.4f",
				i, got, traits.SpawnHealth)
		}
	}
}

// TestFractionNeverStartsAFounderUnderItsSpawnHealth: a founder smaller than
// anything born in the simulation is one that dies before it acts, so the
// fraction can only ever start it bigger.
func TestFractionNeverStartsAFounderUnderItsSpawnHealth(t *testing.T) {
	for _, fraction := range []float64{0.01, 0.1, 0.5} {
		sizeGlobals(t, fraction, 40)
		rng := simrand.New(4)
		for i := 0; i < 500; i++ {
			traits := newRandomTraits(rng)
			if got := initialSize(traits); got < traits.SpawnHealth {
				t.Fatalf("fraction %.2f: founder %d starts at %.4f, under its spawn health %.4f",
					fraction, i, got, traits.SpawnHealth)
			}
		}
	}
}

// TestFractionOverOneIsClamped: the setting is a fraction, so a value past 1
// cannot start an organism over its own max size.
func TestFractionOverOneIsClamped(t *testing.T) {
	sizeGlobals(t, 4, 40)
	rng := simrand.New(5)
	for i := 0; i < 200; i++ {
		traits := newRandomTraits(rng)
		if got := initialSize(traits); got > traits.MaxSize {
			t.Fatalf("founder %d starts at %.3f, past its max size %.3f", i, got, traits.MaxSize)
		}
	}
}
