package manager

import (
	"math"
	"testing"

	d "github.com/Zebbeni/protozoa/decision"

	c "github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// predationWorld puts an attacker beside a defender, both of the given size.
func predationWorld(t *testing.T, gain float64, defenderHealth float64) (*OrganismManager, *organism.Organism, *organism.Organism) {
	t.Helper()
	loadDefaultGlobals(t)
	g := c.GetCurrentGlobals()
	g.AttackHealthGain = gain
	g.ChemoCrowdingPenalty = 0
	c.SetGlobals(g)

	atkScores := physiology.Scores{}
	atkScores[physiology.AbilityAttack] = physiology.MaxAbilityScore
	atkScores[physiology.AbilityTolerance] = physiology.PointTotal - physiology.MaxAbilityScore
	// A defenceless target, so the damage lands whole.
	defScores := physiology.Scores{}
	defScores[physiology.AbilityTolerance] = physiology.PointTotal

	api := &phRecorder{ph: 5}
	m := &OrganismManager{api: api, organismIDGrid: initializeGrid(),
		organisms: map[int]*organism.Organism{}}
	// A hand-built manager has no request maps until the cycle clears them.
	m.requestManager.ClearMaps()

	at := utils.Point{X: 10, Y: 10}
	target := at.Add(utils.Point{X: 1, Y: 0})
	mk := func(id int, p utils.Point, health float64, scores physiology.Scores, act d.Action) *organism.Organism {
		o := organism.Restore(id, 1, health, 20, 0, 0, 0, p, utils.Point{X: 1, Y: 0}, id,
			organism.Traits{IdealPh: 5, Abilities: scores, MaxSize: 100}, nil,
			act, organism.StatusIdle, 0, 0, 0, 0, nil)
		m.organisms[id] = o
		m.organismIDGrid[p.X][p.Y] = id
		m.organismIds = append(m.organismIds, id)
		return o
	}
	attacker := mk(1, at, 20, atkScores, d.ActAttack)
	defender := mk(2, target, defenderHealth, defScores, d.ActChemosynthesis)
	return m, attacker, defender
}

// hit queues the attacker's blow and resolves it on the defender.
func hit(m *OrganismManager, attacker, defender *organism.Organism) {
	m.requestManager.AddAttackRequest(defender.Location,
		m.calculateAttackEffect(attacker), attacker.ID)
	m.applyCycleHealthChanges(defender)
}

// TestAKillPaysItsKiller is the point: with the gain off, killing feeds the
// attacker nothing and the reward is a corpse whoever eats it collects — so
// a predator needs Attack AND Eating AND to be standing on the body, where a
// chemosynthesiser needs one ability and no positioning.
func TestAKillPaysItsKiller(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0.5, 20)
	before := attacker.Health

	hit(m, attacker, defender)

	if gained := attacker.Health - before; gained <= 0 {
		t.Errorf("the attacker gained %v from a kill; it should feed on what it took", gained)
	}
}

// TestPredationPaysOnHealthRemovedNotDamageDealt: a full-Attack hit is worth
// many times its target's whole health, so paying on the nominal damage would
// make one blow on a runt worth more than the runt ever was.
func TestPredationPaysOnHealthRemovedNotDamageDealt(t *testing.T) {
	const share = 0.5
	m, attacker, defender := predationWorld(t, share, 20)
	dealt := -m.calculateAttackEffect(attacker)
	if dealt <= defender.Health {
		t.Fatalf("this test needs overkill: %v damage against %v health", dealt, defender.Health)
	}
	before := attacker.Health

	hit(m, attacker, defender)

	gained := attacker.Health - before
	if want := 20 * share; math.Abs(gained-want) > 1e-9 {
		t.Errorf("a hit taking 20 health paid %v, want %v; overkill must earn nothing extra",
			gained, want)
	}
}

// TestPredationOffChangesNothing is the compatibility claim: a settings file
// written before this decodes the share to 0, and a kill then pays only the
// corpse, exactly as before.
func TestPredationOffChangesNothing(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0, 20)
	before := attacker.Health
	hit(m, attacker, defender)
	if attacker.Health != before {
		t.Errorf("with the gain off the attacker's health moved by %v", attacker.Health-before)
	}
}

