package replay

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
)

// TestNetDump prints one organism's net out of a recording: its shape, every
// weight and bias, and what flowed along each connection on the cycle shown.
// Asserts nothing — it is for answering "why is that edge not drawn".
//
//	NET_DUMP=recordings/x.pzr NET_DUMP_ID=99506 NET_DUMP_CYCLE=7966 \
//	  go test ./replay/ -run TestNetDump -v
func TestNetDump(t *testing.T) {
	path := os.Getenv("NET_DUMP")
	if path == "" {
		t.Skip("set NET_DUMP=<recording.pzr> to dump an organism's net")
	}
	id, err := strconv.Atoi(os.Getenv("NET_DUMP_ID"))
	if err != nil {
		t.Fatalf("NET_DUMP_ID: %v", err)
	}
	cycle, err := strconv.Atoi(os.Getenv("NET_DUMP_CYCLE"))
	if err != nil {
		t.Fatalf("NET_DUMP_CYCLE: %v", err)
	}

	loadDefaultGlobalsFromDisk(t)
	ctrl, err := NewController(path, &config.Options{IsHeadless: true, ReplayFile: path})
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()

	if err := ctrl.SeekToCycle(cycle); err != nil {
		t.Fatal(err)
	}
	sim := ctrl.Simulation()
	for sim.Cycle() < cycle {
		ctrl.StepForward()
	}
	t.Logf("at cycle %d", sim.Cycle())

	net := sim.GetOrganismNeuralNetByID(id)
	if net == nil {
		t.Fatalf("organism %d has no net at cycle %d", id, cycle)
	}

	cols := append([]int{len(net.Inputs())}, net.LayerSizes()...)
	t.Logf("shape %v, %d nodes, %d weights", cols, net.Nodes(), net.Weights())
	for i, c := range net.Inputs() {
		t.Logf("  input %d: %s", i, d.Map[c])
	}
	for i, a := range net.Outputs() {
		t.Logf("  output %d: %s", i, d.Map[a])
	}

	t.Logf("strongest transmission this cycle: %v", net.StrongestTransmission())
	for li := 0; li < len(net.LayerSizes()); li++ {
		in := net.Activations(li)
		out := net.Activations(li + 1)
		t.Logf("layer %d: feeding values %v -> %v", li, in, out)
		for j := 0; j < net.LayerSizes()[li]; j++ {
			// The bias first, because it is the part of a unit's value that
			// the diagram cannot draw: nothing connects to a unit to deliver
			// it, so with every input at 0 a hidden unit is relu(its bias).
			sum := net.BiasAt(li, j)
			for i := 0; i < cols[li]; i++ {
				sum += net.Transmitted(li, j, i)
			}
			t.Logf("  unit %d: bias %v + inbound = %v -> %v", li, net.BiasAt(li, j), sum, out[j])
			for i := 0; i < cols[li]; i++ {
				w := net.WeightAt(li, j, i)
				t.Logf("  w[%d][%d<-%d] = %v   transmitted %v   drawn %v",
					li, j, i, w, net.Transmitted(li, j, i), w != 0)
			}
		}
	}

	// LAST, because Choose overwrites the activations: everything above
	// reports the cycle as it was played, and the sweep below re-evaluates the
	// net and would clobber it. Reading the per-layer values after this ran is
	// reading the last combination, not the recording.
	//
	// Every combination of the conditions, so "can this net ever do anything
	// different" is answered by evaluation rather than by reading weights.
	n := len(net.Inputs())
	for bits := 0; bits < 1<<n; bits++ {
		chosen := net.Choose(func(c d.Condition) bool {
			for i, in := range net.Inputs() {
				if in == c {
					return bits&(1<<i) != 0
				}
			}
			return false
		})
		var hidden []string
		for li := 1; li < len(net.LayerSizes()); li++ {
			hidden = append(hidden, fmt.Sprint(net.Activations(li)))
		}
		t.Logf("inputs %04b -> hidden %v outputs %v -> %s",
			bits, hidden, net.LastOutputs(), d.Map[chosen])
	}

}
