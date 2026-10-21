package simulation

import (
	"os"
	"strconv"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// TestNeuralProbe reports how a world on neural nets actually goes: how many
// seeds survive, how big the brains get and what they cost.
//
// A probe rather than an assertion, because none of these numbers is a
// property worth pinning — they are what a sweep would be read from, and the
// nets ship off precisely because nobody has swept them yet.
//
//	NEURAL_PROBE=1 go test ./simulation/ -run TestNeuralProbe -v -timeout 30m
//
// NEURAL_PROBE_CYCLES and NEURAL_PROBE_SEEDS override the window and the
// sample.
func TestNeuralProbe(t *testing.T) {
	if os.Getenv("NEURAL_PROBE") != "1" {
		t.Skip("set NEURAL_PROBE=1 to measure a neural world")
	}
	cycles := envInt("NEURAL_PROBE_CYCLES", 4000)
	seeds := envInt("NEURAL_PROBE_SEEDS", 8)

	loadDefaultGlobals(t)
	g := config.GetCurrentGlobals()
	g.UseNeuralNets = true
	config.SetGlobals(g)

	survived := 0
	logged := false
	for seed := 1; seed <= seeds; seed++ {
		loadDefaultGlobals(t)
		g := config.GetCurrentGlobals()
		g.UseNeuralNets = true
		if v := os.Getenv("NEURAL_PROBE_UPKEEP"); v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				t.Fatal(err)
			}
			g.HealthChangePerBrainNode = f
		}
		if v := os.Getenv("NEURAL_PROBE_ADD_INPUT"); v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				t.Fatal(err)
			}
			g.MutationWeightAddInput = f
		}
		config.SetGlobals(g)

		if !logged {
			// After the overrides, or the line reports the shipped value for
			// a run that is not using it.
			t.Logf("upkeep=%v width=%d depth=%d add_input=%v remove_input=%v",
				config.HealthChangePerBrainNode(), config.MaxNeuralNetWidth(),
				config.MaxNeuralNetDepth(), config.MutationWeightAddInput(),
				config.MutationWeightRemoveInput())
			logged = true
		}

		sim := NewSimulation(&config.Options{
			IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30,
		})
		died := 0
		for i := 0; i < cycles; i++ {
			sim.Update()
			if sim.OrganismCount() == 0 {
				died = i
				break
			}
		}
		if sim.OrganismCount() == 0 {
			t.Logf("seed %2d: died at cycle %d", seed, died)
			continue
		}
		survived++
		count, _, inputs, outputs, nodes, deepest := netStats(sim)
		t.Logf("seed %2d: %5d organisms, mean %.2f in / %.2f out / %.1f nodes, deepest %d",
			seed, count, float64(inputs)/float64(count), float64(outputs)/float64(count),
			float64(nodes)/float64(count), deepest)
	}
	t.Logf("%d of %d seeds survived %d cycles on neural nets", survived, seeds, cycles)
}

func envInt(name string, fallback int) int {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return fallback
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 {
		return fallback
	}
	return n
}
