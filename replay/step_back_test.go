package replay

import (
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simulation"
)

// netReadings is every brain's display state at one cycle: what flowed
// through each column of each net on the Choose that cycle ran.
type netReadings map[int][][]float64

func readNets(t *testing.T, ctrl *Controller) netReadings {
	t.Helper()
	sim := ctrl.Simulation()
	out := netReadings{}
	for id := range sim.GetAllOrganismInfo() {
		net := sim.GetOrganismNeuralNetByID(id)
		if net == nil {
			continue
		}
		cols := make([][]float64, 0, len(net.LayerSizes())+1)
		for col := 0; col <= len(net.LayerSizes()); col++ {
			cols = append(cols, append([]float64(nil), net.Activations(col)...))
		}
		out[id] = cols
	}
	return out
}

func compareNets(t *testing.T, label string, want, got netReadings) {
	t.Helper()
	empty, differ := 0, 0
	for id, wantCols := range want {
		gotCols, ok := got[id]
		if !ok {
			t.Errorf("%s: organism %d is not in the restored cycle", label, id)
			continue
		}
		carried := false
		for col := range wantCols {
			if col >= len(gotCols) || len(gotCols[col]) != len(wantCols[col]) {
				differ++
				break
			}
			for i := range wantCols[col] {
				if gotCols[col][i] != wantCols[col][i] {
					differ++
					break
				}
			}
			if len(gotCols[col]) > 0 {
				carried = true
			}
		}
		if !carried {
			empty++
		}
	}
	if empty > 0 {
		t.Errorf("%s: %d of %d nets came back with no activations at all, so the SIGNAL view draws nothing flowing",
			label, empty, len(want))
	}
	if differ > 0 {
		t.Errorf("%s: %d of %d nets came back carrying something other than what the cycle ran",
			label, differ, len(want))
	}
}

// TestSteppingBackLandsOnACyclePlayedNotOneRestored: a snapshot carries what
// the world IS, not what each brain computed when it chose, so a state
// restored without playing a cycle leaves every net's activations empty and
// the panel's SIGNAL view with nothing flowing through it. The reported
// symptom was the diagram not updating until next-cycle had been pressed once.
//
// Both ways back are covered: the in-memory ring, which is what the button
// hits, and the seek that takes over when the ring has lost the cycle.
func TestSteppingBackLandsOnACyclePlayedNotOneRestored(t *testing.T) {
	loadDefaultGlobalsFromDisk(t)
	g := config.GetCurrentGlobals()
	g.UseNeuralNets = true
	config.SetGlobals(g)

	// The interval puts a snapshot exactly on the target, which is the seek
	// case that would otherwise play no cycles at all.
	const target, interval = 60, 20
	path := filepath.Join(t.TempDir(), "neural.pzr")
	rec := simulation.NewSimulation(&config.Options{
		IsHeadless: true, Seed: 6, CheckpointInterval: interval, CheckpointFile: path,
	})
	for rec.Cycle() < target+2 && rec.OrganismCount() > 0 {
		rec.Update()
	}
	rec.CloseRecorder()
	if rec.OrganismCount() == 0 {
		t.Fatalf("the recorded world died before cycle %d", target+2)
	}

	ctrl, err := NewController(path, &config.Options{IsHeadless: true, ReplayFile: path})
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()

	for ctrl.Cycle() < target {
		ctrl.StepForward()
	}
	want := readNets(t, ctrl)
	if len(want) == 0 {
		t.Fatalf("cycle %d holds no organism with a net", target)
	}

	ctrl.StepForward()
	if !ctrl.StepBackward() {
		t.Fatal("the ring step back refused")
	}
	if ctrl.Cycle() != target {
		t.Fatalf("stepped back to cycle %d, wanted %d", ctrl.Cycle(), target)
	}
	compareNets(t, "ring", want, readNets(t, ctrl))

	ctrl.StepForward()
	ctrl.ring.Clear()
	if !ctrl.StepBackward() {
		t.Fatal("the seek step back refused")
	}
	if ctrl.Cycle() != target {
		t.Fatalf("seeking back landed on cycle %d, wanted %d", ctrl.Cycle(), target)
	}
	compareNets(t, "seek", want, readNets(t, ctrl))
}
