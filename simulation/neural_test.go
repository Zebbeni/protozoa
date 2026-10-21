package simulation

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
)

// neuralGlobals is the shipped settings with the nets switched on, so a test
// about the nets sets the thing it is about and leaves the balance alone.
func neuralGlobals(t *testing.T) *config.Globals {
	t.Helper()
	loadDefaultGlobals(t)
	g := config.GetCurrentGlobals()
	g.UseNeuralNets = true
	config.SetGlobals(g)
	return config.GetCurrentGlobals()
}

// netStats walks the living population and reports what their brains look
// like.
func netStats(sim *Simulation) (count, withNet, inputs, outputs, nodes, deepest int) {
	for _, o := range sim.organismManager.Organisms() {
		count++
		net := o.Net()
		if net == nil {
			continue
		}
		withNet++
		inputs += len(net.Inputs())
		outputs += len(net.Outputs())
		nodes += net.Nodes()
		if d := net.Depth(); d > deepest {
			deepest = d
		}
	}
	return
}

// TestANeuralWorldRunsAndItsBrainsGrow is the end-to-end claim: switching the
// setting on gives a world whose organisms all run on nets, and whose nets
// gain inputs and outputs over generations rather than staying at the one
// output every founder starts with.
//
// It walks seeds until one survives rather than naming a seed that does,
// because survival on nets is about half of seeds at the shipped upkeep and a
// test that named one would be asserting the balance — it would start failing
// on any change to the defaults, for a reason that has nothing to do with
// whether brains grow. Which seed it lands on is logged.
func TestANeuralWorldRunsAndItsBrainsGrow(t *testing.T) {
	// Long enough for lineages to have mutated many times over, short enough
	// that it is not also a survival test — how long a neural world lasts is
	// what TestNeuralProbe measures, deliberately without asserting anything.
	const cycles = 2000
	var sim *Simulation
	seed := 0
	for try := 1; try <= 8 && sim == nil; try++ {
		neuralGlobals(t)
		candidate := NewSimulation(&config.Options{
			IsHeadless: true, Seed: try, CheckpointInterval: 1 << 30,
		})
		for i := 0; i < cycles && candidate.OrganismCount() > 0; i++ {
			candidate.Update()
		}
		if candidate.OrganismCount() > 0 {
			sim, seed = candidate, try
		}
	}
	if sim == nil {
		t.Fatalf("no seed in 1..8 kept a neural world alive for %d cycles", cycles)
	}
	t.Logf("seed %d survived %d cycles", seed, cycles)

	count, withNet, inputs, outputs, nodes, deepest := netStats(sim)
	if withNet != count {
		t.Errorf("%d of %d organisms run on a net; the setting is meant to decide the whole world",
			withNet, count)
	}
	t.Logf("%d organisms, mean %.2f inputs / %.2f outputs / %.1f nodes, deepest %d hidden layers",
		count, float64(inputs)/float64(count), float64(outputs)/float64(count),
		float64(nodes)/float64(count), deepest)

	if outputs <= count {
		t.Errorf("after %d cycles the population still averages one output per organism; "+
			"nothing has evolved a choice to make (%d outputs over %d organisms)",
			cycles, outputs, count)
	}
	if inputs == 0 {
		t.Errorf("after %d cycles no organism reads any condition at all", cycles)
	}
}

// TestNeuralOrganismsTakeMoreThanOneAction: a net with one output chooses that
// output whatever it reads, so a world where nothing ever evolved a second one
// would be a world of founders idling in place — which would pass the growth
// test above on paper and be a dead simulation in fact.
func TestNeuralOrganismsTakeMoreThanOneAction(t *testing.T) {
	neuralGlobals(t)
	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 6, CheckpointInterval: 1 << 30})

	seen := map[int]int{}
	for i := 0; i < 4000 && sim.OrganismCount() > 0; i++ {
		sim.Update()
		for _, o := range sim.organismManager.Organisms() {
			seen[int(o.Action())]++
		}
	}
	if len(seen) < 3 {
		t.Errorf("the whole population only ever took %d distinct actions in 4000 cycles", len(seen))
	}
	t.Logf("%d distinct actions taken across the run", len(seen))
}

