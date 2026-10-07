package graph

import (
	"testing"
	"time"

	"github.com/Zebbeni/protozoa/physiology"
)

func abilityColor(a physiology.Ability) PopulationColor {
	return PopulationColor{ByAbility: true, Ability: a}
}

// TestTheColouringOnScreenIsAlwaysReady, whatever the cache holds: it is
// already drawn, which is the only thing readiness claims.
func TestTheColouringOnScreenIsAlwaysReady(t *testing.T) {
	g, _, _, _ := newTestGraph()
	g.popColor = abilityColor(physiology.AbilityEating)
	if st := g.PopulationColorStatus(g.popColor); !st.Ready || st.Drawing {
		t.Errorf("the displayed colouring reports %+v, want Ready", st)
	}
}

func TestACachedColouringIsReady(t *testing.T) {
	g, _, pop, _ := newTestGraph()
	pc := abilityColor(physiology.AbilityAttack)
	g.popCache = map[PopulationColor]popRender{pc: {renderer: pop}}
	if st := g.PopulationColorStatus(pc); !st.Ready {
		t.Errorf("a cached colouring reports %+v, want Ready", st)
	}
}

// TestAColouringBeingDrawnAheadReportsItsProgress: the button shows this as
// a fill, which is why the fraction has to come back rather than just a
// "busy" flag.
func TestAColouringBeingDrawnAheadReportsItsProgress(t *testing.T) {
	g, _, _, _ := newTestGraph()
	// beginRender stamps the clock, which a hand-built Graph has not got.
	g.now = time.Now
	pc := abilityColor(physiology.AbilityDigging)
	g.beginRender(true)
	g.prewarming = true
	g.prewarmColor = pc
	g.progress.AddWork(4)
	g.progress.Step()

	st := g.PopulationColorStatus(pc)
	if st.Ready || !st.Drawing {
		t.Fatalf("a colouring being drawn ahead reports %+v, want Drawing", st)
	}
	if st.Fraction <= 0 || st.Fraction > 1 {
		t.Errorf("fraction %v, want a share of the work done", st.Fraction)
	}
}

// TestAnUndrawnColouringIsNeitherReadyNorDrawing — its button is faded with
// no fill, because nothing is happening for it yet.
func TestAnUndrawnColouringIsNeitherReadyNorDrawing(t *testing.T) {
	g, _, _, _ := newTestGraph()
	st := g.PopulationColorStatus(abilityColor(physiology.AbilityDefense))
	if st.Ready || st.Drawing || st.Fraction != 0 {
		t.Errorf("an undrawn colouring reports %+v, want all zero", st)
	}
}

// TestPrewarmActiveIsFalseBeforeTheFirstRealRender: prewarmOrder is only
// filled once one has landed, so before that no colouring will ever become
// ready on its own. A caller grey out un-ready buttons then would leave
// every one of them permanently dead.
func TestPrewarmActiveIsFalseBeforeTheFirstRealRender(t *testing.T) {
	g, _, _, _ := newTestGraph()
	if g.PrewarmActive() {
		t.Error("prewarm reports active before any render has landed")
	}
	g.prewarmOrder = prewarmColours()
	if !g.PrewarmActive() {
		t.Error("prewarm reports inactive once the order is filled")
	}
}

// TestAButtonFillNeverGoesBackwards: a prewarm draws the population graph
// and then the selected sub-tree through ONE Progress, so AddWork grows the
// total when the second starts and the raw fraction drops back to about a
// half. The button fill would visibly restart, which reads as the work being
// redone — reported as "filling up twice in a row before moving to the next
// button".
func TestAButtonFillNeverGoesBackwards(t *testing.T) {
	g, _, _, _ := newTestGraph()
	g.now = time.Now
	pc := abilityColor(physiology.AbilityMovement)
	g.beginRender(true)
	g.prewarming = true
	g.prewarmColor = pc

	read := func() float64 { return g.PopulationColorStatus(pc).Fraction }

	// The first graph: four units, all done.
	g.progress.AddWork(4)
	for i := 0; i < 4; i++ {
		g.progress.Step()
	}
	first := read()
	if first != 1 {
		t.Fatalf("after the first graph finished the fill reads %v, want 1", first)
	}

	// The sub-tree graph declares its own work through the same tracker.
	g.progress.AddWork(4)
	if raw, _ := g.progress.Fraction(); raw >= first {
		t.Fatalf("the raw fraction is %v; this test is not exercising the drop", raw)
	}
	if got := read(); got < first {
		t.Errorf("the fill fell from %v to %v when the second graph started", first, got)
	}
	g.progress.Step()
	if got := read(); got < first {
		t.Errorf("the fill fell to %v part way through the second graph", got)
	}
}

// TestTheBarAndTheButtonAgree: both read the same monotonic value, so a
// colouring cannot be shown as half drawn on its button and nearly done on
// the bar.
func TestTheBarAndTheButtonAgree(t *testing.T) {
	g, _, _, _ := newTestGraph()
	g.now = time.Now
	pc := abilityColor(physiology.AbilityTolerance)
	g.beginRender(false)
	// beginRender stamps renderStarted from the clock, so the clock has to
	// move past progressDelay for the bar to report at all.
	g.now = func() time.Time { return time.Now().Add(time.Hour) }
	g.prewarming = true
	g.prewarmColor = pc
	g.progress.AddWork(4)
	g.progress.Step()

	button := g.PopulationColorStatus(pc).Fraction
	g.prewarming = false
	bar, show := g.RenderProgress()
	if !show {
		t.Fatal("the bar declined to report, so there is nothing to compare")
	}
	if bar != button {
		t.Errorf("the bar reads %v and the button %v", bar, button)
	}
}