// TestTwoAttackersSplitOneKill: the health is only there once, so two
// attackers landing on the same target in one cycle are paid for their halves
// rather than each for a whole kill.
func TestTwoAttackersSplitOneKill(t *testing.T) {
	const share = 1.0
	m, first, defender := predationWorld(t, share, 20)

	second := organism.Restore(3, 1, 20, 20, 0, 0, 0,
		defender.Location.Add(utils.Point{X: 1, Y: 0}), utils.Point{X: -1, Y: 0}, 3,
		organism.Traits{IdealPh: 5, Abilities: first.Abilities(), MaxSize: 100}, nil,
		d.ActAttack, organism.StatusIdle, 0, 0, 0, 0, nil)
	m.organisms[3] = second

	beforeA, beforeB := first.Health, second.Health
	m.requestManager.AddAttackRequest(defender.Location, m.calculateAttackEffect(first), first.ID)
	m.requestManager.AddAttackRequest(defender.Location, m.calculateAttackEffect(second), second.ID)
	m.applyCycleHealthChanges(defender)

	total := (first.Health - beforeA) + (second.Health - beforeB)
	if want := 20 * share; math.Abs(total-want) > 1e-9 {
		t.Errorf("two attackers took %v between them from a 20-health target, want %v",
			total, want)
	}
}

// killCycle runs the hit and then the end-of-cycle claim, which is where a
// killer takes its victim's cell.
func killCycle(m *OrganismManager, attacker, defender *organism.Organism) {
	hit(m, attacker, defender)
	m.markDyingIfDead(defender)
	m.organismIds = []int{attacker.ID, defender.ID}
	m.applyKillClaims()
}

// attackScores distributes the budget into Attack, with the remainder in an
// ability that does nothing to a fight.
func attackScores(attack int) physiology.Scores {
	s := physiology.Scores{}
	s[physiology.AbilityAttack] = attack
	s[physiology.AbilityTolerance] = physiology.PointTotal - attack
	return s
}

// place puts an organism into the manager, replacing whatever held the id.
func place(m *OrganismManager, id int, p utils.Point, scores physiology.Scores, act d.Action) *organism.Organism {
	o := organism.Restore(id, 1, 20, 20, 0, 0, 0, p, utils.Point{X: 1, Y: 0}, id,
		organism.Traits{IdealPh: 5, Abilities: scores, MaxSize: 100}, nil,
		act, organism.StatusIdle, 0, 0, 0, 0, nil)
	m.organisms[id] = o
	m.organismIDGrid[p.X][p.Y] = id
	return o
}

// TestAKillerTakesTheCellInTheSameCycle is the timing the mechanic promises:
// the body keeps rendering at the cell so its death animation plays, but it
// gives up the grid square immediately, so the killer does not wait a cycle.
func TestAKillerTakesTheCellInTheSameCycle(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0, 20)
	corpse := defender.Location
	from := attacker.Location

	killCycle(m, attacker, defender)

	if attacker.Location != corpse {
		t.Errorf("the killer is at %v, want the victim's cell %v", attacker.Location, corpse)
	}
	if got := m.organismIDGrid[corpse.X][corpse.Y]; got != attacker.ID {
		t.Errorf("the grid holds %d at the victim's cell, want the killer %d", got, attacker.ID)
	}
	if got := m.organismIDGrid[from.X][from.Y]; got != -1 {
		t.Errorf("the cell the killer left still holds %d", got)
	}
	// The body is still in the population, at the same cell, for its
	// animation; only the grid square changed hands.
	if _, alive := m.organisms[defender.ID]; !alive {
		t.Error("the body was removed in the kill cycle; its death animation cannot play")
	}
	if defender.Location != corpse {
		t.Errorf("the body moved to %v", defender.Location)
	}
}

