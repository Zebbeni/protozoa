package physiology

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/decision"
)

func plainScores() Scores {
	return Scores{8, 2, 2, 2, 2, 2, 2}
}

// scoresWith builds a valid distribution with one ability raised to `to`, taking the difference out of chemosynthesis so the budget still sums to PointTotal.
func scoresWith(t *testing.T, a Ability, to int) Scores {
	t.Helper()
	s := plainScores()
	delta := to - s[a]
	s[a] = to
	s[AbilityChemosynthesis] -= delta
	if err := s.Validate(); err != nil {
		t.Fatalf("test scores invalid: %v", err)
	}
	return s
}

// TestPlainScoresLookPlain: an organism sitting near the even split in every non-chemo ability has specialised in nothing.
func TestPlainScoresLookPlain(t *testing.T) {
	loadGlobals(t)

	app := AppearanceFor(plainScores(), nil)
	if app.Body != BodyBasic {
		t.Errorf("genesis body = %v, want BodyBasic", app.Body)
	}
	if app.Motor != MotorNone {
		t.Errorf("genesis motor = %v, want MotorNone", app.Motor)
	}
	if app.Mouth != MouthNone {
		t.Errorf("genesis mouth = %v, want MouthNone", app.Mouth)
	}
	if app.Sensor != SensorNone {
		t.Errorf("genesis sensor = %v, want SensorNone", app.Sensor)
	}
}

// TestBodyAndMotorThresholds: a shell comes from Tolerance and spikes from Defense, each past its own threshold.
func TestBodyAndMotorThresholds(t *testing.T) {
	loadGlobals(t)

	shell, spikes := config.ShellBodyThreshold(), config.SpikesBodyThreshold()
	for _, tc := range []struct {
		ability Ability
		score   int
		want    BodyClass
	}{
		{AbilityTolerance, shell - 1, BodyBasic},
		{AbilityTolerance, shell, BodyShell},
		{AbilityDefense, spikes - 1, BodyBasic},
		{AbilityDefense, spikes, BodySpikes},
	} {
		got := AppearanceFor(scoresWith(t, tc.ability, tc.score), nil).Body
		if got != tc.want {
			t.Errorf("%s %d: body = %v, want %v", tc.ability.Name(), tc.score, got, tc.want)
		}
	}

	// With both past their thresholds the higher score wins, and a tie goes to the shell.
	both := plainScores()
	hi, lo := max(shell, spikes)+1, spikes
	both[AbilityChemosynthesis] -= hi + lo - both[AbilityTolerance] - both[AbilityDefense]
	both[AbilityTolerance], both[AbilityDefense] = hi, lo
	if err := both.Validate(); err != nil {
		t.Fatalf("test scores invalid: %v", err)
	}
	if got := AppearanceFor(both, nil).Body; got != BodyShell {
		t.Errorf("tolerance %d over defense %d: body = %v, want BodyShell",
			both[AbilityTolerance], both[AbilityDefense], got)
	}
	both[AbilityTolerance], both[AbilityDefense] = both[AbilityDefense], both[AbilityTolerance]
	if got := AppearanceFor(both, nil).Body; got != BodySpikes {
		t.Errorf("defense %d over tolerance %d: body = %v, want BodySpikes",
			both[AbilityDefense], both[AbilityTolerance], got)
	}

	pili, flagella := config.PiliMotorThreshold(), config.FlagellaMotorThreshold()
	for _, tc := range []struct {
		score int
		want  MotorClass
	}{
		{pili - 1, MotorNone},
		{pili, MotorPili},
		{flagella - 1, MotorPili},
		{flagella, MotorFlagella},
	} {
		got := AppearanceFor(scoresWith(t, AbilityMovement, tc.score), nil).Motor
		if got != tc.want {
			t.Errorf("movement %d: motor = %v, want %v", tc.score, got, tc.want)
		}
	}
}

// TestMouthPicksDominantAbility covers the one place appearance has to choose between competing claims.
func TestMouthPicksDominantAbility(t *testing.T) {
	loadGlobals(t)

	s := plainScores()
	s[AbilityChemosynthesis] = 2
	s[AbilityEating] = 5
	s[AbilityAttack] = 9
	s[AbilityDigging] = 0
	s[AbilityMovement] = 2
	s[AbilityDefense] = 2
	s[AbilityTolerance] = 0
	if err := s.Validate(); err != nil {
		t.Fatalf("test scores invalid: %v", err)
	}

	if got := AppearanceFor(s, nil).Mouth; got != MouthFangs {
		t.Errorf("attack 9 should beat eating 5: mouth = %v, want MouthFangs", got)
	}

	// Flip the two and the mouth follows.
	s[AbilityEating], s[AbilityAttack] = s[AbilityAttack], s[AbilityEating]
	if got := AppearanceFor(s, nil).Mouth; got != MouthTeeth {
		t.Errorf("eating 9 should beat attack 5: mouth = %v, want MouthTeeth", got)
	}
}

