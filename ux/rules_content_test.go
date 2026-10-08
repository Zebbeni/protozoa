package ux

import (
	"strings"
	"testing"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/physiology"
	r "github.com/Zebbeni/protozoa/resources"
)

// rulesProse is every word of text the screen draws, for claims that are
// about the writing rather than about one block.
func rulesProse(t *testing.T) string {
	t.Helper()
	loadKeyGlobals(t)
	var sb strings.Builder
	for _, b := range rulesContent() {
		switch v := b.(type) {
		case rulesHeading:
			sb.WriteString(v.text + "\n")
		case rulesText:
			sb.WriteString(v.text + "\n")
		case rulesTable:
			for _, row := range v.rows {
				sb.WriteString(row[0] + " " + row[1] + "\n")
			}
		case rulesAppearanceGrid:
			for _, dim := range v.cells {
				sb.WriteString(dim.title + "\n")
				for _, c := range dim.cells {
					sb.WriteString(c.label + "\n")
				}
			}
		}
	}
	return sb.String()
}

// TestEveryAnimationIsExplained: the action list is what tells a player how
// to read a crowd, so an animation with no name is one they will see on the
// grid and have no way to identify. An omission has to be declared in
// actionsNotShown rather than being a gap in either list.
func TestEveryAnimationIsExplained(t *testing.T) {
	shown := map[animation.Animation]int{}
	for _, anim := range actionOrder {
		shown[anim]++
	}
	want := 0
	for _, anim := range animation.AllAnimations {
		if actionsNotShown[anim] {
			if shown[anim] != 0 {
				t.Errorf("animation %d is in actionsNotShown and in the list", anim)
			}
			continue
		}
		want++
		if _, ok := actionNames[anim]; !ok {
			t.Errorf("animation %d has no name in the rules screen", anim)
		}
		if shown[anim] != 1 {
			t.Errorf("animation %d appears %d times in the action list", anim, shown[anim])
		}
	}
	if got := len(actionCells()); got != want {
		t.Errorf("the action list shows %d animations, want %d", got, want)
	}
}

// TestEveryAbilityIsExplained: the budget is the central mechanic, so an
// ability missing from the table is one a player cannot find out about.
func TestEveryAbilityIsExplained(t *testing.T) {
	table := abilityTable()
	if got, want := len(table.rows), len(physiology.AllAbilities); got != want {
		t.Fatalf("the ability table has %d rows for %d abilities", got, want)
	}
	for _, a := range physiology.AllAbilities {
		e, ok := abilityEffects[a]
		if !ok {
			t.Errorf("ability %v is not in the rules screen", a)
			continue
		}
		if e[1] == "" {
			t.Errorf("ability %v has a name and no explanation", a)
		}
	}
}

// TestEveryAppearanceClassIsShown: the screen claims you can read an
// organism from its sprite, so it has to show every class it might wear.
func TestEveryAppearanceClassIsShown(t *testing.T) {
	for row, spec := range appearanceRows {
		cells := appearanceCells(row)
		if len(cells) != len(spec.options) {
			t.Errorf("%s shows %d of %d classes", spec.label, len(cells), len(spec.options))
		}
		for i, cell := range cells {
			if cell.label != spec.options[i] {
				t.Errorf("%s cell %d is labelled %q, want %q", spec.label, i, cell.label, spec.options[i])
			}
		}
	}
}

// TestTheRulesDoNotRepeatTheWithdrawnClaims. Both were in the text and both
// are false: tolerance comes from an ability score rather than being global,
// and no per-node health drain was ever implemented.
func TestTheRulesDoNotRepeatTheWithdrawnClaims(t *testing.T) {
	prose := strings.ToLower(rulesProse(t))
	for _, phrase := range []string{
		// The hand-written action list, which named one that never existed.
		// Nothing hand-writes that list now: the strip is generated from
		// animation.AllAnimations, which is the actual protection.
		"attack, feed",
		"tolerance is global",
		"node costs",
		"per node",
		"coloured square",
		"colored square",
	} {
		if strings.Contains(prose, phrase) {
			t.Errorf("the rules still say %q", phrase)
		}
	}
}