// TestTheKillerEndsStandingOnTheCorpse: a kill pays in the body it leaves,
// and eating acts on the cell the organism is standing on, so the corpse has
// to be under the killer or the reward is someone else's.
func TestTheKillerEndsStandingOnTheCorpse(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0, 20)
	corpse := defender.Location
	want := corpseValue(defender)

	killCycle(m, attacker, defender)

	api := m.api.(*phRecorder)
	if got := api.corpses[corpse]; got != want {
		t.Errorf("the corpse at %v is worth %d, want %d", corpse, got, want)
	}
	if attacker.Location != corpse {
		t.Fatalf("the killer is at %v, not on the corpse at %v", attacker.Location, corpse)
	}
	// finalizeDeaths must not drop a second corpse on the cell the killer
	// now occupies, where the no-food-on-an-organism guard would refuse it.
	m.finalizeDeaths()
	if got := api.corpses[corpse]; got != want {
		t.Errorf("after finalizeDeaths the corpse is worth %d, want %d still", got, want)
	}
	if got := m.organismIDGrid[corpse.X][corpse.Y]; got != attacker.ID {
		t.Errorf("finalizeDeaths cleared the cell to %d, erasing the killer %d", got, attacker.ID)
	}
}

// TestTheCellGoesToTheStrongestAttacker: several attackers can land on one
// organism in a cycle and only one can have the cell. The strong one here
// carries the higher id, so ID order would give the opposite answer.
func TestTheCellGoesToTheStrongestAttacker(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0, 20)
	corpse := defender.Location

	weak := place(m, attacker.ID, attacker.Location, attackScores(1), d.ActAttack)
	strong := place(m, 3, corpse.Add(utils.Point{X: 1, Y: 0}),
		attackScores(physiology.MaxAbilityScore), d.ActAttack)

	m.requestManager.AddAttackRequest(corpse, m.calculateAttackEffect(weak), weak.ID)
	m.requestManager.AddAttackRequest(corpse, m.calculateAttackEffect(strong), strong.ID)
	m.applyCycleHealthChanges(defender)
	m.markDyingIfDead(defender)
	m.organismIds = []int{weak.ID, defender.ID, 3}
	m.applyKillClaims()

	if strong.Location != corpse {
		t.Errorf("the strongest attacker is at %v, want the victim's cell %v", strong.Location, corpse)
	}
	if weak.Location == corpse {
		t.Error("the weaker attacker took the cell")
	}
}

// TestAKillerThatWanderedOffDoesNotTeleport: the claim is re-checked rather
// than trusted, since nothing guarantees the attacker is still beside the
// body by the time the claim runs.
func TestAKillerThatWanderedOffDoesNotTeleport(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0, 20)
	corpse := defender.Location

	hit(m, attacker, defender)
	m.markDyingIfDead(defender)
	far := utils.Point{X: 2, Y: 2}
	m.organismIDGrid[attacker.Location.X][attacker.Location.Y] = -1
	attacker.Location = far
	m.organismIDGrid[far.X][far.Y] = attacker.ID

	m.organismIds = []int{attacker.ID, defender.ID}
	m.applyKillClaims()

	if attacker.Location != far {
		t.Errorf("a killer that wandered off was moved to %v", attacker.Location)
	}
	if got := m.organismIDGrid[corpse.X][corpse.Y]; got != defender.ID {
		t.Errorf("the victim's cell reads %d; an unclaimed body keeps its square until finalizeDeaths", got)
	}
}

// TestABodyNobodyKilledKeepsItsCell: only an attack that finished an
// organism hands its cell over. Starving beside an attacker does not.
func TestABodyNobodyKilledKeepsItsCell(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0, 20)
	corpse := defender.Location

	defender.Health = 0
	m.markDyingIfDead(defender)
	if defender.KilledBy != -1 {
		t.Fatalf("the defender names killer %d though nothing attacked it", defender.KilledBy)
	}
	m.organismIds = []int{attacker.ID, defender.ID}
	m.applyKillClaims()

	if attacker.Location == corpse {
		t.Error("an organism that died on its own handed its cell to a neighbour")
	}
}

