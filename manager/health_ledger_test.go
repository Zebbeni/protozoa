package manager

import (
	d "github.com/Zebbeni/protozoa/decision"
	"testing"

	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// sourcesIn names the ledger lines that moved, so a test can state both what
// was charged and what was not.
func sourcesIn(l organism.HealthLedger) map[string]bool {
	out := map[string]bool{}
	for _, src := range organism.AllHealthSources {
		if l.Amounts[src] != 0 {
			out[src.Label()] = true
		}
	}
	return out
}

func wantSources(t *testing.T, l organism.HealthLedger, want ...string) {
	t.Helper()
	got := sourcesIn(l)
	for _, w := range want {
		if !got[w] {
			t.Errorf("the ledger charged nothing to %q; it holds %v", w, got)
		}
		delete(got, w)
	}
	for extra := range got {
		t.Errorf("the ledger charged %q, which this cycle should not have", extra)
	}
}

// TestChemosynthesisIsChargedToItsOwnLine: the gain is the whole point of the
// cycle, so it cannot be lumped in with what the cycle cost.
func TestChemosynthesisIsChargedToItsOwnLine(t *testing.T) {
	m, o, _ := chemoOrganism(t, physiology.MaxAbilityScore, 5)
	o.ResetHealthLedger()
	m.applyCycleHealthChanges(o)
	m.applyChemosynthesis(o)
	l := o.HealthLedger()
	if l.Amounts[organism.HealthFromChemo] <= 0 {
		t.Errorf("chemo recorded %v at its ideal pH, want a gain", l.Amounts[organism.HealthFromChemo])
	}
	// At exactly its ideal pH the water costs nothing, so nothing else moved.
	wantSources(t, l, "chemo")
}

// TestBadWaterIsChargedToPhNotToTheAction: pH damage arrives whatever the
// organism chose to do, which is exactly why it reads as its own line.
func TestBadWaterIsChargedToPhNotToTheAction(t *testing.T) {
	m, o, _ := chemoOrganism(t, 1, 9)
	o.ResetHealthLedger()
	m.applyCycleHealthChanges(o)
	m.applyChemosynthesis(o)
	l := o.HealthLedger()
	if l.Amounts[organism.HealthFromPh] >= 0 {
		t.Errorf("ph recorded %v in water 4 off ideal, want a loss", l.Amounts[organism.HealthFromPh])
	}
	// A failed attempt costs health and is still charged to chemo.
	if l.Amounts[organism.HealthFromChemo] >= 0 {
		t.Errorf("a failed attempt recorded %v, want a loss", l.Amounts[organism.HealthFromChemo])
	}
	wantSources(t, l, "ph", "chemo")
}

func TestAnActionCostIsChargedToTheActionLine(t *testing.T) {
	m, o, _ := chemoOrganism(t, 1, 5)
	o.ResetHealthLedger()
	m.applyRightTurn(o)
	l := o.HealthLedger()
	if l.Amounts[organism.HealthFromAction] >= 0 {
		t.Errorf("a turn recorded %v, want a cost", l.Amounts[organism.HealthFromAction])
	}
	wantSources(t, l, "act")
}

// TestDamageTakenIsChargedSeparately: a health bar dropping for no reason of
// the organism's own is what the "hit" line answers.
func TestDamageTakenIsChargedSeparately(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0, 2000)
	defender.ResetHealthLedger()
	attacker.ResetHealthLedger()
	hit(m, attacker, defender)
	if got := defender.HealthLedger().Amounts[organism.HealthFromAttack]; got >= 0 {
		t.Errorf("being hit recorded %v, want a loss", got)
	}
}

// TestThornsAreNotChargedToTheHitLine: thorns are damage for ATTACKING
// something, not for being attacked, and over 3,000 cycles they were 7,050
// of the 17,037 non-zero hit lines — so sharing a line with incoming damage
// reads as "something attacked me" to an organism nothing attacked.
func TestThornsAreNotChargedToTheHitLine(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0, 2000)
	// A defender with real Defense, so its thorns answer back.
	defScores := physiology.Scores{}
	defScores[physiology.AbilityDefense] = physiology.MaxAbilityScore
	defScores[physiology.AbilityTolerance] = physiology.PointTotal - physiology.MaxAbilityScore
	defender = place(m, defender.ID, defender.Location, defScores, d.ActChemosynthesis)
	defender.ResetHealthLedger()
	attacker.ResetHealthLedger()

	hit(m, attacker, defender)

	l := attacker.HealthLedger()
	if l.Amounts[organism.HealthFromThorns] >= 0 {
		t.Errorf("the attacker's thorns line reads %v, want a loss", l.Amounts[organism.HealthFromThorns])
	}
	if l.Amounts[organism.HealthFromAttack] != 0 {
		t.Errorf("the attacker's hit line reads %v though nothing attacked it",
			l.Amounts[organism.HealthFromAttack])
	}
}

// TestPredationIsChargedToPrey: an attacker pays a cost and may be fed by the
// same blow, and one number cannot tell those apart.
func TestPredationIsChargedToPrey(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0.5, 20)
	attacker.ResetHealthLedger()
	hit(m, attacker, defender)
	if got := attacker.HealthLedger().Amounts[organism.HealthFromPredation]; got <= 0 {
		t.Errorf("a kill fed the attacker %v on the prey line, want a gain", got)
	}
}

// TestACreditedAttackerKeepsItsLedgerLine pins WHERE the ledger is cleared.
//
// Thorns and predation are recorded on the ATTACKER while the DEFENDER
// resolves, so clearing inside the resolve loop wipes whichever of the two
// comes later by id — the attacker here carries the higher one. The clear
// belongs in the decide phase, which runs for every organism before any of
// them resolve, and this test stands in for that by clearing both ledgers
// itself and then resolving.
func TestACreditedAttackerKeepsItsLedgerLine(t *testing.T) {
	m, weak, defender := predationWorld(t, 0.5, 20)
	// A second attacker with a higher id than its victim, which is the case
	// the resolve-loop clear got wrong.
	attacker := place(m, 3, defender.Location.Add(utils.Point{X: 1, Y: 0}),
		attackScores(physiology.MaxAbilityScore), d.ActAttack)
	m.organismIds = []int{weak.ID, defender.ID, 3}
	if attacker.ID <= defender.ID {
		t.Fatalf("the attacker's id %d is not above the defender's %d", attacker.ID, defender.ID)
	}

	m.requestManager.ClearMaps()
	m.requestManager.AddAttackRequest(defender.Location, m.calculateAttackEffect(attacker), attacker.ID)
	// What the decide phase does for every live organism, before any resolve.
	for _, o := range m.organisms {
		o.ResetHealthLedger()
	}

	m.resolveOrganismActions()

	if got := attacker.HealthLedger().Amounts[organism.HealthFromPredation]; got <= 0 {
		t.Errorf("the attacker's prey line reads %v after resolving; its own resolve cleared "+
			"the credit the defender's resolve had just recorded", got)
	}
}
