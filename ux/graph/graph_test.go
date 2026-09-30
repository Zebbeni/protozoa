package graph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	s "github.com/Zebbeni/protozoa/simulation"
	gh "github.com/Zebbeni/protozoa/ux/graph/helpers"
	"github.com/Zebbeni/protozoa/ux/graph/population"
)

// spyRenderer records Reset calls so a test can prove the main goroutine never resets a renderer a background render might be holding.
type spyRenderer struct{ resets int }

func (r *spyRenderer) Render(*s.Simulation, int, int, *gh.Progress) *ebiten.Image { return nil }
func (r *spyRenderer) Reset()                                                     { r.resets++ }

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

// progressGraph returns a Graph with a render begun at a controllable clock, and the clock.
func progressGraph() (*Graph, *time.Time) {
	clock := time.Unix(1000, 0)
	g := &Graph{now: func() time.Time { return clock }}
	g.beginRender()
	return g, &clock
}

// TestProgressWaitsForSlowRenders: routine renders finish inside the delay, so the bar must not flash for them.
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

// TestProgressNeedsReportedWork: a slow render that has reported nothing has no fraction to draw.
func TestProgressNeedsReportedWork(t *testing.T) {
	g, clock := progressGraph()
	*clock = clock.Add(time.Second)
	if _, show := g.RenderProgress(); show {
		t.Error("progress should not show before any work is reported")
	}
}

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

func TestProgressHidesWhenIdle(t *testing.T) {
	g, clock := progressGraph()
	*clock = clock.Add(time.Second)
	g.progress.AddWork(1)
	g.rendering = false
	if _, show := g.RenderProgress(); show {
		t.Error("progress should not show with no render in flight")
	}
}

func TestPopulationColorChangeKeepsOtherGraphs(t *testing.T) {
	g, _, _, _ := newTestGraph()
	g.SetPopulationColor(PopulationColor{ByAbility: true})
	if !g.popOnlyRender {
		t.Error("a colour change should repaint only the population graphs")
	}

	g.Invalidate()
	if g.popOnlyRender {
		t.Error("Invalidate changes what every graph looks like, so all of them must repaint")
	}
}

func TestCarriedImagesSurviveARender(t *testing.T) {
	g, _, pop, _ := newTestGraph()
	g.rendering = false
	carried := map[Mode]*ebiten.Image{ModeFood: nil, ModeWalls: nil}

	g.renderInBackground(map[Mode]Renderer{ModePopulation: pop}, nil, false, 0, 1, 0, g.renderGen, nil, carried)

	result := <-g.pendingResult
	for _, mode := range []Mode{ModeFood, ModeWalls, ModePopulation} {
		if _, ok := result.images[mode]; !ok {
			t.Errorf("mode %v missing from the render result", mode)
		}
	}
}

// selectedTestGraph is newTestGraph with an organism selected and one full render already landed.
func selectedTestGraph() *Graph {
	g, _, _, _ := newTestGraph()
	g.rendering = false
	g.currentBarCount = 12
	g.selectedSubTreeRoot = &organism.DescendantNode{}
	g.selRenderers = map[Mode]Renderer{ModePopulation: &spyRenderer{}}
	return g
}

func TestSelectionRenderDoesNotWaitForAPause(t *testing.T) {
	g := selectedTestGraph()
	g.selectionDirty = true

	if !g.needsSelectionRender() {
		t.Error("a selection nothing has rendered yet should ask for a render")
	}
}

func TestSelectedViewAsksForTheImageItIsMissing(t *testing.T) {
	g := selectedTestGraph()
	g.showSelected = true
	g.mode = ModePopulation

	if !g.needsSelectionRender() {
		t.Error("the selected view with no image for its mode should ask for a render")
	}

	g.selImages[ModePopulation] = ebiten.NewImage(1, 1)
	if g.needsSelectionRender() {
		t.Error("once the image is there, nothing more should be scheduled")
	}
}

func TestSelectedViewDoesNotSpinOnModesWithoutASubTree(t *testing.T) {
	g := selectedTestGraph()
	g.showSelected = true
	g.mode = ModePh

	if g.needsSelectionRender() {
		t.Error("pH has no sub-tree renderer; asking for one would re-render every frame")
	}
}

func TestNoSelectionRenderBeforeTheFirstFullRender(t *testing.T) {
	g := selectedTestGraph()
	g.currentBarCount = 0
	g.selectionDirty = true
	g.showSelected = true

	if g.needsSelectionRender() {
		t.Error("a sub-tree render before the first full render has no bars to cover")
	}
}

