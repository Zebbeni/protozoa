package physiology

import (
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/decision"
)

// Appearance is what an organism looks like, derived rather than inherited.
type Appearance struct {
	Body   BodyClass
	Motor  MotorClass
	Mouth  MouthClass
	Sensor SensorClass
}

// BodyClass is the body silhouette: a shell for pH tolerance, spikes for Defense.
type BodyClass int

const (
	BodyBasic BodyClass = iota
	BodyShell
	BodySpikes
)

type MotorClass int

const (
	MotorNone MotorClass = iota
	MotorPili
	MotorFlagella
)

// MouthClass is the feeding / terrain overlay, from whichever of Eating, Attack or Digging an organism has invested in most.
type MouthClass int

const (
	MouthNone MouthClass = iota
	MouthTeeth
	MouthFangs
	MouthTusks
)

// SensorClass is the sensory overlay, from the decision tree's conditions rather than from any score.
type SensorClass int

const (
	SensorNone     SensorClass = iota
	SensorAntennae             // food and organism presence
	SensorFeelers              // walls and relative size
	SensorTasters              // pH
)

// AppearanceFor derives an organism's look from its scores and its decision tree.
func AppearanceFor(scores Scores, tree *decision.Tree) Appearance {
	return Appearance{
		Body:   bodyClassFor(scores),
		Motor:  motorClassFor(scores),
		Mouth:  mouthClassFor(scores),
		Sensor: sensorClassFor(tree),
	}
}

// bodyClassFor picks the body an organism has most earned.
func bodyClassFor(s Scores) BodyClass {
	best, bestScore := BodyBasic, 0

	if v := s[AbilityTolerance]; v >= config.ShellBodyThreshold() && v > bestScore {
		best, bestScore = BodyShell, v
	}
	if v := s[AbilityDefense]; v >= config.SpikesBodyThreshold() && v > bestScore {
		best, bestScore = BodySpikes, v
	}
	return best
}

func motorClassFor(s Scores) MotorClass {
	switch m := s[AbilityMovement]; {
	case m >= config.FlagellaMotorThreshold():
		return MotorFlagella
	case m >= config.PiliMotorThreshold():
		return MotorPili
	default:
		return MotorNone
	}
}

// mouthClassFor picks the single mouth overlay an organism has most earned.
func mouthClassFor(s Scores) MouthClass {
	best, bestScore := MouthNone, 0

	if v := s[AbilityEating]; v >= config.TeethMouthThreshold() && v > bestScore {
		best, bestScore = MouthTeeth, v
	}
	if v := s[AbilityAttack]; v >= config.FangsMouthThreshold() && v > bestScore {
		best, bestScore = MouthFangs, v
	}
	if v := s[AbilityDigging]; v >= config.TusksMouthThreshold() && v > bestScore {
		best, bestScore = MouthTusks, v
	}
	return best
}

func sensorCategoryOf(c decision.Condition) SensorClass {
	switch c {
	case decision.IsFoodAhead, decision.IsFoodLeft, decision.IsFoodRight,
		decision.IsOrganismAhead, decision.IsOrganismLeft, decision.IsOrganismRight,
		decision.IsRelativeAhead,
		// The coarse flank reads sense a neighbouring cell without distinguishing what is in it.
		decision.IsSomethingLeft, decision.IsSomethingRight:
		return SensorAntennae
	case decision.IsWallAhead, decision.IsWallLeft, decision.IsWallRight,
		decision.IsBiggerOrganismAhead:
		return SensorFeelers
	case decision.IsHealthierPhAhead:
		return SensorTasters
	default:
		return SensorNone
	}
}

// sensorClassFor returns the overlay for whichever sense the tree leans on hardest, or SensorNone when nothing clears the minimum.
func sensorClassFor(tree *decision.Tree) SensorClass {
	if tree == nil {
		return SensorNone
	}

	var counts [4]int
	for _, c := range tree.ConditionNodes() {
		counts[sensorCategoryOf(c)]++
	}

	best, bestCount := SensorNone, config.SensorMinConditions()-1
	for _, class := range []SensorClass{SensorAntennae, SensorFeelers, SensorTasters} {
		if counts[class] > bestCount {
			best, bestCount = class, counts[class]
		}
	}
	return best
}
