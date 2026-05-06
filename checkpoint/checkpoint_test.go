package checkpoint

import (
	"os"
	"testing"
)

func TestWriterReaderRoundTrip(t *testing.T) {
	path := "test_roundtrip.pzr"
	defer os.Remove(path)

	header := FileHeader{
		Seed:               42,
		CheckpointInterval: 100,
		GridUnitsWide:      10,
		GridUnitsHigh:      8,
	}

	// Write
	w, err := NewWriter(path, header)
	if err != nil {
		t.Fatal(err)
	}

	snap := &SnapshotPayload{
		Cycle:                 0,
		RNGState:              []byte{1, 2, 3, 4},
		TotalOrganismsCreated: 5,
		Organisms: []OrganismRecord{
			{ID: 1, Health: 10.5, LocationX: 3, LocationY: 4, DecisionTree: "02"},
			{ID: 2, Health: 8.0, LocationX: 7, LocationY: 2, DecisionTree: "030102"},
		},
		OrganismGrid: [][]int{{-1, 1}, {2, -1}},
		CurrentPhMap: [][]float64{{5.0, 5.1}, {4.9, 5.0}},
		FoodItems:    []FoodRecord{{X: 1, Y: 1, Value: 50}},
		Ancestors:    []AncestorRecord{{ID: 1, ColorR: 0.5, ColorG: 0.3, ColorB: 0.1}},
	}
	if err := w.WriteSnapshot(snap); err != nil {
		t.Fatal(err)
	}

	delta := &DeltaPayload{
		Cycle:  1,
		Births: []OrganismRecord{{ID: 3, Health: 5.0, DecisionTree: "02"}},
		Deaths: []uint32{2},
		Moves:  []MoveRecord{{ID: 1, LocationX: 4, LocationY: 4}},
	}
	if err := w.WriteDelta(delta); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Read
	r, err := OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	// Verify header
	if r.Header.Seed != 42 {
		t.Errorf("seed: got %d, want 42", r.Header.Seed)
	}
	if r.Header.GridUnitsWide != 10 {
		t.Errorf("grid width: got %d, want 10", r.Header.GridUnitsWide)
	}

	// Verify snapshot index
	if r.SnapshotCount() != 1 {
		t.Fatalf("snapshot count: got %d, want 1", r.SnapshotCount())
	}

	// Read snapshot
	readSnap, err := r.ReadSnapshot(0)
	if err != nil {
		t.Fatal(err)
	}
	if readSnap.Cycle != 0 {
		t.Errorf("snapshot cycle: got %d, want 0", readSnap.Cycle)
	}
	if len(readSnap.Organisms) != 2 {
		t.Errorf("organisms: got %d, want 2", len(readSnap.Organisms))
	}
	if readSnap.Organisms[0].Health != 10.5 {
		t.Errorf("organism health: got %f, want 10.5", readSnap.Organisms[0].Health)
	}
	if readSnap.Organisms[1].DecisionTree != "030102" {
		t.Errorf("decision tree: got %q, want %q", readSnap.Organisms[1].DecisionTree, "030102")
	}

	// Read sections sequentially
	r.SeekAfterHeader()
	sType, cycle, payload, err := r.ReadNextSection()
	if err != nil {
		t.Fatal(err)
	}
	if sType != SectionSnapshot || cycle != 0 {
		t.Errorf("first section: type=%d cycle=%d, want type=%d cycle=0", sType, cycle, SectionSnapshot)
	}
	if _, ok := payload.(*SnapshotPayload); !ok {
		t.Error("first section payload is not *SnapshotPayload")
	}

	sType, cycle, payload, err = r.ReadNextSection()
	if err != nil {
		t.Fatal(err)
	}
	if sType != SectionDelta || cycle != 1 {
		t.Errorf("second section: type=%d cycle=%d, want type=%d cycle=1", sType, cycle, SectionDelta)
	}
	readDelta, ok := payload.(*DeltaPayload)
	if !ok {
		t.Fatal("second section payload is not *DeltaPayload")
	}
	if len(readDelta.Births) != 1 {
		t.Errorf("delta births: got %d, want 1", len(readDelta.Births))
	}
	if len(readDelta.Deaths) != 1 || readDelta.Deaths[0] != 2 {
		t.Errorf("delta deaths: got %v, want [2]", readDelta.Deaths)
	}
}
