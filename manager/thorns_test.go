package manager

import (
	"math"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// TestThornsDependOnlyOnDefender pins the formula: the configured damage
// at 100 Defense, along the Thorns curve, per unit of defender size, and
// nothing at all without Defense.
func TestThornsDependOnlyOnDefender(t *testing.T) {
	loadDefaultGlobals(t)

	if d := thornsDamage(0, 50); d != 0 {
		t.Errorf("with no Defense thorns should deal nothing, got %.3f", d)
	}
	// The setting is a positive damage magnitude; what lands on the
	// attacker is the health change, so the expected value is negated.
	full := -config.ThornsDamageAtFullDefense() * 50
	if d := thornsDamage(physiology.MaxAbilityScore, 50); math.Abs(d-full) > 1e-9 {
		t.Errorf("full Defense, size 50: %.3f, want the full %.3f", d, full)
	}
	mid := thornsDamage(physiology.MaxAbilityScore/2, 50)
	if !(mid < 0 && mid > full) {
		t.Errorf("half Defense should deal something short of the full amount: %.3f vs %.3f", mid, full)
	}
	if bigger := thornsDamage(physiology.MaxAbilityScore, 100); math.Abs(bigger-2*full) > 1e-9 {
		t.Errorf("double the size should double thorns: %.3f, want %.3f", bigger, 2*full)
	}
}

// thornsSetup queues one hit of the given amount from an attacker on a
// defender, and returns the manager and both organisms.
func thornsSetup(t *testing.T, amount float64) (*OrganismManager, *organism.Organism, *organism.Organism) {
	t.Helper()
	loadDefaultGlobals(t)
	attacker := &organism.Organism{ID: 1, Location: utils.Point{X: 10, Y: 10}, Size: 50, Health: 50}
	defender := &organism.Organism{ID: 2, Location: utils.Point{X: 11, Y: 10}, Size: 60, Health: 60}
	m := &OrganismManager{
		api:            &gridStub{},
		organismIDGrid: initializeGrid(),
		organisms:      map[int]*organism.Organism{1: attacker, 2: defender},
	}
	m.requestManager.ClearMaps()
	m.requestManager.AddAttackRequest(defender.Location, amount, attacker.ID)
	return m, attacker, defender
}

// TestThornsIgnoreAttackStrength is the property the rework exists for: a
// light hit and a massive one draw exactly the same thorns damage.
func TestThornsIgnoreAttackStrength(t *testing.T) {
	const defense = 80
	var taken []float64
	for _, amount := range []float64{-1, -1000} {
		m, attacker, defender := thornsSetup(t, amount)
		m.applyThorns(defender, m.requestManager.GetHealthEffects(defender.Location)[0], defense)
		taken = append(taken, 50-attacker.Health)
	}
	if taken[0] <= 0 {
		t.Fatalf("an armoured defender should hurt its attacker, took %.3f", taken[0])
	}
	if math.Abs(taken[0]-taken[1]) > 1e-9 {
		t.Errorf("thorns differ with attack strength: %.3f from a light hit, %.3f from a heavy one", taken[0], taken[1])
	}
	if want := -thornsDamage(defense, 60); math.Abs(taken[0]-want) > 1e-9 {
		t.Errorf("attacker took %.3f, want %.3f", taken[0], want)
	}
}

// TestThornsOnlyAnswerDamage: effects that aren't damage, or have no
// organism behind them, never trigger thorns.
func TestThornsOnlyAnswerDamage(t *testing.T) {
	const defense = 80
	m, attacker, defender := thornsSetup(t, -5)
	for _, hit := range []HealthEffect{
		{Amount: 5, SourceID: attacker.ID},
		{Amount: -5, SourceID: -1},
		{Amount: -5, SourceID: defender.ID},
	} {
		m.applyThorns(defender, hit, defense)
	}
	if attacker.Health != 50 {
		t.Errorf("thorns fired on a non-damage or sourceless effect: attacker health %.3f", attacker.Health)
	}
}
