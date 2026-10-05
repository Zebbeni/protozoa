package decision

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

func loadDefaults(t *testing.T) *config.Globals {
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
	return config.GetCurrentGlobals()
}

func TestNewConditionsShipDisabled(t *testing.T) {
	loadDefaults(t)
	// Only what disabled_decision_nodes holds back. The refined reads are
	// governed by basic_only_condition_families now, and the shipped
	// settings leave every family open — so naming one here would be
	// asserting a balance choice rather than the hold-out rule. The rule
	// itself is TestOnlyBasicReadsAreOfferedToANewNode below.
	for _, c := range []Condition{
		IsHealthAboveTwentyPercent,
		IsAgeMultipleOfTwo, IsAgeMultipleOfTen,
	} {
		if IsNodeEnabled(c) {
			t.Errorf("%s is offered to mutation by default", Names[c])
		}
	}
	for _, c := range []Condition{
		CanMove, IsFoodAhead, IsOrganismAhead, IsWallAhead,
		CanChemosynthesizeHere, IsRelativeAhead, IsHealthyPhHere,
		// These two are the deliberate exception to "new conditions ship disabled".
		IsFoodHere, IsFoodBuriedHere,
	} {
		if !IsNodeEnabled(c) {
			t.Errorf("%s was held back; it has always been available", Names[c])
		}
	}
}

func TestDisabledNodesStillDeserialize(t *testing.T) {
	loadDefaults(t)

	var found bool
	for _, c := range Conditions {
		if c == IsPhTooLowHere {
			found = true
		}
	}
	if !found {
		t.Error("a disabled condition fell out of the registration table; old trees would fail to load")
	}
	if Map[IsPhTooLowHere] == "" {
		t.Error("a disabled condition has no label, so an old tree would render blank")
	}
}

func TestEnabledPoolsKeepDeclarationOrder(t *testing.T) {
	g := loadDefaults(t)
	g.DisabledDecisionNodes = []string{"IsFoodLeft", "ActTurnRight"}
	// Cleared as well, or the shipped basic-only families take twelve advanced reads out of the pool and this measures those instead of the ordering it is about.
	g.BasicOnlyConditionFamilies = nil
	config.SetGlobals(g)

	var wantC []Condition
	for _, c := range MutableConditions {
		if c != IsFoodLeft {
			wantC = append(wantC, c)
		}
	}
	got := EnabledConditions()
	if len(got) != len(wantC) {
		t.Fatalf("pool has %d conditions, want %d", len(got), len(wantC))
	}
	for i := range got {
		if got[i] != wantC[i] {
			t.Fatalf("condition %d is %s, want %s", i, Names[got[i]], Names[wantC[i]])
		}
	}
	for _, a := range EnabledActions() {
		if a == ActTurnRight {
			t.Error("a disabled action is still in the pool")
		}
	}
}

func TestEnabledPoolsFollowASettingsChange(t *testing.T) {
	g := loadDefaults(t)
	if !IsNodeEnabled(IsFoodAhead) {
		t.Fatal("IsFoodAhead should start enabled")
	}
	g.DisabledDecisionNodes = []string{"IsFoodAhead"}
	config.SetGlobals(g)
	if IsNodeEnabled(IsFoodAhead) {
		t.Error("the pool kept a node the settings just disabled")
	}
	if !IsNodeEnabled(IsAgeMultipleOfTwo) {
		t.Error("the pool kept holding back a node the settings just released")
	}
}

func TestEmptyPoolFallsBackRatherThanPanicking(t *testing.T) {
	g := loadDefaults(t)
	var all []string
	for _, v := range MutableNodeTypes() {
		all = append(all, Names[v])
	}
	g.DisabledDecisionNodes = all
	config.SetGlobals(g)

	if len(EnabledActions()) == 0 {
		t.Error("disabling everything left an empty action pool; mutation would panic")
	}
	if len(EnabledConditions()) == 0 {
		t.Error("disabling everything left an empty condition pool; mutation would panic")
	}
}

// TestEveryMutableTypeHasANameAndALabel: the settings key on Names and the config screen shows Map.
func TestEveryMutableTypeHasANameAndALabel(t *testing.T) {
	seen := map[string]bool{}
	for _, v := range MutableNodeTypes() {
		name := Names[v]
		if name == "" {
			t.Errorf("%v has no settings name", v)
			continue
		}
		if seen[name] {
			t.Errorf("two node types share the settings name %q", name)
		}
		seen[name] = true
		if Map[v] == "" {
			t.Errorf("%s has no display label", name)
		}
	}
}

func TestDefaultDisabledNodesAllExist(t *testing.T) {
	known := map[string]bool{}
	for _, v := range MutableNodeTypes() {
		known[Names[v]] = true
	}
	for _, name := range config.DefaultDisabledNodes {
		if !known[name] {
			t.Errorf("DefaultDisabledNodes holds back %q, which is not a mutable node type", name)
		}
	}
}

func TestNoConditionDuplicatesAPairOfOthers(t *testing.T) {
	loadDefaults(t)

	// Gone, and must not come back as a node type.
	for _, name := range []string{"CanBurrowAhead"} {
		for _, v := range MutableNodeTypes() {
			if Names[v] == name {
				t.Errorf("%s is back in the pool; IsWallAhead + CanMove already answers it", name)
			}
		}
	}

	// These survived the bar: each reads something no combination of the others gives.
	for _, c := range []Condition{
		IsBiggerOrganismLeft, IsBiggerOrganismRight,
		IsRelativeLeft, IsRelativeRight,
		IsPhTooLowHere, IsPhTooHighHere,
		IsHealthAboveTwentyPercent,
	} {
		if Names[c] == "" {
			t.Errorf("%v lost its settings name", c)
		}
		if Map[c] == "" {
			t.Errorf("%s lost its label", Names[c])
		}
	}
}

func TestOnlyBasicReadsAreOfferedToANewNode(t *testing.T) {
	loadDefaults(t)
	g := *config.GetCurrentGlobals()
	g.TieredConditionMutation = true
	config.SetGlobals(&g)

	pool := BasicConditions()
	if len(pool) == 0 {
		t.Fatal("the basic pool is empty, so this proves nothing")
	}
	for _, c := range pool {
		if !IsBasicCondition(c) {
			t.Errorf("%s is offered to a new condition node but is a refinement",
				Names[c])
		}
	}
	for _, c := range []Condition{
		IsBiggerOrganismLeft, IsBiggerOrganismRight,
		IsMuchBiggerOrganismAhead, IsVeryHealthy, IsVeryUnhealthy,
		IsOrganismLeft, IsWallLeft, IsFoodLeft,
	} {
		if contains(pool, c) {
			t.Errorf("%s is in the pool a new condition node draws from", Names[c])
		}
	}
}
