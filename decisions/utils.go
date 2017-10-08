package decisions

import (
	"math/rand"
)

// NewRandomSubSequence returns random, minimum length sequence
func newRandomSubSequence() Sequence {
	subSequence := []interface{}{
		GetRandomCondition(),
		GetRandomAction(),
		GetRandomAction(),
	}
	return subSequence
}

// GetRandomCondition returns a random Condition from the Conditions array
func GetRandomCondition() Condition {
	return Conditions[rand.Intn(len(Conditions))]
}

// GetRandomAction returns a random Action from the Actions array
func GetRandomAction() Action {
	return Actions[rand.Intn(len(Actions))]
}

// isAction returns true if the object passed in is an Action
func isAction(v interface{}) bool {
	switch v.(type) {
	case Action:
		return true
	}
	return false
}