// TestEveryBlockReportsTheHeightItDraws: the scroller advances by height(),
// so a block that paints taller than it reports overlaps the next one.
func TestEveryBlockReportsTheHeightItDraws(t *testing.T) {
	// height() measures wrapped text, so the fonts have to exist.
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	for i, b := range rulesContent() {
		if h := b.height(); h <= 0 {
			t.Errorf("block %d (%T) reports height %d", i, b, h)
		}
	}
}

// TestTheActionListFitsItsColumns: the names are centred under their
// sprites in a fixed column, so one wider than the column runs into its
// neighbour with nothing to clip it.
func TestTheActionListFitsItsColumns(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	if rulesListCols*rulesListColW > rulesPanelW {
		t.Fatalf("%d columns of %dpx overrun a %dpx panel", rulesListCols, rulesListColW, rulesPanelW)
	}
	for _, b := range rulesContent() {
		list, ok := b.(rulesActionList)
		if !ok {
			continue
		}
		want := (len(list.cells) + rulesListCols - 1) / rulesListCols
		if got := list.rows(); got != want {
			t.Errorf("the list is %d rows of %d for %d actions, want %d", got, rulesListCols, len(list.cells), want)
		}
		if (list.rows()-1)*rulesListCols >= len(list.cells) {
			t.Errorf("the last of %d rows holds nothing", list.rows())
		}
		for _, cell := range list.cells {
			w := boundString(r.FontSourceCodePro10, cell.label).Dx()
			if w > rulesListColW {
				t.Errorf("%q is %dpx against a %dpx column", cell.label, w, rulesListColW)
			}
		}
		drawn := rulesParaGap
		for row := 0; row < list.rows(); row++ {
			if list.cellHeight(row) < rulesListCellW {
				t.Errorf("row %d reserves %dpx for a %dpx sprite", row, list.cellHeight(row), rulesListCellW)
			}
			drawn += list.rowHeight(row)
		}
		if drawn != list.height() {
			t.Errorf("the list paints %dpx and reports %dpx", drawn, list.height())
		}
	}
}

// TestWrappedParagraphsHaveNoStraySpaces: the wrapper used to hand each word
// back with the space it was split on still attached and then join with
// another, which double-spaced every paragraph and indented every line after
// the first by one space.
func TestWrappedParagraphsHaveNoStraySpaces(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	for _, b := range rulesContent() {
		para, ok := b.(rulesText)
		if !ok {
			continue
		}
		lines := wrapParagraph(para.text, rulesPanelW)
		for i, line := range lines {
			if line != strings.TrimSpace(line) {
				t.Errorf("line %d has leading or trailing space: %q", i, line)
			}
			if strings.Contains(line, "  ") {
				t.Errorf("line %d has a double space: %q", i, line)
			}
			if w := boundString(r.FontSourceCodePro10, line).Dx(); w > rulesPanelW {
				t.Errorf("line %d is %dpx against a %dpx panel: %q", i, w, rulesPanelW, line)
			}
		}
		// Nothing is dropped or added by the wrap.
		if got, want := strings.Join(lines, " "), strings.Join(strings.Fields(para.text), " "); got != want {
			t.Errorf("wrapping changed the text:\n got %q\nwant %q", got, want)
		}
	}
}

// headingsOf is the section titles in one list of blocks.
func headingsOf(blocks []rulesBlock) []string {
	var out []string
	for _, b := range blocks {
		if h, ok := b.(rulesHeading); ok {
			out = append(out, h.text)
		}
	}
	return out
}

// TestEverySectionIsOnExactlyOneTab: the screen no longer scrolls through one
// document, so a section left out of every tab is one nothing can reach, and
// one on two tabs is written twice.
func TestEverySectionIsOnExactlyOneTab(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	want := map[string]string{
		"THE WORLD":                        "WORLD",
		"HOW pH CHANGES":                   "WORLD",
		"ORGANISM":                         "ORGANISMS",
		"WHAT EACH ABILITY BUYS":           "ORGANISMS",
		"HEALTH":                           "ORGANISMS",
		"ACTIONS, AND WHAT THEY LOOK LIKE": "ACTIONS",
		"APPEARANCE":                       "ACTIONS",
		"DECISION TREES":                   "ACTIONS",
		"LINEAGES AND THE REPLAY":          "VIEWER",
		"WHAT THE VIEWER SHOWS":            "VIEWER",
	}
	seen := map[string]string{}
	for _, tab := range aboutTabs() {
		for _, heading := range headingsOf(tab.blocks) {
			if strings.HasPrefix(heading, " ") {
				// The appearance sub-headings belong to APPEARANCE.
				continue
			}
			if first, dup := seen[heading]; dup {
				t.Errorf("%q is on both %s and %s", heading, first, tab.label)
			}
			seen[heading] = tab.label
		}
	}
	for heading, tab := range want {
		if got, ok := seen[heading]; !ok {
			t.Errorf("%q is on no tab", heading)
		} else if got != tab {
			t.Errorf("%q is on %s, want %s", heading, got, tab)
		}
	}
	for heading := range seen {
		if _, ok := want[heading]; !ok {
			t.Errorf("%q is a section this test does not place; add it", heading)
		}
	}
}