// TestTheNeuralWorldIsDeterministic is the property every recording rests on,
// stated for the brain that was just added: two runs of one seed agree on the
// population, on what each organism is doing and on the brain it is doing it
// with.
func TestTheNeuralWorldIsDeterministic(t *testing.T) {
	neuralGlobals(t)

	// Keyed by id, because Organisms() ranges a map and hands its results
	// back in whatever order that gave — comparing two of those slices
	// position by position reports a difference between two identical worlds.
	run := func() map[int]string {
		sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 5, CheckpointInterval: 1 << 30})
		for i := 0; i < 1200 && sim.OrganismCount() > 0; i++ {
			sim.Update()
		}
		out := map[int]string{}
		for _, o := range sim.organismManager.Organisms() {
			net := o.Net()
			if net == nil {
				out[o.ID] = "no net"
				continue
			}
			out[o.ID] = string(net.Encode())
		}
		return out
	}

	first, second := run(), run()
	if len(first) != len(second) {
		t.Fatalf("two runs of seed 5 ended with %d and %d organisms", len(first), len(second))
	}
	if len(first) == 0 {
		t.Fatal("the world died before the comparison could say anything")
	}
	for id, brain := range first {
		other, ok := second[id]
		if !ok {
			t.Fatalf("organism %d is alive in one run of seed 5 and not the other", id)
		}
		if brain != other {
			t.Fatalf("organism %d has a different brain between two runs of the same seed", id)
		}
	}
	t.Logf("%d organisms identical across two runs", len(first))
}

// TestTheBrainUpkeepIsChargedPerNode pins the cost to the thing it is named
// for: a bigger brain costs more, and a world on decision trees is charged
// nothing at all.
func TestTheBrainUpkeepIsChargedPerNode(t *testing.T) {
	g := neuralGlobals(t)
	g.HealthChangePerBrainNode = -0.01
	// Growth off, so the size the charge was computed against is the size
	// read back afterwards. With growth on, an organism that ate well can be
	// several times bigger by the time the ledger is inspected, and dividing
	// the charge back out reports a fraction of the setting — which is the
	// organism growing, not the formula being wrong.
	g.GrowthFactor = 0
	g.EatingGrowthFactor = 0
	config.SetGlobals(g)

	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 13, CheckpointInterval: 1 << 30})
	for i := 0; i < 500 && sim.OrganismCount() > 0; i++ {
		sim.Update()
	}

	charged, compared := 0, 0
	for _, o := range sim.organismManager.Organisms() {
		l := o.HealthLedger()
		if !l.Recorded {
			continue
		}
		amount := l.Amounts[organism.HealthFromBrain]
		if amount == 0 {
			continue
		}
		charged++
		if amount > 0 {
			t.Errorf("organism %d was PAID %v for running a brain", o.ID, amount)
		}
		// With growth off this is exact: the charge is nodes times size times
		// the setting, so dividing it back out gives the setting again.
		perNodePerSize := amount / (float64(o.BrainNodes()) * o.Size)
		if diff := perNodePerSize - g.HealthChangePerBrainNode; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("organism %d (%d nodes, size %v) was charged %v per node per unit of size, the setting says %v",
				o.ID, o.BrainNodes(), o.Size, perNodePerSize, g.HealthChangePerBrainNode)
		}
		compared++
		if compared > 50 {
			break
		}
	}
	if charged == 0 {
		t.Fatal("nothing was charged for its brain in 500 cycles of a neural world")
	}

	// The same world on decision trees pays nothing, which is what makes the
	// setting inert outside a neural run. The brain is set here rather than
	// taken from the shipped defaults, which are a neural world: this arm is
	// about what a TREE organism is charged, whichever way the default points.
	loadDefaultGlobals(t)
	trees := config.GetCurrentGlobals()
	trees.UseNeuralNets = false
	trees.HealthChangePerBrainNode = -0.01
	config.SetGlobals(trees)
	treeSim := NewSimulation(&config.Options{IsHeadless: true, Seed: 13, CheckpointInterval: 1 << 30})
	for i := 0; i < 200 && treeSim.OrganismCount() > 0; i++ {
		treeSim.Update()
	}
	for _, o := range treeSim.organismManager.Organisms() {
		if amount := o.HealthLedger().Amounts[organism.HealthFromBrain]; amount != 0 {
			t.Fatalf("an organism running on a decision tree was charged %v for a brain", amount)
		}
	}
}