// TestAnAttackIntoOpenWaterCarriesTheAttacker: an attack aimed at nothing is
// a lunge, not a swing at empty space.
func TestAnAttackIntoOpenWaterCarriesTheAttacker(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0, 20)
	// Clear the cell ahead so the attack has nothing to hit.
	m.organismIDGrid[defender.Location.X][defender.Location.Y] = -1
	delete(m.organisms, defender.ID)
	ahead := attacker.Location.Add(attacker.Direction)

	m.requestManager.ClearMaps()
	m.updateRequestMapTo(attacker, &m.requestManager)
	m.applyAttack(attacker)

	if attacker.Location != ahead {
		t.Errorf("the attacker is at %v, want the open cell %v it lunged into",
			attacker.Location, ahead)
	}
	if got := m.organismIDGrid[ahead.X][ahead.Y]; got != attacker.ID {
		t.Errorf("the grid holds %d at %v, want the attacker %d", got, ahead, attacker.ID)
	}
	if attacker.Status != organism.StatusAttackMove {
		t.Errorf("the lunge ended on status %d, want StatusAttackMove (%d)",
			attacker.Status, organism.StatusAttackMove)
	}
}

// TestAnAttackOnAnOrganismDoesNotMove: the attacker stays put and the blow
// lands. Moving would walk it into a cell something alive is still in.
func TestAnAttackOnAnOrganismDoesNotMove(t *testing.T) {
	m, attacker, _ := predationWorld(t, 0, 20)
	where := attacker.Location

	m.requestManager.ClearMaps()
	m.updateRequestMapTo(attacker, &m.requestManager)
	m.applyAttack(attacker)

	if attacker.Location != where {
		t.Errorf("the attacker moved to %v onto the organism it attacked", attacker.Location)
	}
	if attacker.Status != organism.StatusAttacking {
		t.Errorf("the attack ended on status %d, want StatusAttacking (%d)",
			attacker.Status, organism.StatusAttacking)
	}
}

// TestAnAttackDoesNotOpenAWall: burrowing belongs to ActMove, and an attack
// that could break terrain would make Digging's only route through a wall
// reachable from Attack as well.
func TestAnAttackDoesNotOpenAWall(t *testing.T) {
	m, attacker, defender := predationWorld(t, 0, 20)
	m.organismIDGrid[defender.Location.X][defender.Location.Y] = -1
	delete(m.organisms, defender.ID)
	ahead := attacker.Location.Add(attacker.Direction)
	api := m.api.(*phRecorder)
	api.walls = map[utils.Point]int{ahead: 1}
	where := attacker.Location

	m.requestManager.ClearMaps()
	m.updateRequestMapTo(attacker, &m.requestManager)
	m.applyAttack(attacker)

	if attacker.Location != where {
		t.Errorf("the attacker lunged into a wall cell at %v", attacker.Location)
	}
	if api.walls[ahead] != 1 {
		t.Errorf("the wall at %v is now %d; an attack removed it", ahead, api.walls[ahead])
	}
}

// TestTwoAttackersCannotLungeIntoOneCell: the lunge claims its cell the way
// a move does, so the usual largest-wins arbitration applies.
func TestTwoAttackersCannotLungeIntoOneCell(t *testing.T) {
	m, first, defender := predationWorld(t, 0, 20)
	target := defender.Location
	m.organismIDGrid[target.X][target.Y] = -1
	delete(m.organisms, defender.ID)

	// A second attacker on the far side, facing back into the same cell.
	second := place(m, 4, target.Add(utils.Point{X: 1, Y: 0}), attackScores(1), d.ActAttack)
	second.Direction = utils.Point{X: -1, Y: 0}

	m.requestManager.ClearMaps()
	m.updateRequestMapTo(first, &m.requestManager)
	m.updateRequestMapTo(second, &m.requestManager)
	m.applyAttack(first)
	m.applyAttack(second)

	in := 0
	for _, o := range []*organism.Organism{first, second} {
		if o.Location == target {
			in++
		}
	}
	if in != 1 {
		t.Errorf("%d attackers ended in the same cell %v, want 1", in, target)
	}
}