func TestNoSelectionRenderWithoutASelection(t *testing.T) {
	g := selectedTestGraph()
	g.selectedSubTreeRoot = nil
	g.selectionDirty = true
	g.showSelected = true

	if g.needsSelectionRender() {
		t.Error("no selection, no sub-tree render")
	}
}

func idleTestGraph() (*Graph, *spyRenderer) {
	g, _, pop, _ := newTestGraph()
	g.rendering = false
	g.currentBarCount = 40
	g.images[ModePopulation] = ebiten.NewImage(1, 1)
	return g, pop
}

// prewarmTestGraph is idleTestGraph with a real simulation attached and the graph caught up to it.
func prewarmTestGraph(t *testing.T) *Graph {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var gl config.Globals
	if err := json.Unmarshal(data, &gl); err != nil {
		t.Fatal(err)
	}
	gl.GridUnitsWide, gl.GridUnitsHigh = 12, 10
	config.SetGlobals(&gl)

	g, _ := idleTestGraph()
	g.simulation = s.NewSimulation(&config.Options{IsHeadless: true, Seed: 1, CheckpointInterval: 1 << 30})
	g.aliveCache = population.NewAliveCache()
	g.currentBarCount = g.targetBarCount()
	g.prewarmOrder = prewarmColours()
	// NewGraph sets these; a Graph built as a literal has not.
	g.now = time.Now
	g.popCache = make(map[PopulationColor]popRender)
	return g
}

func TestSwitchingBackReusesTheCachedRenderer(t *testing.T) {
	g, pop := idleTestGraph()
	first := PopulationColor{}
	second := PopulationColor{ByAbility: true}

	g.SetPopulationColor(second)
	if g.renderers[ModePopulation] == pop {
		t.Fatal("switching to an unseen colouring should build a fresh renderer")
	}
	if !g.forceRender {
		t.Fatal("an unseen colouring has to be drawn")
	}
	g.forceRender, g.popOnlyRender = false, false
	secondRenderer := g.renderers[ModePopulation]
	g.images[ModePopulation] = ebiten.NewImage(1, 1)

	g.SetPopulationColor(first)
	if g.renderers[ModePopulation] != pop {
		t.Error("switching back should reinstate the original renderer, not build one")
	}
	if g.forceRender {
		t.Error("switching back to a cached colouring should not schedule a repaint")
	}

	g.SetPopulationColor(second)
	if g.renderers[ModePopulation] != secondRenderer {
		t.Error("the second colouring should have been cached on the way out too")
	}
	if g.forceRender {
		t.Error("switching back to the second cached colouring should not repaint either")
	}
}

func TestCachedSwitchDoesNotMutateAnInFlightMap(t *testing.T) {
	g, pop := idleTestGraph()
	second := PopulationColor{ByAbility: true}

	g.SetPopulationColor(second)
	g.forceRender = false
	g.images[ModePopulation] = ebiten.NewImage(1, 1)

	// A render starts on the second colouring, holding its map.
	handed := g.renderers
	g.rendering = true
	g.renderingPopColor = second

	g.SetPopulationColor(PopulationColor{})
	if g.renderers[ModePopulation] != pop {
		t.Error("switching back should reinstate the cached renderer")
	}
	if handed[ModePopulation] == pop {
		t.Error("the map the in-flight render is iterating was written to")
	}
}

func TestRendererInFlightIsNeverHandedBack(t *testing.T) {
	g, pop := idleTestGraph()
	second := PopulationColor{ByAbility: true}

	// Leave the first colouring while a render on it is in flight, so it is never filed away.
	g.rendering = true
	g.renderingPopColor = PopulationColor{}
	g.SetPopulationColor(second)

	if _, ok := g.popCache[PopulationColor{}]; ok {
		t.Error("a colouring whose renderer is in flight was filed into the cache")
	}
	if !g.forceRender {
		t.Error("an unseen colouring still has to be drawn")
	}
	_ = pop
}

