package simulation

import (
	"os"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

// TestSensorProbe reports what a live neural world is wearing, under the rule
// that weighs each read by how hard the brain leans on it against the rule
// that only counts the reads. Asserts nothing: which overlay a lineage ends up
// with is a balance outcome, not a property.
//
//	SENSOR_PROBE=1 SENSOR_PROBE_CYCLES=2000 go test ./simulation/ -run TestSensorProbe -v -timeout 30m
func TestSensorProbe(t *testing.T) {
	if os.Getenv("SENSOR_PROBE") != "1" {
		t.Skip("set SENSOR_PROBE=1 to measure what a neural world is wearing")
	}
	cycles := envInt("SENSOR_PROBE_CYCLES", 2000)
	seeds := envInt("SENSOR_PROBE_SEEDS", 4)

	names := map[physiology.SensorClass]string{
		physiology.SensorNone:     "none",
		physiology.SensorAntennae: "antennae",
		physiology.SensorFeelers:  "feelers",
		physiology.SensorTasters:  "tasters",
	}
	order := []physiology.SensorClass{
		physiology.SensorNone, physiology.SensorAntennae,
		physiology.SensorFeelers, physiology.SensorTasters,
	}

	var weighted, counted [4]int
	organisms, disagreed, inputs := 0, 0, 0
	for seed := 1; seed <= seeds; seed++ {
		loadDefaultGlobals(t)
		g := config.GetCurrentGlobals()
		g.UseNeuralNets = true
		g.Seed = seed
		config.SetGlobals(g)

		sim := NewSimulation(&config.Options{Seed: seed, CheckpointInterval: 1 << 30})
		for i := 0; i < cycles && sim.OrganismCount() > 0; i++ {
			sim.Update()
		}
		scores := physiology.Scores{8, 2, 2, 2, 2, 2, 2}
		for id := range sim.GetAllOrganismInfo() {
			net := sim.GetOrganismNeuralNetByID(id)
			if net == nil {
				continue
			}
			senses := make([]physiology.SensedCondition, len(net.Inputs()))
			plain := make([]physiology.SensedCondition, len(net.Inputs()))
			for i, c := range net.Inputs() {
				senses[i] = physiology.SensedCondition{Condition: c, Pull: net.InputPull(i)}
				plain[i] = physiology.SensedCondition{Condition: c, Pull: 1}
			}
			w := physiology.AppearanceForSenses(scores, senses).Sensor
			c := physiology.AppearanceForSenses(scores, plain).Sensor
			weighted[w]++
			counted[c]++
			if w != c {
				disagreed++
			}
			organisms++
			inputs += len(net.Inputs())
		}
		t.Logf("seed %d: %d alive at cycle %d", seed, sim.OrganismCount(), sim.Cycle())
	}

	if organisms == 0 {
		t.Log("nothing survived; no sensors to report")
		return
	}
	t.Logf("%d organisms, %.1f inputs each", organisms, float64(inputs)/float64(organisms))
	for _, class := range order {
		t.Logf("  %-9s weighted %5d (%4.1f%%)   counted %5d (%4.1f%%)",
			names[class],
			weighted[class], 100*float64(weighted[class])/float64(organisms),
			counted[class], 100*float64(counted[class])/float64(organisms))
	}
	t.Logf("the two rules disagree on %d of %d organisms (%.1f%%)",
		disagreed, organisms, 100*float64(disagreed)/float64(organisms))
}
