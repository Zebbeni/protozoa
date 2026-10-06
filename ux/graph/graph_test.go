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
	"github.com/Zebbeni/protozoa/physiology"
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
	g.beginRender(false)
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
	g.speculative = false
	if _, show := g.RenderProgress(); !show {
		t.Error("a real render should still show progress")
	}
}

// TestASubTreeRenderForAHiddenViewShowsNoProgressBar: every selection change
// schedules a sub-tree render whether or not anything is going to show it,
// so with the selected view switched off the bar would blank the graph the
// user is actually looking at, repeatedly, as the selection moves.
func TestASubTreeRenderForAHiddenViewShowsNoProgressBar(t *testing.T) {
	g := prewarmTestGraph(t)
	g.showSelected = false

	g.beginRender(!g.showSelected)
	g.now = func() time.Time { return time.Now().Add(time.Hour) }
	g.progress.AddWork(10)
	g.progress.Step()

	if _, show := g.RenderProgress(); show {
		t.Error("a sub-tree render for a hidden view put a progress bar over the graph on screen")
	}

	// With the view ON the user is waiting for exactly that image.
	g.showSelected = true
	g.beginRender(!g.showSelected)
	// beginRender restamps renderStarted from the clock, so the clock has to
	// move again for this render to count as slow.
	g.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	g.progress.AddWork(10)
	g.progress.Step()
	if _, show := g.RenderProgress(); !show {
		t.Error("a sub-tree render the user is waiting for should report progress")
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

// TestSwitchingAwayDuringAPrewarmStillCaches is the reported fault: clicking
// a new colouring while a speculative render is in flight loses the one being
// left, so coming back to it costs a full redraw even though it was cached a
// moment ago.
//
// The guard that caused it is right for a REAL render — that one was handed
// the displayed renderers and a goroutine is writing to them — and wrong for
// a prewarm, which builds its own. beginRender records the displayed colour
// either way, so a prewarm marked the displayed colouring as in flight when
// nothing of its was.
func TestSwitchingAwayDuringAPrewarmStillCaches(t *testing.T) {
	g := &Graph{
		popCache:     map[PopulationColor]popRender{},
		renderers:    map[Mode]Renderer{ModePopulation: stubRenderer{}},
		selRenderers: map[Mode]Renderer{},
		images:       map[Mode]*ebiten.Image{},
		selImages:    map[Mode]*ebiten.Image{},
		popColor:     popLineage,
	}

	// A prewarm of some OTHER colouring is in flight. It holds the rendering
	// flag, and beginRender recorded the displayed colour.
	g.rendering = true
	g.prewarming = true
	g.renderingPopColor = g.popColor

	g.SetPopulationColor(popByAbility)

	if _, ok := g.popCache[popLineage]; !ok {
		t.Error("the colouring being left was not cached, so returning to it " +
			"will redraw from scratch; a prewarm does not touch its renderers")
	}
}

// TestSwitchingAwayDuringARealRenderDoesNotCache is the other half, and the
// reason the guard exists: a real render WAS handed the displayed renderers,
// so filing them away hands out something a goroutine is still writing to.
func TestSwitchingAwayDuringARealRenderDoesNotCache(t *testing.T) {
	g := &Graph{
		popCache:     map[PopulationColor]popRender{},
		renderers:    map[Mode]Renderer{ModePopulation: stubRenderer{}},
		selRenderers: map[Mode]Renderer{},
		images:       map[Mode]*ebiten.Image{},
		selImages:    map[Mode]*ebiten.Image{},
		popColor:     popLineage,
	}
	g.rendering = true
	g.prewarming = false
	g.renderingPopColor = g.popColor

	g.SetPopulationColor(popByAbility)

	if _, ok := g.popCache[popLineage]; ok {
		t.Error("renderers a live render is writing to were filed into the cache")
	}
}

// stubRenderer stands in for a population renderer: these tests are about
// which renderer object ends up where, never about pixels.
type stubRenderer struct{ id int }

func (stubRenderer) Render(*s.Simulation, int, int, *gh.Progress) *ebiten.Image { return nil }
func (stubRenderer) Reset()                                                     {}

var (
	popLineage   = PopulationColor{}
	popByAbility = PopulationColor{ByAbility: true, Ability: physiology.AbilityDigging}
)

// TestSwitchingToTheColouringBeingPrewarmedAdoptsIt: clicking the colouring a
// speculative render is ALREADY drawing used to discard it — renderGen was
// bumped, which drops the result when it lands, and a second render of the
// same thing was queued behind it. The user waited for the prewarm to finish,
// watched it be thrown away, then waited again for an identical render.
//
// The work in flight is exactly what was asked for, so it is adopted.
func TestSwitchingToTheColouringBeingPrewarmedAdoptsIt(t *testing.T) {
	g := &Graph{
		popCache:     map[PopulationColor]popRender{},
		renderers:    map[Mode]Renderer{ModePopulation: stubRenderer{}},
		selRenderers: map[Mode]Renderer{},
		images:       map[Mode]*ebiten.Image{},
		selImages:    map[Mode]*ebiten.Image{},
		popColor:     popLineage,
	}
	g.rendering = true
	g.prewarming = true
	g.prewarmColor = popByAbility
	g.renderingPopColor = g.popColor
	gen := g.renderGen

	g.SetPopulationColor(popByAbility)

	if g.renderGen != gen {
		t.Errorf("renderGen moved from %d to %d, which drops the prewarm that was "+
			"drawing exactly this colouring", gen, g.renderGen)
	}
	if g.forceRender {
		t.Error("a second render was queued for a colouring already being drawn")
	}
	if !g.adoptPrewarm {
		t.Error("the in-flight prewarm was not adopted")
	}
	// The colouring being left is still cached, as in the test above.
	if _, ok := g.popCache[popLineage]; !ok {
		t.Error("the colouring being left was not cached")
	}
}

// TestSwitchingToAnUnrelatedColourStillRedraws: adoption applies only when
// the in-flight prewarm is for the colouring being asked for.
func TestSwitchingToAnUnrelatedColourStillRedraws(t *testing.T) {
	g := &Graph{
		popCache:     map[PopulationColor]popRender{},
		renderers:    map[Mode]Renderer{ModePopulation: stubRenderer{}},
		selRenderers: map[Mode]Renderer{},
		images:       map[Mode]*ebiten.Image{},
		selImages:    map[Mode]*ebiten.Image{},
		popColor:     popLineage,
	}
	g.rendering = true
	g.prewarming = true
	g.prewarmColor = PopulationColor{ByAbility: true, Ability: physiology.AbilityEating}
	gen := g.renderGen

	g.SetPopulationColor(popByAbility)

	if g.renderGen == gen {
		t.Error("switching to a colouring nothing is drawing should start a render")
	}
	if g.adoptPrewarm {
		t.Error("a prewarm for a different colouring was adopted")
	}
}

// TestAnAdoptedPrewarmGoesOnScreen is the other end of adoption: the result
// is the render the user is waiting on, so it must be installed rather than
// filed into the cache and left invisible.
func TestAnAdoptedPrewarmGoesOnScreen(t *testing.T) {
	g := &Graph{
		popCache:     map[PopulationColor]popRender{},
		renderers:    map[Mode]Renderer{ModePopulation: stubRenderer{id: 1}},
		selRenderers: map[Mode]Renderer{},
		images:       map[Mode]*ebiten.Image{},
		selImages:    map[Mode]*ebiten.Image{},
		popColor:     popByAbility,
	}
	drawn := stubRenderer{id: 2}
	g.pendingPrewarm.Store(&prewarmRenderers{pc: popByAbility, renderer: drawn})

	g.installPrewarm(renderResult{prewarm: popByAbility, barCount: 42})

	if got := g.renderers[ModePopulation]; got != Renderer(drawn) {
		t.Errorf("the displayed renderer is %v, want the one the prewarm drew with", got)
	}
	if g.currentBarCount != 42 {
		t.Errorf("bar count is %d, want the adopted render's 42", g.currentBarCount)
	}
}

// TestAnAdoptedPrewarmWithoutItsRenderersRedraws: the renderers come back
// through an atomic handoff, and a result whose handoff did not arrive has
// nothing to install — falling through would leave the old colouring's
// renderer on screen under the new colouring's name.
func TestAnAdoptedPrewarmWithoutItsRenderersRedraws(t *testing.T) {
	g := &Graph{
		popCache:     map[PopulationColor]popRender{},
		renderers:    map[Mode]Renderer{ModePopulation: stubRenderer{id: 1}},
		selRenderers: map[Mode]Renderer{},
		images:       map[Mode]*ebiten.Image{},
		selImages:    map[Mode]*ebiten.Image{},
		popColor:     popByAbility,
	}
	gen := g.renderGen

	g.installPrewarm(renderResult{prewarm: popByAbility, barCount: 42})

	if g.renderGen == gen || !g.forceRender {
		t.Error("with no renderers handed back, the colouring should be redrawn")
	}
}
