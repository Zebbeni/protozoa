package ux

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	res "github.com/Zebbeni/protozoa/resources"
	"github.com/Zebbeni/protozoa/simulation"
)

func loadShippedGlobals(t *testing.T) *config.Globals {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return &g
}

func TestEndConditionsListWhatIsInForce(t *testing.T) {
	g := &config.Globals{MinOrganisms: 10, MaxCycles: 5000}
	lines := endConditionLines(g, simulation.EndNone, 0)
	if len(lines) != 3 {
		t.Fatalf("with both limits set, got %d lines, want extinction + 2", len(lines))
	}
	if !strings.Contains(lines[1].label, "10") || !strings.Contains(lines[2].label, "5000") {
		t.Errorf("the lines don't carry their values: %q, %q", lines[1].label, lines[2].label)
	}

	off := endConditionLines(&config.Globals{}, simulation.EndNone, 0)
	if len(off) != 1 || off[0].label != "extinction" {
		t.Errorf("with both limits off, got %v, want extinction alone", off)
	}
}

func TestOnlyTheFiredConditionIsLit(t *testing.T) {
	g := &config.Globals{MinOrganisms: 10, MaxCycles: 5000}

	for _, tc := range []struct {
		fired    simulation.EndCondition
		wantLit  string
		anyIsLit bool
	}{
		{simulation.EndNone, "", false},
		{simulation.EndExtinct, "extinction", true},
		{simulation.EndBelowMinimum, "min organisms: 10", true},
		{simulation.EndMaxCycles, "max cycles: 5000", true},
	} {
		lit := ""
		count := 0
		for _, line := range endConditionLines(g, tc.fired, 0) {
			if line.met {
				lit = line.label
				count++
			}
		}
		if count > 1 {
			t.Errorf("%v lit %d lines; only the one that fired should be", tc.fired, count)
		}
		if lit != tc.wantLit {
			t.Errorf("%v lit %q, want %q", tc.fired, lit, tc.wantLit)
		}
	}
}

func TestEndConditionsBlockGrowsUpward(t *testing.T) {
	// The line height comes from the face.
	loadKeyGlobals(t)
	res.UseDirAssets("..")
	res.Init()

	one := endConditionsHeight(endConditionLines(&config.Globals{}, simulation.EndNone, 0))
	three := endConditionsHeight(endConditionLines(
		&config.Globals{MinOrganisms: 10, MaxCycles: 5000}, simulation.EndNone, 0))

	if one <= 0 {
		t.Fatalf("a one-line list measured %d high", one)
	}
	if three <= one {
		t.Errorf("three lines measured %d, not taller than one line's %d", three, one)
	}
	if three != 3*one {
		t.Errorf("three lines measured %d, want exactly three times %d", three, one)
	}
}

func TestReplaySizeLineShowsProgressNotJustTheLimit(t *testing.T) {
	g := &config.Globals{MaxReplaySizeMb: 250}

	lines := endConditionLines(g, simulation.EndNone, 120<<20)
	var size string
	for _, line := range lines {
		if strings.Contains(line.label, "replay") {
			size = line.label
		}
	}
	if size == "" {
		t.Fatal("no replay-size line with the cap set")
	}
	if !strings.Contains(size, "120MB") {
		t.Errorf("line %q doesn't show how far along the run is", size)
	}
	if !strings.Contains(size, "250MB") {
		t.Errorf("line %q doesn't show the limit", size)
	}

	for _, line := range endConditionLines(g, simulation.EndMaxReplaySize, 250<<20) {
		if strings.Contains(line.label, "replay") && !line.met {
			t.Error("the replay-size line should be lit when it is what fired")
		}
	}
}

func TestReplaySizeReadsBelowAMegabyte(t *testing.T) {
	for _, tc := range []struct {
		bytes int64
		want  string
	}{
		{0, "0.0MB"},
		{300 << 10, "0.3MB"},
		{1 << 20, "1MB"},
		{250 << 20, "250MB"},
	} {
		if got := formatBytes(tc.bytes); got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

func TestReplaySizeIsAlwaysListed(t *testing.T) {
	g := loadShippedGlobals(t)
	if g.MaxReplaySizeMb <= 0 {
		t.Fatalf("the shipped max_replay_size_mb is %d; it has no unlimited setting", g.MaxReplaySizeMb)
	}
	var found bool
	for _, line := range endConditionLines(g, simulation.EndNone, 0) {
		if strings.Contains(line.label, "replay") {
			found = true
		}
	}
	if !found {
		t.Error("the shipped defaults don't list the replay-size condition")
	}
}
