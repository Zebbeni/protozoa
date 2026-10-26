package config

import (
	"encoding/json"
	"testing"
)

// TestTheMediumWorldIsTheShippedDefault is the one that makes the rest of the
// table trustworthy: MEDIUM claims to be what settings/default.json ships, and
// nothing else in the build would notice if it stopped being. A MEDIUM button
// that did not restore the shipped world would be wrong in the quietest way
// available — it would still set a perfectly reasonable world.
func TestTheMediumWorldIsTheShippedDefault(t *testing.T) {
	d := shippedDefaults(t)
	m := worldSizes[WorldMedium]

	if d.GridUnitsWide != m.wide || d.GridUnitsHigh != m.high {
		t.Errorf("default.json grid is %dx%d, MEDIUM is %dx%d",
			d.GridUnitsWide, d.GridUnitsHigh, m.wide, m.high)
	}
	if d.InitialFood != m.food {
		t.Errorf("default.json initial_food is %d, MEDIUM is %d", d.InitialFood, m.food)
	}
	if d.InitialBuriedFood != m.buriedFood {
		t.Errorf("default.json initial_buried_food is %d, MEDIUM is %d", d.InitialBuriedFood, m.buriedFood)
	}

	// Stated the other way round too: applying MEDIUM to the shipped file has
	// to be a no-op, which is what the button promises.
	g := d
	ApplyWorldSize(&g, WorldMedium)
	a, _ := json.Marshal(d)
	b, _ := json.Marshal(g)
	if string(a) != string(b) {
		t.Error("applying MEDIUM to the shipped defaults changed something")
	}
}

// TestEverySizeIsNamedAndOrdered: AllWorldSizes is what the buttons walk, so a
// size missing from it is one with no control, and an unnamed one draws a
// button with no label rather than failing.
func TestEverySizeIsNamedAndOrdered(t *testing.T) {
	if len(AllWorldSizes) != len(worldSizes) {
		t.Fatalf("AllWorldSizes lists %d sizes against %d in the table",
			len(AllWorldSizes), len(worldSizes))
	}
	prev := 0
	for _, size := range AllWorldSizes {
		if size.Name() == "" {
			t.Errorf("size %d has no name", size)
		}
		if cells := size.Cells(); cells <= prev {
			t.Errorf("%s holds %d cells, which is not more than the size before it (%d)",
				size.Name(), cells, prev)
		} else {
			prev = cells
		}
	}
}

// TestTheSizesAreOneWorldAtThreeScales pins the ratio the table is built on:
// each dimension steps by 1.25 and the stocks step with the AREA. Without the
// second half a smaller world starts denser than the numbers were chosen for,
// which is the mistake the scaling exists to prevent.
func TestTheSizesAreOneWorldAtThreeScales(t *testing.T) {
	m := worldSizes[WorldMedium]
	for _, size := range AllWorldSizes {
		w := worldSizes[size]
		share := float64(w.wide*w.high) / float64(m.wide*m.high)

		if want := int(float64(m.food) * share); w.food != want {
			t.Errorf("%s initial_food is %d, want %d (%d x %.4f)", w.name, w.food, want, m.food, share)
		}
		if want := int(float64(m.buriedFood) * share); w.buriedFood != want {
			t.Errorf("%s initial_buried_food is %d, want %d (%d x %.4f)",
				w.name, w.buriedFood, want, m.buriedFood, share)
		}
		// The whole-number property: the figures in the table are exact, not
		// rounded, so no size is an approximation of another.
		if float64(m.food)*share != float64(w.food) {
			t.Errorf("%s initial_food %d is a rounded figure, not an exact share of %d",
				w.name, w.food, m.food)
		}
	}
}