func TestMouthTiesAreStable(t *testing.T) {
	loadGlobals(t)

	s := plainScores()
	s[AbilityChemosynthesis] = 4
	s[AbilityEating] = 5
	s[AbilityAttack] = 5
	s[AbilityDigging] = 5
	s[AbilityMovement] = 1
	s[AbilityDefense] = 0
	s[AbilityTolerance] = 0
	if err := s.Validate(); err != nil {
		t.Fatalf("test scores invalid: %v", err)
	}

	first := AppearanceFor(s, nil).Mouth
	for i := 0; i < 50; i++ {
		if got := AppearanceFor(s, nil).Mouth; got != first {
			t.Fatalf("tie resolved inconsistently: %v then %v", first, got)
		}
	}
	if first != MouthTeeth {
		t.Errorf("tie should resolve to the first declared class, got %v", first)
	}
}

// treeWith builds a tree whose conditions are the given ones, nested so every condition ends up as a real node.
func treeWith(conds ...decision.Condition) *decision.Tree {
	node := decision.NodeFromAction(decision.ActChemosynthesis)
	for _, c := range conds {
		node = &decision.Node{
			NodeType: c,
			YesNode:  node,
			NoNode:   decision.NodeFromAction(decision.ActIdle),
		}
	}
	return &decision.Tree{Node: node}
}

func TestSensorFollowsTreeConditions(t *testing.T) {
	loadGlobals(t)

	minimum := config.SensorMinConditions()
	genesis := plainScores()

	food := make([]decision.Condition, minimum)
	for i := range food {
		food[i] = decision.IsFoodAhead
	}
	if got := AppearanceFor(genesis, treeWith(food...)).Sensor; got != SensorAntennae {
		t.Errorf("%d food checks should give antennae, got %v", minimum, got)
	}

	// One short of the minimum earns nothing.
	if minimum > 1 {
		if got := AppearanceFor(genesis, treeWith(food[:minimum-1]...)).Sensor; got != SensorNone {
			t.Errorf("%d food checks is below the minimum but gave %v", minimum-1, got)
		}
	}

	walls := make([]decision.Condition, minimum)
	for i := range walls {
		walls[i] = decision.IsWallAhead
	}
	if got := AppearanceFor(genesis, treeWith(walls...)).Sensor; got != SensorFeelers {
		t.Errorf("wall checks should give feelers, got %v", got)
	}

	ph := make([]decision.Condition, minimum)
	for i := range ph {
		ph[i] = decision.IsHealthierPhAhead
	}
	if got := AppearanceFor(genesis, treeWith(ph...)).Sensor; got != SensorTasters {
		t.Errorf("pH checks should give tasters, got %v", got)
	}
}

func TestSelfChecksGrowNoSensors(t *testing.T) {
	loadGlobals(t)

	tree := treeWith(
		decision.IsHealthy,
		decision.IsHealthyPhHere,
		decision.CanChemosynthesizeHere,
		decision.IsAgeMultipleOfTwo,
		decision.IsAgeMultipleOfTen,
		decision.IsHealthy,
	)
	if got := AppearanceFor(plainScores(), tree).Sensor; got != SensorNone {
		t.Errorf("self-checks should need no sense organ, got %v", got)
	}
}

func TestSensorPicksDominantCategory(t *testing.T) {
	loadGlobals(t)

	tree := treeWith(
		decision.IsFoodAhead, decision.IsFoodLeft, decision.IsFoodRight, decision.IsFoodAhead,
		decision.IsWallAhead, decision.IsWallLeft,
	)
	if got := AppearanceFor(plainScores(), tree).Sensor; got != SensorAntennae {
		t.Errorf("4 food vs 2 wall checks should give antennae, got %v", got)
	}
}

// sensesWith pairs each condition with the pull given beside it.
func sensesWith(t *testing.T, conditions []decision.Condition, pulls []float64) []SensedCondition {
	t.Helper()
	if len(conditions) != len(pulls) {
		t.Fatalf("%d conditions against %d pulls", len(conditions), len(pulls))
	}
	out := make([]SensedCondition, len(conditions))
	for i, c := range conditions {
		out[i] = SensedCondition{Condition: c, Pull: pulls[i]}
	}
	return out
}

