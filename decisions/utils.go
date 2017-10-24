package decisions

import (
	"math/rand"

	c "../constants"
)

// NewRandomSequence generates a new Sequence of random length
func NewRandomSequence() Sequence {
	numSequenceNodes := rand.Intn(c.MaxSequenceNodes)
	sequence := make(Sequence, numSequenceNodes)
	for n := 0; n < numSequenceNodes; n++ {
		if rand.Float32() < c.PercentActions {
			sequence[n] = GetRandomAction()
		} else {
			sequence[n] = GetRandomCondition()
		}
	}
	return sequence
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
