package physiology

import (
	"fmt"
	"sort"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

// Ability indexes one of the scores every organism distributes a fixed
// budget across. Replaces the old feature trees: every organism can
// perform every action, and these scores decide how well and at what
// cost.
type Ability int

const (
	AbilityChemosynthesis Ability = iota
	AbilityEating
	AbilityMovement
	AbilityDigging
	AbilityAttack
	AbilityDefense
	// AbilityTolerance is how wide a band of pH an organism can sit in
	// without taking damage. Appended rather than slotted in beside
	// Defense: the iteration order below is load-bearing for replay
	// determinism.
	AbilityTolerance
	abilityCount
)

// AllAbilities lists every Ability in declaration order. Iteration order is
// load-bearing: mutation picks donors and recipients from it, so a
// reordering would change which organism mutates how from the same RNG
// stream and break replay determinism.
var AllAbilities = []Ability{
	AbilityChemosynthesis,
	AbilityEating,
	AbilityMovement,
	AbilityDigging,
	AbilityAttack,
	AbilityDefense,
	AbilityTolerance,
}

var abilityNames = [abilityCount]string{
	"Chemosynthesis", "Eating", "Movement", "Digging", "Attack", "Defense", "Tolerance",
}

// Name returns the human-readable ability name, used by the panel.
func (a Ability) Name() string { return abilityNames[a] }

// AbilityCount is how many abilities there are, for arrays indexed by
// Ability outside this package.
const AbilityCount = int(abilityCount)

// PointTotal is the budget every organism's scores must sum to. Fixed,
// so gaining ability in one area always costs ability in another —
// that tradeoff is the whole point of the scores replacing the trees.
const PointTotal = 20

// MaxAbilityScore is the most any single ability can hold, and the score
// every curve reaches its full effect at. It is deliberately less than
// PointTotal: the budget buys exactly two full specialisations, so a
// lineage picks a combination rather than pouring everything into one
// ability.
//
// The scale is deliberately coarse. Eleven steps per ability means a
// single mutation is a visible change in what an organism is, and the
// whole strategy space is small enough to explore in a run — where a
// 0-100 scale spent thousands of cycles drifting between scores that
// behaved identically.
const MaxAbilityScore = 10

// SpecialistScore is the score at which an ability counts as specialised
// — well past the ~29 an even split of the budget gives each ability,
// but short of the cap. For displays and checks that need a line: the
// ABILITY colour view shows it as fully green, and the reachability test
// counts organisms past it. It plays no part in how abilities work — those scale along
// their curves from 0 to MaxAbilityScore.
const SpecialistScore = 6

// BalancedScores returns the budget split as evenly as the integer scores
// allow, extra points going to the first abilities: [3 3 3 3 3 3 2].
// The fallback starting distribution when the configured initial scores
// are missing or invalid, and for records restored with no scores.
func BalancedScores() Scores {
	var s Scores
	base, extra := PointTotal/len(AllAbilities), PointTotal%len(AllAbilities)
	for i, a := range AllAbilities {
		s[a] = base
		if i < extra {
			s[a]++
		}
	}
	return s
}

// Scores is one organism's ability distribution. Invariant: the
// elements are between 0 and MaxAbilityScore and sum to PointTotal.
// Mutation preserves this structurally (it only ever moves points
// between entries, and never past the cap), and
// Validate is the assertion for paths that build a Scores from
// outside — notably snapshot restore.
type Scores [abilityCount]int

// ScoresFromSlice builds a Scores from values in AllAbilities order,
// checking the budget invariant. Used for distributions that come from
// outside the simulation, such as the configured initial scores.
func ScoresFromSlice(values []int) (Scores, error) {
	var s Scores
	if len(values) != len(AllAbilities) {
		return s, fmt.Errorf("physiology: %d ability scores given, want %d", len(values), len(AllAbilities))
	}
	for _, a := range AllAbilities {
		s[a] = values[a]
	}
	return s, s.Validate()
}

// RandomScores draws a random split of PointTotal across the abilities:
// sorted cut points on [0, PointTotal] divide the budget, then anything
// over MaxAbilityScore spills onto the abilities with room. Draws from
// rng in a fixed order, so a seeded simulation reproduces the same
// organisms.
func RandomScores(rng *simrand.RNG) Scores {
	cuts := make([]int, len(AllAbilities)-1)
	for i := range cuts {
		cuts[i] = rng.Intn(PointTotal + 1)
	}
	sort.Ints(cuts)
	var s Scores
	prev := 0
	for i, a := range AllAbilities {
		next := PointTotal
		if i < len(cuts) {
			next = cuts[i]
		}
		s[a] = next - prev
		prev = next
	}
	s.spillOverCap()
	return s
}

// spillOverCap moves anything above MaxAbilityScore onto the abilities
// with room, in ability order. The cut points divide the budget without
// knowing about the per-ability cap, and redrawing until they happen to
// respect it would consume a variable number of rng values — which would
// make the stream position depend on luck, and replays with it.
func (s *Scores) spillOverCap() {
	for _, a := range AllAbilities {
		excess := s[a] - MaxAbilityScore
		if excess <= 0 {
			continue
		}
		s[a] = MaxAbilityScore
		for _, b := range AllAbilities {
			if excess == 0 {
				break
			}
			room := MaxAbilityScore - s[b]
			if b == a || room <= 0 {
				continue
			}
			take := min(excess, room)
			s[b] += take
			excess -= take
		}
	}
}

// InitialScores returns the ability distribution for one of the
// simulation's initial organisms: a random split when
// RandomInitialAbilities is on, otherwise InitialAbilityScores, or the
// balanced distribution if that setting is missing or doesn't sum to the
// budget. Only the random path draws from rng, so the default setup's
// random sequence is unchanged.
func InitialScores(rng *simrand.RNG) Scores {
	if config.RandomInitialAbilities() {
		return RandomScores(rng)
	}
	if s, err := ScoresFromSlice(config.InitialAbilityScores()); err == nil {
		return s
	}
	return BalancedScores()
}

// Total returns the sum of all scores. Should always be PointTotal.
func (s Scores) Total() int {
	total := 0
	for _, a := range AllAbilities {
		total += s[a]
	}
	return total
}

// Validate reports an error if the invariant is broken: any entry
// outside [0, MaxAbilityScore], or a sum other than PointTotal. Called
// on snapshot restore so a corrupt or stale file fails loudly instead of
// silently running a sim where one lineage holds more ability points
// than everyone else.
func (s Scores) Validate() error {
	for _, a := range AllAbilities {
		if s[a] < 0 {
			return fmt.Errorf("physiology: %s score is negative (%d)", a.Name(), s[a])
		}
		if s[a] > MaxAbilityScore {
			return fmt.Errorf("physiology: %s score is %d, over the cap of %d", a.Name(), s[a], MaxAbilityScore)
		}
	}
	if total := s.Total(); total != PointTotal {
		return fmt.Errorf("physiology: scores sum to %d, want %d (%v)", total, PointTotal, s)
	}
	return nil
}

// Mutated returns a copy of the scores with a single point transfer
// applied, or the scores unchanged if the mutation roll fails.
//
// Transfers rather than independent per-score jitter because the budget
// has to stay exactly PointTotal. Moving points between two entries
// preserves the sum structurally — there is no renormalisation step,
// and therefore no rounding path that could drift a lineage off the
// invariant over thousands of generations.
//
// The RNG is consumed in a fixed order (roll, donor, recipient, amount)
// and only when the roll succeeds, so replay determinism depends on
// nothing but the stream itself.
func (s Scores) Mutated(rng *simrand.RNG) Scores {
	if rng.Float64() >= config.ChanceToMutateAbilities() {
		return s
	}

	// Donors must have points to give. Every score being zero is
	// impossible while PointTotal is positive, but guard anyway so a
	// future PointTotal of 0 can't deadlock the picker.
	donors := make([]Ability, 0, len(AllAbilities))
	for _, a := range AllAbilities {
		if s[a] > 0 {
			donors = append(donors, a)
		}
	}
	if len(donors) == 0 {
		return s
	}
	donor := donors[rng.Intn(len(donors))]

	// Recipient is any OTHER ability with room under the cap; building
	// the list without the donor keeps this a single rng draw instead of
	// a reject-and-retry loop, which would consume a variable number of
	// values and make the stream position depend on luck.
	recipients := make([]Ability, 0, len(AllAbilities)-1)
	for _, a := range AllAbilities {
		if a != donor && s[a] < MaxAbilityScore {
			recipients = append(recipients, a)
		}
	}
	if len(recipients) == 0 {
		return s
	}
	recipient := recipients[rng.Intn(len(recipients))]

	// One point, always: on an eleven-step scale a single point is
	// already a tenth of an ability, so a variable shift would let one
	// mutation redraw an organism entirely. The donor has at least one
	// point and the recipient has room (both were filtered above), so
	// the transfer always lands.
	out := s
	out[donor]--
	out[recipient]++
	return out
}