// TestApplyingASizeTwiceChangesNothingTheSecondTime, and clicking through the
// buttons and back lands exactly where it started. A size that scaled what was
// already there instead of writing fixed values would drift by the rounding at
// every click.
func TestApplyingASizeTwiceChangesNothingTheSecondTime(t *testing.T) {
	base := shippedDefaults(t)

	for _, size := range AllWorldSizes {
		once := base
		ApplyWorldSize(&once, size)
		twice := once
		ApplyWorldSize(&twice, size)
		a, _ := json.Marshal(once)
		b, _ := json.Marshal(twice)
		if string(a) != string(b) {
			t.Errorf("applying %s a second time changed the configuration", size.Name())
		}
	}

	// Small, large, small again.
	round := base
	ApplyWorldSize(&round, WorldSmall)
	start, _ := json.Marshal(round)
	ApplyWorldSize(&round, WorldLarge)
	ApplyWorldSize(&round, WorldSmall)
	end, _ := json.Marshal(round)
	if string(start) != string(end) {
		t.Error("small -> large -> small did not land back on small")
	}
}

// TestWorldSizeOfFindsTheSizeItWasGiven, and reports a hand-edited world as
// belonging to none of them rather than lighting the nearest button.
func TestWorldSizeOfFindsTheSizeItWasGiven(t *testing.T) {
	base := shippedDefaults(t)

	for _, size := range AllWorldSizes {
		g := base
		ApplyWorldSize(&g, size)
		got, ok := WorldSizeOf(&g)
		if !ok || got != size {
			t.Errorf("after applying %s, WorldSizeOf says %v (known: %v)", size.Name(), got, ok)
		}
	}

	// A grid nobody offers.
	custom := base
	ApplyWorldSize(&custom, WorldMedium)
	custom.GridUnitsWide += 7
	if _, ok := WorldSizeOf(&custom); ok {
		t.Error("an edited grid was reported as one of the offered sizes")
	}

	// The right grid with an edited food stock is still custom: the stock is
	// part of what the button sets, so it is part of what it means to be on
	// that size.
	custom = base
	ApplyWorldSize(&custom, WorldMedium)
	custom.InitialFood += 1
	if _, ok := WorldSizeOf(&custom); ok {
		t.Error("an edited initial_food was reported as one of the offered sizes")
	}
}

// TestTheBrowserGetsASizeRatherThanItsOwnNumbers: the browser's world has to
// be one a desktop user can also reach from the settings screen, or it is a
// configuration only measurable by building for wasm — which is the thing this
// untagged file exists to prevent.
func TestTheBrowserGetsASizeRatherThanItsOwnNumbers(t *testing.T) {
	before := shippedDefaults(t)
	g := before
	BrowserDefaults(&g)

	size, ok := WorldSizeOf(&g)
	if !ok {
		t.Fatal("the browser world matches none of the offered sizes")
	}
	if size != BrowserWorldSize {
		t.Errorf("the browser world is %s, want %s", size.Name(), BrowserWorldSize.Name())
	}
	if g.GridUnitsWide*g.GridUnitsHigh >= before.GridUnitsWide*before.GridUnitsHigh {
		t.Errorf("the browser world is %d cells against the desktop's %d",
			g.GridUnitsWide*g.GridUnitsHigh, before.GridUnitsWide*before.GridUnitsHigh)
	}
}

// TestBrowserDefaultsChangeNothingElse: every other setting is the desktop
// baseline, so a browser-only balance change has to be added deliberately
// rather than drifting in.
func TestBrowserDefaultsChangeNothingElse(t *testing.T) {
	before := shippedDefaults(t)
	g := before
	BrowserDefaults(&g)

	// Put back exactly what a world size is allowed to touch — the grid and
	// the two food stocks, nothing else. Anything left over is a browser-only
	// balance change that drifted in rather than being decided.
	g.GridUnitsWide, g.GridUnitsHigh = before.GridUnitsWide, before.GridUnitsHigh
	g.InitialFood, g.InitialBuriedFood = before.InitialFood, before.InitialBuriedFood
	// also accommodate the browser-only default to turn music off
	g.Music = before.Music

	a, _ := json.Marshal(before)
	b, _ := json.Marshal(g)
	if string(a) != string(b) {
		t.Error("the browser world differs from the desktop baseline beyond the grid and the food stocks")
	}
}