// TestAWeightedBrainFollowsWhereItsWeightIs is the difference between a net
// and a tree. A net reads every input it has on every cycle, so holding an
// input says nothing about how much of the brain is built on it; what says so
// is the weight leaving it. Four reads it barely uses lose to two it leans on.
func TestAWeightedBrainFollowsWhereItsWeightIs(t *testing.T) {
	loadGlobals(t)

	senses := sensesWith(t,
		[]decision.Condition{
			decision.IsFoodAhead, decision.IsFoodLeft, decision.IsFoodRight, decision.IsOrganismAhead,
			decision.IsWallAhead, decision.IsWallLeft,
		},
		[]float64{0.01, 0.01, 0.01, 0.01, 4.0, 4.0},
	)
	if got := AppearanceForSenses(plainScores(), senses).Sensor; got != SensorFeelers {
		t.Errorf("four reads at 0.01 against two at 4.0 gave %v, not the feelers the weight is in", got)
	}
	// Counting the same reads, which is what a tree does, answers the other
	// way — so the two rules are genuinely different and this is testing the
	// new one.
	counted := make([]decision.Condition, 0, len(senses))
	for _, s := range senses {
		counted = append(counted, s.Condition)
	}
	if got := AppearanceForConditions(plainScores(), counted).Sensor; got != SensorAntennae {
		t.Errorf("counting the same reads gave %v, so the test is not measuring the weighting", got)
	}
}

// TestScalingEveryWeightChangesNothing is the property that makes this a share
// and not a magnitude. A lineage's weights drift to whatever absolute size
// they drift to, with no ceiling anywhere, so an overlay handed out on raw
// magnitude would be handed out by that accident.
func TestScalingEveryWeightChangesNothing(t *testing.T) {
	loadGlobals(t)

	conditions := []decision.Condition{
		decision.IsFoodAhead, decision.IsFoodLeft, decision.IsWallAhead, decision.IsHealthy,
	}
	base := []float64{3, 2, 0.5, 1}
	want := AppearanceForSenses(plainScores(), sensesWith(t, conditions, base)).Sensor
	if want == SensorNone {
		t.Fatal("the sample grows no sensor, so scaling it proves nothing")
	}
	for _, factor := range []float64{0.001, 0.1, 10, 1000} {
		scaled := make([]float64, len(base))
		for i, p := range base {
			scaled[i] = p * factor
		}
		if got := AppearanceForSenses(plainScores(), sensesWith(t, conditions, scaled)).Sensor; got != want {
			t.Errorf("scaling every weight by %v changed the sensor from %v to %v", factor, want, got)
		}
	}
}

// TestAnInputTheBrainIgnoresGrowsNoSensor: a net can hold a read for
// generations with nothing built on it, since a connection arrives at exactly
// 0 and drift moves it slowly. Those should not dress the organism.
func TestAnInputTheBrainIgnoresGrowsNoSensor(t *testing.T) {
	loadGlobals(t)

	senses := sensesWith(t,
		[]decision.Condition{
			decision.IsFoodAhead, decision.IsFoodLeft, decision.IsFoodRight,
			decision.IsHealthy, decision.IsAgeMultipleOfTwo,
		},
		[]float64{0.001, 0.001, 0.001, 5, 5},
	)
	if got := AppearanceForSenses(plainScores(), senses).Sensor; got != SensorNone {
		t.Errorf("a brain built on its own health and age gave %v", got)
	}
}

// TestEqualPullsCountTheWayATreeDoes keeps the two brains comparable: with
// nothing to tell its reads apart, a weighted brain is read exactly as a
// counted one, which is what `sensor_min_conditions` still means.
func TestEqualPullsCountTheWayATreeDoes(t *testing.T) {
	loadGlobals(t)

	sets := [][]decision.Condition{
		{decision.IsFoodAhead, decision.IsFoodLeft},
		{decision.IsFoodAhead},
		{decision.IsWallAhead, decision.IsWallLeft, decision.IsFoodAhead},
		{decision.IsHealthierPhAhead, decision.IsHealthierPhAhead, decision.IsHealthy},
		{decision.IsHealthy, decision.IsAgeMultipleOfTen},
		nil,
	}
	for _, conditions := range sets {
		pulls := make([]float64, len(conditions))
		for i := range pulls {
			pulls[i] = 7 // any one value, since only the shares matter
		}
		want := AppearanceForConditions(plainScores(), conditions).Sensor
		if got := AppearanceForSenses(plainScores(), sensesWith(t, conditions, pulls)).Sensor; got != want {
			t.Errorf("%v: weighted evenly gave %v where counting gives %v", conditions, got, want)
		}
	}
}

// TestABrainThatLeansOnNothingIsReadByItsReads: every connection a net is
// given starts at exactly 0, so a founder's whole brain pulls nothing. There
// is no share to take then, and the reads themselves are what is left.
func TestABrainThatLeansOnNothingIsReadByItsReads(t *testing.T) {
	loadGlobals(t)

	conditions := []decision.Condition{decision.IsFoodAhead, decision.IsFoodLeft, decision.IsWallAhead}
	zeroed := sensesWith(t, conditions, []float64{0, 0, 0})
	want := AppearanceForConditions(plainScores(), conditions).Sensor
	if got := AppearanceForSenses(plainScores(), zeroed).Sensor; got != want {
		t.Errorf("a brain with every weight still at 0 gave %v, not the %v its reads give", got, want)
	}
	if got := AppearanceForSenses(plainScores(), nil).Sensor; got != SensorNone {
		t.Errorf("a brain with no reads at all gave %v", got)
	}
}
