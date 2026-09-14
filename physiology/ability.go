package physiology

import (
	"fmt"
	"math"
	"sort"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

// Ability indexes one of the six scores every organism distributes a
// fixed budget across. Replaces the old feature trees: every organism
// can perform every action, and these scores decide how well and at
// what cost.
type Ability int

const (
	AbilityChemosynthesis Ability = iota
	AbilityEating
	AbilityMovement
	AbilityDigging
	AbilityAttack
	AbilityDefense
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
}

var abilityNames = [abilityCount]string{
	"Chemosynthesis", "Eating", "Movement", "Digging", "Attack", "Defense",
}

// Name returns the human-readable ability name, used by the panel.
func (a Ability) Name() string { return abilityNames[a] }

// PointTotal is the budget every organism's scores must sum to. Fixed,
// so gaining ability in one area always costs ability in another —
// that tradeoff is the whole point of the scores replacing the trees.
const PointTotal = 100

// neutralScore returns the score at which an ability's multiplier is
// exactly 1.0. Each ability pivots on its OWN genesis allocation, not
// on the even split of the budget.
//
// The even split is the tempting choice and it is wrong here. Genesis
// organisms are deliberately lopsided — nearly all their points sit in
// chemosynthesis — so pivoting every curve on the even split would put
// a starting organism below neutral in five abilities at once and
// above it in only one. Measured, that was a population collapse: the
// sim died out at cycle 2438 on a seed that previously ran indefinitely.
//
// Pivoting on the genesis allocation instead makes a genesis organism
// exactly 1.0 across the board, which is precisely how the old
// featureless organism behaved. The scores then measure departure from
// the starting body plan: spend points out of chemosynthesis and its
// multiplier falls below 1 while the ability you fed rises above it.
// That is the trade the fixed budget is supposed to express.
func neutralScore(a Ability) float64 {
	return float64(GenesisScores()[a])
}

// Scores is one organism's ability distribution. Invariant: the
// elements are non-negative and sum to PointTotal. Mutation preserves
// this structurally (it only ever moves points between entries), and
// Validate is the assertion for paths that build a Scores from
// outside — notably snapshot restore.
type Scores [abilityCount]int

// GenesisScores returns the starting distribution for a genesis
// organism: heavily weighted toward chemosynthesis, minimal everywhere
// else. Genesis organisms are base chemosynthesizers that must evolve
// their way into every other capability, and under a fixed budget that
// evolution is a genuine trade — every point spent on moving or
// fighting is a point taken off self-feeding.
func GenesisScores() Scores {
	var s Scores
	minor := config.GenesisMinorAbilityScore()
	for _, a := range AllAbilities {
		s[a] = minor
	}
	s[AbilityChemosynthesis] = PointTotal - minor*(len(AllAbilities)-1)
	return s
}

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

// RandomScores draws a uniformly random split of PointTotal across the
// abilities: sorted cut points on [0, PointTotal] divide the budget, so
// every non-negative distribution is equally likely. Draws from rng in a
// fixed order, so a seeded simulation reproduces the same organisms.
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
	return s
}

// InitialScores returns the ability distribution for one of the
// simulation's initial organisms: a random split when
// RandomInitialAbilities is on, otherwise InitialAbilityScores, or the
// genesis distribution if that setting is missing or doesn't sum to the
// budget. Only the random path draws from rng, so the default setup's
// random sequence is unchanged.
func InitialScores(rng *simrand.RNG) Scores {
	if config.RandomInitialAbilities() {
		return RandomScores(rng)
	}
	if s, err := ScoresFromSlice(config.InitialAbilityScores()); err == nil {
		return s
	}
	return GenesisScores()
}

// SpecialistScore is the score at which an ability reaches its full
// multiplier: its genesis allocation plus the specialization span. It
// is the natural "fully specialised" line for anything that needs one —
// the ability colour view anchors green there, and the reachability
// test counts organisms past it.
func SpecialistScore(a Ability) int {
	return GenesisScores()[a] + config.AbilitySpecializationSpan()
}

// Total returns the sum of all scores. Should always be PointTotal.
func (s Scores) Total() int {
	total := 0
	for _, a := range AllAbilities {
		total += s[a]
	}
	return total
}

// Validate reports an error if the invariant is broken: any negative
// entry, or a sum other than PointTotal. Called on snapshot restore so
// a corrupt or stale file fails loudly instead of silently running a
// sim where one lineage holds more ability points than everyone else.
func (s Scores) Validate() error {
	for _, a := range AllAbilities {
		if s[a] < 0 {
			return fmt.Errorf("physiology: %s score is negative (%d)", a.Name(), s[a])
		}
	}
	if total := s.Total(); total != PointTotal {
		return fmt.Errorf("physiology: scores sum to %d, want %d (%v)", total, PointTotal, s)
	}
	return nil
}

