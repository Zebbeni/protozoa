package physiology

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/decision"
)

// scoresWith builds a valid distribution with one ability raised to
// `to`, taking the difference out of chemosynthesis so the budget still
// sums to PointTotal.
func scoresWith(t *testing.T, a Ability, to int) Scores {
	t.Helper()
	s := GenesisScores()
	delta := to - s[a]
	s[a] = to
	s[AbilityChemosynthesis] -= delta
	if err := s.Validate(); err != nil {
		t.Fatalf("test scores invalid: %v", err)
	}
	return s
}

// TestGenesisLooksPlain is the property that keeps appearance honest:
// a newborn has specialised in nothing, so it should wear nothing.
// Any threshold set at or below GenesisMinorAbilityScore would hand
// every organism in the world an overlay for free, which reads as "the
// thresholds aren't working" rather than as a config mistake.
func TestGenesisLooksPlain(t *testing.T) {
	loadGlobals(t)

	app := AppearanceFor(GenesisScores(), nil)
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

func TestBodyAndMotorThresholds(t *testing.T) {
	loadGlobals(t)

	shell, spikes := config.ShellBodyThreshold(), config.SpikesBodyThreshold()
	for _, tc := range []struct {
		score int
		want  BodyClass
	}{
		{shell - 1, BodyBasic},
		{shell, BodyShell},
		{spikes - 1, BodyShell},
		{spikes, BodySpikes},
	} {
		got := AppearanceFor(scoresWith(t, AbilityDefense, tc.score), nil).Body
		if got != tc.want {
			t.Errorf("defense %d: body = %v, want %v", tc.score, got, tc.want)
		}
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

// TestMouthPicksDominantAbility covers the one place appearance has to
// choose between competing claims: the renderer draws a single mouth,
// but Eating, Attack and Digging each want one.
func TestMouthPicksDominantAbility(t *testing.T) {
	loadGlobals(t)

	s := GenesisScores()
	s[AbilityChemosynthesis] = 10
	s[AbilityEating] = 25
	s[AbilityAttack] = 45
	s[AbilityDigging] = 0
	s[AbilityMovement] = 10
	s[AbilityDefense] = 10
	if err := s.Validate(); err != nil {
		t.Fatalf("test scores invalid: %v", err)
	}

	if got := AppearanceFor(s, nil).Mouth; got != MouthFangs {
		t.Errorf("attack 45 should beat eating 25: mouth = %v, want MouthFangs", got)
	}

	// Flip the two and the mouth follows.
	s[AbilityEating], s[AbilityAttack] = s[AbilityAttack], s[AbilityEating]
	if got := AppearanceFor(s, nil).Mouth; got != MouthTeeth {
		t.Errorf("eating 45 should beat attack 25: mouth = %v, want MouthTeeth", got)
	}
}

// TestMouthTiesAreStable pins the tie-break. Equal scores must resolve
// to the same class every time — an appearance that alternated between
// two sprites on equal scores would look like a rendering glitch, and
// nothing would point at the tie as the cause.
func TestMouthTiesAreStable(t *testing.T) {
	loadGlobals(t)

	s := GenesisScores()
	s[AbilityChemosynthesis] = 20
	s[AbilityEating] = 25
	s[AbilityAttack] = 25
	s[AbilityDigging] = 25
	s[AbilityMovement] = 5
	s[AbilityDefense] = 0
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

// treeWith builds a tree whose conditions are the given ones, nested so
// every condition ends up as a real node.
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

// TestSensorFollowsTreeConditions covers the half of appearance that
// comes from behaviour rather than from scores.
func TestSensorFollowsTreeConditions(t *testing.T) {
	loadGlobals(t)

	minimum := config.SensorMinConditions()
	genesis := GenesisScores()

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

// TestSelfChecksGrowNoSensors guards the classification boundary.
// Health, age and own-cell pH need no sense organ, so a tree full of
// them must stay bare — otherwise every organism sprouts antennae for
// introspecting.
func TestSelfChecksGrowNoSensors(t *testing.T) {
	loadGlobals(t)

	tree := treeWith(
		decision.IsHealthAboveFiftyPercent,
		decision.IsHealthyPhHere,
		decision.CanChemosynthesizeHere,
		decision.IsAgeMultipleOfTwo,
		decision.IsAgeMultipleOfTen,
		decision.IsHealthAboveFiftyPercent,
	)
	if got := AppearanceFor(GenesisScores(), tree).Sensor; got != SensorNone {
		t.Errorf("self-checks should need no sense organ, got %v", got)
	}
}

// TestSensorPicksDominantCategory: counting every occurrence rather
// than distinct conditions is what makes "leans on this sense"
// meaningful, so a tree that checks food repeatedly should read as
// food-focused even against a broader spread of wall checks.
func TestSensorPicksDominantCategory(t *testing.T) {
	loadGlobals(t)

	tree := treeWith(
		decision.IsFoodAhead, decision.IsFoodLeft, decision.IsFoodRight, decision.IsFoodAhead,
		decision.IsWallAhead, decision.IsWallLeft,
	)
	if got := AppearanceFor(GenesisScores(), tree).Sensor; got != SensorAntennae {
		t.Errorf("4 food vs 2 wall checks should give antennae, got %v", got)
	}
}
