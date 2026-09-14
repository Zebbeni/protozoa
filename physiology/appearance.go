package physiology

import (
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/decision"
)

// Appearance is what an organism looks like, derived rather than
// inherited. Nothing here is a trait: the body silhouette and the
// motor and mouth overlays are read off the ability scores, and the
// sensor overlay is read off what the decision tree actually tests
// for. An organism therefore looks like what it does.
//
// Lives in this package because both organism (which computes it) and
// resources (which maps it to sprite layers) already import physiology,
// while resources -> animation -> organism means organism can never
// import resources directly.
type Appearance struct {
	Body   BodyClass
	Motor  MotorClass
	Mouth  MouthClass
	Sensor SensorClass
}

// BodyClass is the body silhouette, from the Defense score.
type BodyClass int

const (
	BodyBasic BodyClass = iota
	BodyShell
	BodySpikes
)

// MotorClass is the locomotion overlay, from the Movement score.
type MotorClass int

const (
	MotorNone MotorClass = iota
	MotorPili
	MotorFlagella
)

// MouthClass is the feeding / terrain overlay, from whichever of
// Eating, Attack or Digging an organism has invested in most.
type MouthClass int

const (
	MouthNone MouthClass = iota
	MouthTeeth
	MouthFangs
	MouthTusks
)

// SensorClass is the sensory overlay, from the decision tree's
// conditions rather than from any score — sensing is a behaviour, not
// an ability, so what an organism looks like it senses with follows
// from what it actually checks.
type SensorClass int

const (
	SensorNone     SensorClass = iota
	SensorAntennae             // food and organism presence
	SensorFeelers              // walls and relative size
	SensorTasters              // pH
)

// AppearanceFor derives an organism's look from its scores and its
// decision tree.
//
// Call once per organism, at birth or on restore, and cache the
// result: the inputs are fixed for an organism's lifetime (scores
// mutate only into children, and the tree is copied-then-mutated for a
// child rather than edited in place), and the sensor half walks every
// node. Re-deriving it per frame would put a tree walk per organism
// into the render loop.
func AppearanceFor(scores Scores, tree *decision.Tree) Appearance {
	return Appearance{
		Body:   bodyClassFor(scores),
		Motor:  motorClassFor(scores),
		Mouth:  mouthClassFor(scores),
		Sensor: sensorClassFor(tree),
	}
}

func bodyClassFor(s Scores) BodyClass {
	switch d := s[AbilityDefense]; {
	case d >= config.SpikesBodyThreshold():
		return BodySpikes
	case d >= config.ShellBodyThreshold():
		return BodyShell
	default:
		return BodyBasic
	}
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

// mouthClassFor picks the single mouth overlay an organism has most
// earned. Eating, Attack and Digging each have their own threshold and
// their own sprite, but the renderer draws one mouth, so the highest
// qualifying score wins.
//
// Ties resolve in declaration order (teeth, fangs, tusks) via strict
// greater-than, which keeps the choice deterministic — an appearance
// that flickered between two sprites on equal scores would look like a
// rendering bug.
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

// sensorCategoryOf buckets a condition by the sense it implies.
// Conditions that need no sense organ — an organism's own health, age,
// or the pH of the cell it already occupies — return SensorNone and
// contribute nothing, so a tree full of self-checks grows no sensors.
func sensorCategoryOf(c decision.Condition) SensorClass {
	switch c {
	case decision.IsFoodAhead, decision.IsFoodLeft, decision.IsFoodRight,
		decision.IsOrganismAhead, decision.IsOrganismLeft, decision.IsOrganismRight,
		decision.IsRelativeAhead:
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

// sensorClassFor returns the overlay for whichever sense the tree leans
// on hardest, or SensorNone when nothing clears the minimum. Counting
// every occurrence rather than distinct conditions means a tree that
// checks for food repeatedly reads as more food-focused than one that
// checks once.
//
// Ties go to the earlier class in declaration order, so the result is
// stable for a given tree.
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