// Multiplier maps a score onto the effect multiplier for its ability:
// two linear segments meeting at 1.0 exactly at neutralScore.
//
//	score 0          -> multAtZero   (config, per ability)
//	score neutral    -> 1.0          (always)
//	score PointTotal -> multAtMax    (config, per ability)
//
// Pivoting on a guaranteed 1.0 means an evenly-split organism behaves
// identically to the pre-scores featureless one, so the curve can be
// retuned without silently rebaselining the whole simulation.
//
// For cost-style abilities (movement, the cost half of digging) the
// endpoints invert — multAtZero is above 1 and multAtMax below — so a
// high score makes the action cheaper. A score of 0 is always playable,
// just expensive: the curve never divides by the score, so it has no
// singularity.
//
// Past the specialization span the curve keeps going linearly rather
// than clamping, so a lineage that keeps investing keeps gaining. For
// a damage-taken multiplier that eventually crosses zero; callers that
// apply it to damage must floor it (see manager.defenseDamageMult).
func (s Scores) Multiplier(a Ability) float64 {
	return multiplierFor(a, float64(s[a]))
}

// MultiplierAt returns the multiplier an organism with the given score in
// ability a would have. For calibrating against the curve at a reference
// score, where there is no whole Scores distribution to ask.
func MultiplierAt(a Ability, score int) float64 {
	return multiplierFor(a, float64(score))
}

func multiplierFor(a Ability, score float64) float64 {
	atZero, atMax := curveEndpoints(a)
	neutral := neutralScore(a)
	if score <= neutral {
		// Lower segment: atZero at score 0 -> 1.0 at neutral. The floor
		// is a real floor — an ability cannot go below zero — so the
		// full penalty lands exactly where the points run out.
		if neutral == 0 {
			return 1.0
		}
		return atZero + (1.0-atZero)*(score/neutral)
	}
	// Upper segment: 1.0 at neutral -> atMax a fixed SPAN of points
	// above it, and kept linear beyond so investment past the span
	// still pays off.
	//
	// The span is measured from the ability's own neutral rather than
	// running to PointTotal, and that is the whole point. Anchoring the
	// top at 100 made specialising cost (100 - genesis) points: 50 for
	// chemosynthesis but 90 for everything else, while dumping an
	// ability cost only its genesis allocation — 50 for chemo, 10 for
	// the rest. Non-chemo abilities were nine times harder to grow into
	// than to abandon, so the first points a lineage moved out of
	// chemosynthesis bought less than they cost and selection killed
	// every organism attempting the crossing. A fixed span makes the
	// investment symmetric across abilities: the same number of points
	// buys the same fraction of the payoff, wherever the ability
	// started, so a defense specialist is exactly as reachable as a
	// chemosynthesis one.
	span := float64(config.AbilitySpecializationSpan())
	if span <= 0 {
		return 1.0
	}
	progress := (score - neutral) / span
	if exp := curveExponent(a); exp != 1 && exp > 0 {
		// Shapes the climb without moving its ends: progress is still 0
		// at neutral and 1 at the span, so the curve still pivots on 1.0
		// at genesis and still reaches atMax a span above it. Below 1 is
		// concave — diminishing returns on stacking the ability.
		progress = math.Pow(progress, exp)
	}
	return 1.0 + (atMax-1.0)*progress
}

// curveExponent returns the shape exponent for an ability's upper curve
// segment. Only Chemosynthesis is configurable; the other abilities stay
// linear.
func curveExponent(a Ability) float64 {
	if a == AbilityChemosynthesis {
		return config.ChemoCurveExponent()
	}
	return 1
}

// curveEndpoints returns the (multiplier at score 0, multiplier at
// score PointTotal) pair for an ability. Read from config so the whole
// balance surface is tunable from default.json and the config screen
// without a rebuild.
func curveEndpoints(a Ability) (atZero, atMax float64) {
	switch a {
	case AbilityChemosynthesis:
		return config.ChemoMultAtZero(), config.ChemoMultAtMax()
	case AbilityEating:
		return config.EatingMultAtZero(), config.EatingMultAtMax()
	case AbilityMovement:
		return config.MovementMultAtZero(), config.MovementMultAtMax()
	case AbilityDigging:
		return config.DiggingMultAtZero(), config.DiggingMultAtMax()
	case AbilityAttack:
		return config.AttackMultAtZero(), config.AttackMultAtMax()
	case AbilityDefense:
		return config.DefenseMultAtZero(), config.DefenseMultAtMax()
	}
	return 1.0, 1.0
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

	// Recipient is any OTHER ability; building the list without the
	// donor keeps this a single rng draw instead of a reject-and-retry
	// loop, which would consume a variable number of values and make
	// the stream position depend on luck.
	recipients := make([]Ability, 0, len(AllAbilities)-1)
	for _, a := range AllAbilities {
		if a != donor {
			recipients = append(recipients, a)
		}
	}
	recipient := recipients[rng.Intn(len(recipients))]

	shift := 1 + rng.Intn(config.MaxAbilityShift())
	if shift > s[donor] {
		shift = s[donor]
	}

	out := s
	out[donor] -= shift
	out[recipient] += shift
	return out
}
