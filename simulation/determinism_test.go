package simulation

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// fingerprint hashes the state that matters: every organism's identity,
// position, health, size, abilities and status, plus the full pH map.
func fingerprint(sim *Simulation) string {
	h := sha256.New()
	orgs := sim.organismManager.Organisms()
	sort.Slice(orgs, func(i, j int) bool { return orgs[i].ID < orgs[j].ID })
	buf := make([]byte, 8)
	put := func(v uint64) { binary.LittleEndian.PutUint64(buf, v); h.Write(buf) }
	for _, o := range orgs {
		put(uint64(o.ID))
		put(uint64(o.Location.X)); put(uint64(o.Location.Y))
		put(math.Float64bits(o.Health)); put(math.Float64bits(o.Size))
		put(uint64(o.Status))
		for _, v := range o.Traits().Abilities {
			put(uint64(v))
		}
	}
	for _, col := range sim.GetPhMap() {
		for _, v := range col {
			put(math.Float64bits(v))
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// TestFixedSeedIsDeterministic runs the same non-zero seed twice from
// scratch and requires identical state at every checkpoint.
//
// This is the fresh-run-vs-fresh-run check; the replay tests only compare
// a replay against its own recording, so a nondeterminism that affects
// every run equally would slip past them. Two runs in the same process
// are enough to catch map-iteration dependence, because Go randomises
// map order per range statement, not per process.
//
// Note seed 0 is not a fixed seed: it means "pick one from the wall
// clock", so default runs are deliberately different every launch.
func TestFixedSeedIsDeterministic(t *testing.T) {
	loadDefaultGlobals(t)
	// Long enough to get past founding into mutation, attacks, thorns and
	// food spawning; seed 12345 keeps a population alive throughout.
	const cycles = 3000
	for _, seed := range []int{12345} {
		run := func() []string {
			sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
			var prints []string
			for i := 1; i <= cycles; i++ {
				sim.Update()
				if i%250 == 0 {
					prints = append(prints, fmt.Sprintf("c%d pop%d %s", sim.Cycle(), sim.OrganismCount(), fingerprint(sim)))
				}
			}
			return prints
		}
		a, b := run(), run()
		diverged := false
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("seed %d diverged: run A %s  vs  run B %s", seed, a[i], b[i])
				diverged = true
				break
			}
		}
		if !diverged {
			t.Logf("seed %d identical across runs through %d cycles (last: %s)", seed, cycles, a[len(a)-1])
		}
	}
}
