package manager

import (
	"math"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

func (s *gridStub) AddOrganismUpdate(utils.Point) {}

// predationSetup puts an attacker and a victim in a manager with the
// attacker's hit already queued, and AttackHealthGain set to gain.
func predationSetup(t *testing.T, gain, attack, victimHealth float64) (*OrganismManager, *organism.Organism, *organism.Organism) {
	t.Helper()
	loadDefaultGlobals(t)
	config.GetCurrentGlobals().AttackHealthGain = gain

	attacker := &organism.Organism{ID: 1, Location: utils.Point{X: 10, Y: 10}, Size: 50, Health: 10}
	victim := &organism.Organism{ID: 2, Location: utils.Point{X: 11, Y: 10}, Size: 50, Health: victimHealth}
	m := &OrganismManager{
		api:            &gridStub{},
		organismIDGrid: initializeGrid(),
		organisms:      map[int]*organism.Organism{1: attacker, 2: victim},
	}
	m.requestManager.ClearMaps()
	m.requestManager.AddAttackRequest(victim.Location, attack, attacker.ID)
	return m, attacker, victim
}

// TestPredationFeedsAttacker: the attacker gains AttackHealthGain of the
// damage that landed.
func TestPredationFeedsAttacker(t *testing.T) {
	m, attacker, victim := predationSetup(t, 0.5, -4, 50)
	landed := 4 * defenseDamageMult(victim.Abilities())

	m.applyCycleHealthChanges(victim)

	if want := 10 + 0.5*landed; math.Abs(attacker.Health-want) > 1e-9 {
		t.Errorf("attacker health %.3f, want %.3f", attacker.Health, want)
	}
}

// TestPredationCapsAtVictimHealth: an attack far bigger than the victim
// feeds only on the health the victim had.
func TestPredationCapsAtVictimHealth(t *testing.T) {
	m, attacker, victim := predationSetup(t, 1, -500, 6)
	m.applyCycleHealthChanges(victim)
	if want := 10.0 + 6; math.Abs(attacker.Health-want) > 1e-9 {
		t.Errorf("attacker health %.3f, want %.3f (capped at the victim's 6 health)", attacker.Health, want)
	}
}

// TestPredationOffByDefault: at the default gain, attacking feeds nothing.
func TestPredationOffByDefault(t *testing.T) {
	m, attacker, victim := predationSetup(t, 0, -4, 50)
	m.applyCycleHealthChanges(victim)
	if attacker.Health != 10 {
		t.Errorf("attacker health changed to %.3f with predation off", attacker.Health)
	}
}

// TestShovesDoNotFeed: only attacks are predatory.
func TestShovesDoNotFeed(t *testing.T) {
	m, attacker, victim := predationSetup(t, 1, -4, 50)
	m.requestManager.ClearMaps()
	m.requestManager.AddHealthEffectRequest(victim.Location, -4, attacker.ID)
	m.applyCycleHealthChanges(victim)
	if attacker.Health != 10 {
		t.Errorf("a shove fed the attacker: health %.3f", attacker.Health)
	}
}

// TestChemoCrowdingFactor: each occupied neighbour costs ChemoCrowdingPenalty
// of chemosynthesis, floored at zero, and it is off by default.
func TestChemoCrowdingFactor(t *testing.T) {
	loadDefaultGlobals(t)
	o := &organism.Organism{ID: 1, Location: utils.Point{X: 10, Y: 10}}
	m := &OrganismManager{api: &gridStub{}, organismIDGrid: initializeGrid()}
	m.organismIDGrid[10][9] = 2
	m.organismIDGrid[11][10] = 3

	if f := m.chemoCrowdingFactor(o); f != 1 {
		t.Errorf("crowding off by default should give 1, got %.2f", f)
	}
	config.GetCurrentGlobals().ChemoCrowdingPenalty = 0.2
	if f := m.chemoCrowdingFactor(o); math.Abs(f-0.6) > 1e-9 {
		t.Errorf("two neighbours at 0.2 each: factor %.2f, want 0.60", f)
	}
	config.GetCurrentGlobals().ChemoCrowdingPenalty = 0.9
	if f := m.chemoCrowdingFactor(o); f != 0 {
		t.Errorf("factor should floor at 0, got %.2f", f)
	}
}
