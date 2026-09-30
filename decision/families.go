package decision

// Condition families, as a forest of coarse reads refined by finer ones.
var conditionParent = map[Condition]Condition{
	// Amount refines presence, at the organism's own cell.
	IsMuchFoodHere:       IsFoodHere,
	IsMuchFoodBuriedHere: IsFoodBuriedHere,

	// A tighter threshold on the same comparison.
	IsAgeMultipleOfTen: IsAgeMultipleOfTwo,

	// The health family, shaped like the pH one.
	IsVeryHealthy:   IsHealthy,
	IsVeryUnhealthy: IsHealthy,

	// Which way the water is off refines that it is off at all.
	IsPhTooLowHere:  IsHealthyPhHere,
	IsPhTooHighHere: IsHealthyPhHere,

	// One family per direction: something is there, then what it is.
	IsBiggerOrganismAhead:      IsOrganismAhead,
	IsMuchBiggerOrganismAhead:  IsBiggerOrganismAhead,
	IsMuchSmallerOrganismAhead: IsOrganismAhead,
	IsRelativeAhead:            IsOrganismAhead,

	IsBiggerOrganismLeft:      IsOrganismLeft,
	IsMuchBiggerOrganismLeft:  IsBiggerOrganismLeft,
	IsMuchSmallerOrganismLeft: IsOrganismLeft,
	IsRelativeLeft:            IsOrganismLeft,

	IsBiggerOrganismRight:      IsOrganismRight,
	IsMuchBiggerOrganismRight:  IsBiggerOrganismRight,
	IsMuchSmallerOrganismRight: IsOrganismRight,
	IsRelativeRight:            IsOrganismRight,

	// The flanks hang off one coarse "is anything there" per side.
	IsOrganismLeft: IsSomethingLeft,
	IsWallLeft:     IsSomethingLeft,
	IsFoodLeft:     IsSomethingLeft,

	IsOrganismRight: IsSomethingRight,
	IsWallRight:     IsSomethingRight,
	IsFoodRight:     IsSomethingRight,

	// Roots, listed here only as a reminder of what is deliberately basic rather than as entries.
}

var conditionChildren = buildConditionChildren()

func buildConditionChildren() map[Condition][]Condition {
	children := map[Condition][]Condition{}
	for _, c := range MutableConditions {
		if p, ok := conditionParent[c]; ok {
			children[p] = append(children[p], c)
		}
	}
	return children
}

// basicConditions are the family roots in MutableConditions order.
var basicConditions = buildBasicConditions()

func buildBasicConditions() []Condition {
	var out []Condition
	for _, c := range MutableConditions {
		if _, refined := conditionParent[c]; !refined {
			out = append(out, c)
		}
	}
	return out
}

func IsBasicCondition(c Condition) bool {
	_, refined := conditionParent[c]
	return !refined
}

// BasicConditions is the pool for a NEW condition node: the family roots, minus whatever the settings disable.
func BasicConditions() []Condition {
	return intersectOrdered(basicConditions, EnabledConditions())
}

// LadderConditions is the pool for swapping the condition already at a node.
func LadderConditions(current Condition) []Condition {
	wanted := map[Condition]bool{}
	for _, c := range basicConditions {
		wanted[c] = true
	}
	for _, c := range conditionChildren[current] {
		wanted[c] = true
	}
	if p, ok := conditionParent[current]; ok {
		wanted[p] = true
	}
	var out []Condition
	for _, c := range MutableConditions {
		if wanted[c] {
			out = append(out, c)
		}
	}
	return intersectOrdered(out, EnabledConditions())
}

// intersectOrdered keeps the order of want and drops anything absent from allowed.
func intersectOrdered(want, allowed []Condition) []Condition {
	ok := map[Condition]bool{}
	for _, c := range allowed {
		ok[c] = true
	}
	var out []Condition
	for _, c := range want {
		if ok[c] {
			out = append(out, c)
		}
	}
	return out
}

// FamilyRoot walks c up to the basic read at the top of its family.
func FamilyRoot(c Condition) Condition {
	for i := 0; i <= len(MutableConditions); i++ {
		p, ok := conditionParent[c]
		if !ok {
			return c
		}
		c = p
	}
	// Unreachable: TestFamiliesAreAcyclicAndRootedAtBasics pins that every chain terminates.
	return c
}

// ConditionFamilies lists the families that HAVE advanced reads, in MutableConditions order.
func ConditionFamilies() []Condition {
	var out []Condition
	for _, c := range basicConditions {
		if len(conditionChildren[c]) > 0 {
			out = append(out, c)
		}
	}
	return out
}

// AdvancedConditions lists every refined read below root, depth first in MutableConditions order.
func AdvancedConditions(root Condition) []Condition {
	var out []Condition
	for _, c := range MutableConditions {
		if c != root && FamilyRoot(c) == root {
			out = append(out, c)
		}
	}
	return out
}

// BasicConditionsAll is every family root, whatever the settings say.
func BasicConditionsAll() []Condition {
	out := make([]Condition, len(basicConditions))
	copy(out, basicConditions)
	return out
}
