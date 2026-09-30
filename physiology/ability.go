package physiology

import (
	"fmt"
	"sort"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/simrand"
)

// Ability indexes one of the scores every organism distributes a fixed budget across.
type Ability int

const (
	AbilityChemosynthesis Ability = iota
	AbilityEating
	AbilityMovement
	AbilityDigging
	AbilityAttack
	AbilityDefense
	// AbilityTolerance is how wide a band of pH an organism can sit in without taking damage.
	AbilityTolerance
	abilityCount
)

// AllAbilities lists every Ability in declaration order.
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

func (a Ability) Name() string { return abilityNames[a] }

// AbilityCount is how many abilities there are, for arrays indexed by Ability outside this package.
const AbilityCount = int(abilityCount)

// PointTotal is the budget every organism's scores must sum to.
const PointTotal = 20

// MaxAbilityScore is the most any single ability can hold, and the score every curve reaches its full effect at.
const MaxAbilityScore = 10

const SpecialistScore = 6

// BalancedScores returns the budget split as evenly as the integer scores allow, extra points going to the first abilities.
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

type Scores [abilityCount]int

// ScoresFromSlice builds a Scores from values in AllAbilities order, checking the budget invariant.
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

// RandomScores draws a random split of PointTotal across the abilities.
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

// spillOverCap moves anything above MaxAbilityScore onto the abilities with room, in ability order.
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

// InitialScores returns the ability distribution for one of the simulation's initial organisms.
func InitialScores(rng *simrand.RNG) Scores {
	if config.RandomInitialAbilities() {
		return RandomScores(rng)
	}
	if s, err := ScoresFromSlice(config.InitialAbilityScores()); err == nil {
		return s
	}
	return BalancedScores()
}

func (s Scores) Total() int {
	total := 0
	for _, a := range AllAbilities {
		total += s[a]
	}
	return total
}

// Validate reports an error if the invariant is broken.
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

// Mutated returns a copy of the scores with a single point transfer applied, or the scores unchanged if the mutation roll fails.
func (s Scores) Mutated(rng *simrand.RNG) Scores {
	if rng.Float64() >= config.ChanceToMutateAbilities() {
		return s
	}

	// Donors must have points to give.
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

	// Recipient is any OTHER ability with room under the cap.
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

	// One point, always: on an eleven-step scale a single point is already a tenth of an ability.
	out := s
	out[donor]--
	out[recipient]++
	return out
}
