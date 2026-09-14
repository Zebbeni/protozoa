package manager

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/Zebbeni/protozoa/checkpoint"
)

func newHistoryManager() *OrganismManager {
	return &OrganismManager{history: map[HistoryType]map[int]map[int]int32{
		HistoryPopulation:     {},
		HistoryPhDistribution: {},
		HistoryFood:           {},
		HistoryWalls:          {},
	}}
}

// TestHistoryRoundTripsThroughReplay is the regression test for the food
// graph showing zero for most of a replay: only pH history was saved, so
// every cycle before playback resumed had no food count. Food and wall
// history must survive capture, the replay file's gob encoding, and
// restore.
func TestHistoryRoundTripsThroughReplay(t *testing.T) {
	src := newHistoryManager()
	for cycle := 0; cycle <= 100; cycle += 20 {
		src.history[HistoryPhDistribution][cycle] = map[int]int32{3: int32(cycle)}
		src.history[HistoryFood][cycle] = map[int]int32{0: int32(cycle + 1)}
		src.history[HistoryWalls][cycle] = map[int]int32{0: int32(cycle + 2)}
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src.CaptureHistory()); err != nil {
		t.Fatal(err)
	}
	var payload checkpoint.HistoryPayload
	if err := gob.NewDecoder(&buf).Decode(&payload); err != nil {
		t.Fatal(err)
	}

	dst := newHistoryManager()
	dst.RestoreHistory(&payload)
	for _, h := range []HistoryType{HistoryPhDistribution, HistoryFood, HistoryWalls} {
		for cycle, want := range src.history[h] {
			got := dst.history[h][cycle]
			for k, v := range want {
				if got[k] != v {
					t.Errorf("history %d cycle %d key %d: got %d, want %d", h, cycle, k, got[k], v)
				}
			}
		}
	}
}

// TestRestoreHistoryFromOlderReplay: files written before food and wall
// history were saved decode with nil maps. Restoring one must leave
// writable maps, or the first history update after a seek panics.
func TestRestoreHistoryFromOlderReplay(t *testing.T) {
	m := newHistoryManager()
	m.RestoreHistory(&checkpoint.HistoryPayload{PhDistribution: map[int]map[int]int32{0: {1: 5}}})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("writing history after restoring an older replay panicked: %v", r)
		}
	}()
	m.history[HistoryFood][20] = map[int]int32{0: 1}
	m.history[HistoryWalls][20] = map[int]int32{0: 1}
}

// TestRestoreHistoryDoesNotAliasPayload: the replay controller restores
// the same cached payload on every seek, so forward play writing into the
// restored maps must not change it.
func TestRestoreHistoryDoesNotAliasPayload(t *testing.T) {
	payload := &checkpoint.HistoryPayload{
		PhDistribution: map[int]map[int]int32{0: {1: 1}},
		Food:           map[int]map[int]int32{0: {0: 1}},
		Walls:          map[int]map[int]int32{0: {0: 1}},
	}
	m := newHistoryManager()
	m.RestoreHistory(payload)

	m.history[HistoryFood][20] = map[int]int32{0: 9}
	m.history[HistoryWalls][0][0] = 9
	m.history[HistoryPhDistribution][0][1] = 9

	if len(payload.Food) != 1 || payload.Walls[0][0] != 1 || payload.PhDistribution[0][1] != 1 {
		t.Error("forward play mutated the cached replay payload")
	}
}
