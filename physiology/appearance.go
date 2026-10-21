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

// SensedCondition is one read a brain makes and how hard that brain leans on
// it. A decision tree leans by asking: every node weighs the same and a
// condition asked down three branches is three of them. A net leans by weight,
// since it reads every input on every cycle and presence alone would say
// nothing.
type SensedCondition struct {
	Condition decision.Condition
	Pull      float64
}

// AppearanceFor derives an organism's look from its scores and its decision tree.
func AppearanceFor(scores Scores, tree *decision.Tree) Appearance {
	return AppearanceForConditions(scores, treeConditions(tree))
}

// AppearanceForConditions is the derivation for a brain whose reads all weigh
// the same, which is what a decision tree's nodes are.
func AppearanceForConditions(scores Scores, conditions []decision.Condition) Appearance {
	senses := make([]SensedCondition, 0, len(conditions))
	for _, c := range conditions {
		senses = append(senses, SensedCondition{Condition: c, Pull: 1})
	}
	return AppearanceForSenses(scores, senses)
}

// AppearanceForSenses is the same derivation with each read carrying its own
// weight, which is what a net's inputs do.
//
// Sensing is a behaviour rather than a score, so the overlay follows what the
// organism reads — and which conditions a brain reads is a question both
// brains can answer, where "walk the tree" is not.
func AppearanceForSenses(scores Scores, senses []SensedCondition) Appearance {
	return Appearance{
		Body:   bodyClassFor(scores),
		Motor:  motorClassFor(scores),
		Mouth:  mouthClassFor(scores),
		Sensor: sensorClassFor(senses),
	}
}

// treeConditions is nil-safe, because Restore is handed a tree a snapshot may
// not carry.
func treeConditions(tree *decision.Tree) []decision.Condition {
	if tree == nil {
		return nil
	}
	return tree.ConditionNodes()
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

// sensorClassFor returns the overlay for whichever sense the brain leans on
// hardest, or SensorNone when nothing clears the minimum.
//
// Each read counts for its share of the average read rather than for its raw
// pull, so the threshold is the same number of reads it has always been:
// `sensor_min_conditions` is "this sense pulls as hard as two average reads".
// Scaling every weight in a net changes nothing, which is the property that
// matters — a lineage's weights drift to whatever absolute size they drift to,
// so a raw magnitude threshold would hand out overlays by that accident.
//
// Reads that need no sense organ are in the average but win nothing, so a
// brain built mostly on its own health and age grows no sensors, the same as
// a tree full of self-checks.
func sensorClassFor(senses []SensedCondition) SensorClass {
	if len(senses) == 0 {
		return SensorNone
	}
	total := 0.0
	for _, s := range senses {
		total += s.Pull
	}
	// A brain that leans on nothing at all — every connection still at the 0
	// it was introduced with, which is every founder — has no share to take,
	// so its reads are counted instead. That is what it had before any weight
	// could matter.
	even := total <= 0
	if even {
		total = float64(len(senses))
	}

	var shares [4]float64
	for _, s := range senses {
		pull := s.Pull
		if even {
			pull = 1
		}
		shares[sensorCategoryOf(s.Condition)] += pull
	}

	average := total / float64(len(senses))
	best, bestCount := SensorNone, float64(config.SensorMinConditions())-1
	for _, class := range []SensorClass{SensorAntennae, SensorFeelers, SensorTasters} {
		if n := shares[class] / average; n > bestCount {
			best, bestCount = class, n
		}
	}
	return best
}