// TestSwitchingBrainsLeavesTheTreeWorldAlone: a world with the setting OFF
// has to be the world before any of this existed — no organism grows a net,
// and none is charged for brain nodes it does not have.
//
// It used to assert that the shipped default was off as well. That is now a
// balance decision rather than a property: the defaults ship a neural world,
// and a test that fails when the shipped value changes is testing the
// settings file rather than the mechanism. The flag is set here instead, so
// what is pinned is what switching it off MEANS.
func TestSwitchingBrainsLeavesTheTreeWorldAlone(t *testing.T) {
	loadDefaultGlobals(t)
	trees := config.GetCurrentGlobals()
	trees.UseNeuralNets = false
	config.SetGlobals(trees)

	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 21, CheckpointInterval: 1 << 30})
	for i := 0; i < 600 && sim.OrganismCount() > 0; i++ {
		sim.Update()
	}
	for _, o := range sim.organismManager.Organisms() {
		if o.Net() != nil {
			t.Fatal("an organism grew a neural net in a world that runs on decision trees")
		}
		if o.BrainNodes() != 0 {
			t.Fatalf("a decision-tree organism reports %d brain nodes", o.BrainNodes())
		}
	}
}

// TestANeuralWorldGrowsSensorOverlays: the sensor overlay follows how hard a
// brain leans on each read, which for a net is where its weight is. The
// property worth pinning is that the threshold is REACHABLE — this file
// already records an appearance threshold shipped so high that 11.9M
// organism-frames produced 406 shells and no spikes at all, and the overlay a
// net grows was in the same state: counting distinct inputs, no organism
// measured ever grew tasters.
//
// It is a reachability guard and NOT a regression test for the weighting:
// counting distinct inputs also clears it, since antennae and feelers both
// appeared under that rule. SENSOR_PROBE=1 is what measures the difference
// between the two rules.
//
// No seed is named, for the reason the test above gives: about half of neural
// seeds die, and a test naming one would fail on any change to the defaults
// for a reason that has nothing to do with sensors.
func TestANeuralWorldGrowsSensorOverlays(t *testing.T) {
	const cycles = 1500
	seen := map[physiology.SensorClass]int{}
	seed := 0
	for try := 1; try <= 8 && seed == 0; try++ {
		neuralGlobals(t)
		sim := NewSimulation(&config.Options{
			IsHeadless: true, Seed: try, CheckpointInterval: 1 << 30,
		})
		for i := 0; i < cycles && sim.OrganismCount() > 0; i++ {
			sim.Update()
		}
		if sim.OrganismCount() == 0 {
			continue
		}
		seed = try
		for _, o := range sim.organismManager.Organisms() {
			seen[o.Appearance().Sensor]++
		}
	}
	if seed == 0 {
		t.Fatalf("no seed in 1..8 kept a neural world alive for %d cycles", cycles)
	}
	t.Logf("seed %d: sensors %v", seed, seen)

	if len(seen) < 2 {
		t.Errorf("every organism wears the same sensor overlay (%v); the threshold is likely unreachable", seen)
	}
	worn := 0
	for class, n := range seen {
		if class != physiology.SensorNone {
			worn += n
		}
	}
	if worn == 0 {
		t.Error("no organism in the world grew any sensor at all")
	}
}
