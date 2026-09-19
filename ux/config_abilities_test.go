package ux

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
)

func abilityConfigScreen(t *testing.T) (*ConfigScreen, *config.Globals) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	config.SetGlobals(&g)
	defaults := g
	defaults.InitialAbilityScores = append([]int(nil), g.InitialAbilityScores...)
	loadConfigDefaults = func() config.Globals { return defaults }
	form := g // what the popup edits: a copy of the active globals
	return NewConfigScreen(&form), &g
}

func TestInitialAbilitiesBlockStartUntilTheyTotalTheBudget(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	if reason := cs.StartBlockedReason(); reason != "" {
		t.Fatalf("default scores should be startable, got %q", reason)
	}

	cs.adjustAbilityScore(int(physiology.AbilityAttack), 2)
	if cs.StartBlockedReason() == "" {
		t.Errorf("a total of %d should block the start", physiology.PointTotal+2)
	}
	cs.adjustAbilityScore(int(physiology.AbilityChemosynthesis), -2)
	if reason := cs.StartBlockedReason(); reason != "" {
		t.Errorf("rebalanced to %d should start, got %q", physiology.PointTotal, reason)
	}

	cs.adjustAbilityScore(int(physiology.AbilityDefense), -1)
	cs.globals.RandomInitialAbilities = true
	if reason := cs.StartBlockedReason(); reason != "" {
		t.Errorf("Random ignores the fixed scores, got %q", reason)
	}
}

func TestInitialAbilityScoresClamp(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	cs.adjustAbilityScore(int(physiology.AbilityEating), -1000)
	cs.adjustAbilityScore(int(physiology.AbilityMovement), 1000)
	if got := cs.globals.InitialAbilityScores[physiology.AbilityEating]; got != 0 {
		t.Errorf("score below 0 should clamp to 0, got %d", got)
	}
	if got := cs.globals.InitialAbilityScores[physiology.AbilityMovement]; got != physiology.MaxAbilityScore {
		t.Errorf("score above 100 should clamp to 100, got %d", got)
	}
}

// TestConfigScreenDoesNotEditActiveScores: the form's globals are a copy,
// but a slice copies by reference; editing must not reach the active
// config before Start.
func TestConfigScreenDoesNotEditActiveScores(t *testing.T) {
	cs, active := abilityConfigScreen(t)
	before := active.InitialAbilityScores[physiology.AbilityAttack]
	cs.adjustAbilityScore(int(physiology.AbilityAttack), 7)
	if active.InitialAbilityScores[physiology.AbilityAttack] != before {
		t.Error("editing the form changed the active config's ability scores")
	}
}

// TestOversizedFounderBlocksStart: a ticked design whose decision tree
// is over the configured limit holds the start, naming the design and
// both ways out. Starting anyway would either silently drop it or run an
// organism with a tree the simulation's own mutation limit forbids —
// which would out-compete every evolved tree for a reason no setting
// explains.
func TestOversizedFounderBlocksStart(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	dir := t.TempDir()

	// A five-node design, then a limit of four.
	ds := organism.NewDesign("branchy")
	root := d.NodeFromCondition(d.IsFoodAhead)
	root.YesNode = d.NodeFromCondition(d.IsWallAhead)
	root.YesNode.YesNode = d.NodeFromAction(d.ActDig)
	root.YesNode.NoNode = d.NodeFromAction(d.ActEat)
	root.NoNode = d.NodeFromAction(d.ActMove)
	ds.DecisionTree = d.TreeFromNode(root).Serialize()
	if _, err := organism.SaveDesign(dir, ds); err != nil {
		t.Fatal(err)
	}

	// The running simulation's limit is left low throughout, and never
	// touched: what the screen judges designs against is the value on the
	// screen. The two disagreeing is the normal case, since an edited
	// limit isn't installed until a run starts.
	config.GetCurrentGlobals().MaxDecisionTreeSize = 4

	cs.globals.MaxDecisionTreeSize = 4
	cs.globals.InitialDesigns = []string{"branchy"}
	reason := cs.startBlockedByDesigns(organism.LoadDesigns(dir))
	if !strings.Contains(reason, "branchy") || !strings.Contains(reason, "limit") {
		t.Errorf("start should be blocked naming the design, got %q", reason)
	}

	// Raising the limit on the screen clears it there and then. This used
	// to read the active config, so the row went on reporting "5 nodes >
	// 4 limit" however high the user set Max Tree Size — cancelling and
	// reopening didn't help either, since the edit is never installed.
	cs.globals.MaxDecisionTreeSize = 5
	if reason := cs.startBlockedByDesigns(organism.LoadDesigns(dir)); reason != "" {
		t.Errorf("raising the limit on the screen should clear the block, got %q", reason)
	}
	if over := organism.OversizedDesigns(organism.LoadDesigns(dir), cs.treeLimit()); len(over) != 0 {
		t.Errorf("at the limit, %v was flagged", over)
	}

	cs.globals.MaxDecisionTreeSize = 4
	over := organism.OversizedDesigns(organism.LoadDesigns(dir), cs.treeLimit())
	if len(over) != 1 || over[0] != "branchy" {
		t.Fatalf("over the limit, flagged %v, want branchy", over)
	}

	cs.globals.InitialDesigns = nil
	if reason := cs.startBlockedByDesigns(organism.LoadDesigns(dir)); reason != "" {
		t.Errorf("an unticked oversized design shouldn't block the start, got %q", reason)
	}
}

// TestNoEmptyConfigSections: a section header that expands to nothing is
// a dead end — it reads as a category whose settings have gone missing.
// The TERRAIN section became one when the wall and dig settings moved
// into the Digging ability block, and nothing pointed at it.
func TestNoEmptyConfigSections(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	for _, section := range cs.sections {
		if len(section.fields) == 0 {
			t.Errorf("%s has no settings in it", section.title)
		}
	}
}
