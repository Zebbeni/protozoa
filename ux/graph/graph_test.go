package graph

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	s "github.com/Zebbeni/protozoa/simulation"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
)

// spyRenderer records Reset calls so a test can prove the main goroutine
// never resets a renderer a background render might be holding.
type spyRenderer struct{ resets int }

func (r *spyRenderer) Render(*s.Simulation, int, int, *gh.Progress) *ebiten.Image { return nil }
func (r *spyRenderer) Reset()                                                     { r.resets++ }

// newTestGraph builds a Graph whose renderers are spies, with a snapshot
// of the maps as a background render would have captured them.
func newTestGraph() (g *Graph, inFlight map[Mode]Renderer, pop, ph *spyRenderer) {
	pop, ph = &spyRenderer{}, &spyRenderer{}
	g = &Graph{
		selectedID:    -1,
		pendingResult: make(chan renderResult, 1),
		renderers:     map[Mode]Renderer{ModePopulation: pop, ModePh: ph},
		selRenderers:  map[Mode]Renderer{},
		images:        map[Mode]*ebiten.Image{},
		selImages:     map[Mode]*ebiten.Image{},
		rendering:     true, // a background render is in flight
	}
	return g, g.renderers, pop, ph
}

// TestInvalidateLeavesInFlightRenderersAlone is the regression test for
// the population-graph crash: Invalidate (fired by a pH colour-scheme
// click) used to Reset() the population renderer while a background
// render was drawing with it, nil-ing its base image mid-draw.
//
// The in-flight render must keep seeing exactly the renderers and map it
// was handed, untouched; the graph must pick up new ones; and the
// in-flight result must be marked stale.
func TestInvalidateLeavesInFlightRenderersAlone(t *testing.T) {
	g, inFlight, pop, ph := newTestGraph()
	genBefore := g.renderGen

	g.Invalidate()

	if pop.resets != 0 || ph.resets != 0 {
		t.Errorf("Invalidate reset renderers a background render may hold: pop=%d ph=%d",
			pop.resets, ph.resets)
	}
	if inFlight[ModePopulation] != pop {
		t.Error("Invalidate mutated the renderer map an in-flight render is iterating")
	}
	if g.renderers[ModePopulation] == pop {
		t.Error("Invalidate should install a fresh population renderer")
	}
	if g.renderGen == genBefore {
		t.Error("Invalidate should bump renderGen so the in-flight result is discarded")
	}
	if !g.forceRender {
		t.Error("Invalidate should schedule a full repaint")
	}
}

// TestPopulationColorChangeLeavesInFlightRenderersAlone pins the same
// guarantee for the other invalidation path.
func TestPopulationColorChangeLeavesInFlightRenderersAlone(t *testing.T) {
	g, inFlight, pop, _ := newTestGraph()

	g.SetPopulationColor(PopulationColor{ByAbility: true})

	if pop.resets != 0 {
		t.Errorf("colour change reset an in-flight renderer %d times", pop.resets)
	}
	if inFlight[ModePopulation] != pop {
		t.Error("colour change mutated the renderer map an in-flight render is iterating")
	}
}

// progressGraph returns a Graph with a render begun at a controllable
// clock, and the clock.
func progressGraph() (*Graph, *time.Time) {
	clock := time.Unix(1000, 0)
	g := &Graph{now: func() time.Time { return clock }}
	g.beginRender()
	return g, &clock
}

// TestProgressWaitsForSlowRenders: routine renders finish inside the
// delay, so the bar must not flash for them.
func TestProgressWaitsForSlowRenders(t *testing.T) {
	g, clock := progressGraph()
	g.progress.AddWork(10)

	if _, show := g.RenderProgress(); show {
		t.Error("progress should not show before progressDelay")
	}
	*clock = clock.Add(progressDelay)
	if _, show := g.RenderProgress(); !show {
		t.Error("progress should show once a render has run past progressDelay")
	}
}

// TestProgressNeedsReportedWork: a slow render that has reported nothing
// has no fraction to draw.
func TestProgressNeedsReportedWork(t *testing.T) {
	g, clock := progressGraph()
	*clock = clock.Add(time.Second)
	if _, show := g.RenderProgress(); show {
		t.Error("progress should not show before any work is reported")
	}
}

// TestProgressNeverMovesBackwards: when a later renderer declares more
// work the raw fraction drops; the bar must hold its position.
func TestProgressNeverMovesBackwards(t *testing.T) {
	g, clock := progressGraph()
	*clock = clock.Add(time.Second)
	g.progress.AddWork(2)
	g.progress.Step()
	first, _ := g.RenderProgress()

	g.progress.AddWork(8)
	second, _ := g.RenderProgress()
	if second < first {
		t.Errorf("progress slid back from %.2f to %.2f", first, second)
	}
}

// TestProgressHidesWhenIdle: nothing shows once no render is in flight.
func TestProgressHidesWhenIdle(t *testing.T) {
	g, clock := progressGraph()
	*clock = clock.Add(time.Second)
	g.progress.AddWork(1)
	g.rendering = false
	if _, show := g.RenderProgress(); show {
		t.Error("progress should not show with no render in flight")
	}
}