func TestInvalidationClearsEveryCachedColouring(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*Graph)
	}{
		{"Invalidate", (*Graph).Invalidate},
		{"InvalidateTrees", (*Graph).InvalidateTrees},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := idleTestGraph()
			g.SetPopulationColor(PopulationColor{ByAbility: true})
			g.forceRender = false
			g.images[ModePopulation] = ebiten.NewImage(1, 1)
			g.SetPopulationColor(PopulationColor{})
			if len(g.popCache) == 0 {
				t.Fatal("nothing was cached to invalidate")
			}

			tc.call(g)

			if len(g.popCache) != 0 {
				t.Errorf("%d cached colourings survived %s", len(g.popCache), tc.name)
			}
		})
	}
}

func TestPrewarmOnlyRunsWhenIdle(t *testing.T) {
	base := func() *Graph { return prewarmTestGraph(t) }

	if g := base(); !g.startPrewarm() {
		t.Fatal("an idle graph with colourings left should prewarm")
	}

	for _, tc := range []struct {
		name  string
		setup func(*Graph)
	}{
		{"a render is in flight", func(g *Graph) { g.rendering = true }},
		{"a repaint is queued", func(g *Graph) { g.forceRender = true }},
		{"nothing has rendered yet", func(g *Graph) { g.currentBarCount = 0 }},
		{"the order isn't populated yet", func(g *Graph) { g.prewarmOrder = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := base()
			tc.setup(g)
			if g.startPrewarm() {
				t.Errorf("prewarmed while %s", tc.name)
			}
		})
	}
}

func TestPrewarmSkipsWhatIsAlreadyCached(t *testing.T) {
	g := prewarmTestGraph(t)

	first, ok := g.nextPrewarm()
	if !ok {
		t.Fatal("nothing to prewarm on a cold graph")
	}
	if first == g.popColor {
		t.Error("the colouring already on screen was picked for prewarming")
	}

	g.popCache[first] = popRender{barCount: g.currentBarCount}
	second, ok := g.nextPrewarm()
	if !ok {
		t.Fatal("more colourings should remain")
	}
	if second == first {
		t.Error("a cached colouring was picked again")
	}

	// Once every colouring is cached there is nothing left to do.
	for _, pc := range prewarmColours() {
		g.popCache[pc] = popRender{barCount: g.currentBarCount}
	}
	if _, ok := g.nextPrewarm(); ok {
		t.Error("prewarming should stop once every colouring is drawn")
	}
}

func TestPrewarmShowsNoProgressBar(t *testing.T) {
	g := prewarmTestGraph(t)

	g.startPrewarm()
	g.now = func() time.Time { return time.Now().Add(time.Hour) }
	g.progress.AddWork(10)
	g.progress.Step()

	if _, show := g.RenderProgress(); show {
		t.Error("a prewarm put a progress bar over the user's graph")
	}

	// A render the user *did* ask for still reports.
	g.prewarming = false
	if _, show := g.RenderProgress(); !show {
		t.Error("a real render should still show progress")
	}
}

func TestPrewarmResultIsFiledNotDisplayed(t *testing.T) {
	g, _ := idleTestGraph()
	pc := PopulationColor{ByAbility: true, Ability: 1}
	shown := g.images[ModePopulation]
	drawn := ebiten.NewImage(1, 1)

	g.pendingPrewarm.Store(&prewarmRenderers{pc: pc, renderer: &spyRenderer{}})
	g.filePrewarm(renderResult{
		barCount:  g.currentBarCount,
		renderGen: g.renderGen,
		prewarm:   pc,
		isPrewarm: true,
		images:    map[Mode]*ebiten.Image{ModePopulation: drawn},
	})

	entry, ok := g.popCache[pc]
	if !ok {
		t.Fatal("the prewarmed colouring wasn't cached")
	}
	if entry.image != drawn {
		t.Error("the cached entry doesn't hold what was drawn")
	}
	if g.images[ModePopulation] != shown {
		t.Error("a prewarm changed the graph on screen")
	}
}

func TestStalePrewarmIsDropped(t *testing.T) {
	g, _ := idleTestGraph()
	pc := PopulationColor{ByAbility: true, Ability: 1}

	g.pendingPrewarm.Store(&prewarmRenderers{pc: pc, renderer: &spyRenderer{}})
	g.filePrewarm(renderResult{
		barCount:  g.currentBarCount,
		renderGen: g.renderGen - 1, // the world moved on while it drew
		prewarm:   pc,
		isPrewarm: true,
		images:    map[Mode]*ebiten.Image{ModePopulation: ebiten.NewImage(1, 1)},
	})

	if _, ok := g.popCache[pc]; ok {
		t.Error("a prewarm from before an invalidation was filed anyway")
	}
}