// TestTheHeaderStaysOutOfTheTabs: the summary and the organism crossing the
// page are drawn above the strip on every tab, so a copy inside a tab would
// render twice.
func TestTheHeaderStaysOutOfTheTabs(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	header := aboutHeader()
	if len(header) == 0 {
		t.Fatal("the header is empty")
	}
	if headings := headingsOf(header); len(headings) != 0 {
		t.Errorf("the header carries section headings: %v", headings)
	}
	var walkers int
	for _, b := range header {
		if _, ok := b.(rulesWalker); ok {
			walkers++
		}
	}
	for _, tab := range aboutTabs() {
		if len(tab.blocks) == 0 {
			t.Errorf("tab %q has no blocks", tab.label)
		}
		for _, b := range tab.blocks {
			if _, ok := b.(rulesWalker); ok {
				walkers++
			}
		}
	}
	if walkers != 1 {
		t.Errorf("the document holds %d walkers, want 1 in the header", walkers)
	}
}

// TestTheAppearanceGridFitsItsColumns: the four classes are hand-placed two
// across, so a title or a row of sprites wider than half the panel runs into
// the class beside it.
func TestTheAppearanceGridFitsItsColumns(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	var grids int
	for _, b := range rulesContent() {
		grid, ok := b.(rulesAppearanceGrid)
		if !ok {
			continue
		}
		grids++
		if got, want := len(grid.cells), len(appearanceRows); got != want {
			t.Errorf("the grid shows %d classes, want %d", got, want)
		}
		if got, want := grid.rows(), 2; got != want {
			t.Errorf("the grid is %d rows of %d, want %d", got, rulesAppCols, want)
		}
		for _, dim := range grid.cells {
			if w := boundString(r.FontSourceCodePro10, dim.title).Dx(); w > rulesAppColW {
				t.Errorf("the title %q is %dpx against a %dpx column", dim.title, w, rulesAppColW)
			}
			if w := appearanceGridWidth(len(dim.cells)); w > rulesAppColW {
				t.Errorf("%q needs %dpx of sprites against a %dpx column", dim.title, w, rulesAppColW)
			}
			for _, cell := range dim.cells {
				if w := boundString(r.FontSourceCodePro8, cell.label).Dx(); w > rulesAppCellW+rulesAppSpriteGap {
					t.Errorf("the class label %q is %dpx against a %dpx cell", cell.label, w, rulesAppCellW+rulesAppSpriteGap)
				}
			}
		}
		if drawn := rulesParaGap + grid.rows()*grid.rowHeight(); drawn != grid.height() {
			t.Errorf("the grid paints %dpx and reports %dpx", drawn, grid.height())
		}
	}
	if grids != 1 {
		t.Errorf("the document holds %d appearance grids, want 1", grids)
	}
}

// TestEveryTableRowFitsThePanel: a table draws its name at the left and its
// value at a fixed column, neither wrapped, so a long value runs off the
// panel with nothing to clip it.
func TestEveryTableRowFitsThePanel(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()
	for _, b := range rulesContent() {
		table, ok := b.(rulesTable)
		if !ok {
			continue
		}
		for _, row := range table.rows {
			if w := boundString(r.FontSourceCodePro10, row[0]).Dx(); w > table.nameW {
				t.Errorf("the name %q is %dpx against a %dpx column", row[0], w, table.nameW)
			}
			if w := boundString(r.FontSourceCodePro10, row[1]).Dx(); table.nameW+w > rulesPanelW {
				t.Errorf("%q needs %dpx and the panel is %dpx: %q", row[0], table.nameW+w, rulesPanelW, row[1])
			}
		}
	}
}
