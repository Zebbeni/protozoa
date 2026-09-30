package decision

type literal struct {
	cond  Condition
	value bool
}

func yes(c Condition) literal { return literal{cond: c, value: true} }
func no(c Condition) literal  { return literal{cond: c, value: false} }

type clause []literal

func implies(a, b Condition) clause { return clause{no(a), yes(b)} }

func excludes(a, b Condition) clause { return clause{no(a), no(b)} }

func oneOf(cs ...Condition) clause {
	cl := make(clause, 0, len(cs))
	for _, c := range cs {
		cl = append(cl, yes(c))
	}
	return cl
}

// An organismFamily is the set of conditions reading the same neighbouring cell.
type organismFamily struct {
	occupied, bigger, muchBigger, muchSmaller, relative Condition
}

var organismFamilies = []organismFamily{
	{IsOrganismAhead, IsBiggerOrganismAhead, IsMuchBiggerOrganismAhead, IsMuchSmallerOrganismAhead, IsRelativeAhead},
	{IsOrganismLeft, IsBiggerOrganismLeft, IsMuchBiggerOrganismLeft, IsMuchSmallerOrganismLeft, IsRelativeLeft},
	{IsOrganismRight, IsBiggerOrganismRight, IsMuchBiggerOrganismRight, IsMuchSmallerOrganismRight, IsRelativeRight},
}

// conditionRules is everything known about how the conditions relate.
var conditionRules = buildConditionRules()

func buildConditionRules() []clause {
	rules := []clause{
		// Health above half is health above a fifth.
		implies(IsHealthy, IsHealthAboveTwentyPercent),

		implies(IsVeryHealthy, IsHealthy),
		excludes(IsVeryUnhealthy, IsHealthy),
		excludes(IsVeryUnhealthy, IsVeryHealthy),
		// Below a fifth is not above a fifth.
		excludes(IsVeryUnhealthy, IsHealthAboveTwentyPercent),

		// Age divisible by ten is divisible by two.
		implies(IsAgeMultipleOfTen, IsAgeMultipleOfTwo),

		// "Much food here" tests the amount AND that there is any.
		implies(IsOrganismLeft, IsSomethingLeft),
		implies(IsWallLeft, IsSomethingLeft),
		implies(IsFoodLeft, IsSomethingLeft),
		implies(IsOrganismRight, IsSomethingRight),
		implies(IsWallRight, IsSomethingRight),
		implies(IsFoodRight, IsSomethingRight),

		implies(IsMuchFoodHere, IsFoodHere),
		implies(IsMuchFoodBuriedHere, IsFoodBuriedHere),

		// canMove refuses a cell holding another organism.
		excludes(CanMove, IsOrganismAhead),

		// The three pH-here conditions partition the scale.
		excludes(IsHealthyPhHere, IsPhTooLowHere),
		excludes(IsHealthyPhHere, IsPhTooHighHere),
		excludes(IsPhTooLowHere, IsPhTooHighHere),
		oneOf(IsHealthyPhHere, IsPhTooLowHere, IsPhTooHighHere),
	}

	for _, f := range organismFamilies {
		// Every size and kinship read requires an organism to be there.
		rules = append(rules,
			implies(f.bigger, f.occupied),
			implies(f.muchBigger, f.occupied),
			implies(f.muchSmaller, f.occupied),
			implies(f.relative, f.occupied),

			// Much bigger is bigger, and much smaller rules bigger out.
			implies(f.muchBigger, f.bigger),
			excludes(f.bigger, f.muchSmaller),
		)
	}
	return rules
}

// entail runs unit propagation over the rules from a set of known answers and returns every condition whose answer follows.
func entail(facts []literal) (map[Condition]bool, bool) {
	known := make(map[Condition]bool, len(facts)+len(conditionRules))
	for _, f := range facts {
		if v, seen := known[f.cond]; seen && v != f.value {
			return nil, false
		}
		known[f.cond] = f.value
	}

	// Each pass that changes anything adds at least one answer and never revises one.
	for changed := true; changed; {
		changed = false
		for _, cl := range conditionRules {
			var open literal
			openCount := 0
			satisfied := false
			for _, l := range cl {
				v, seen := known[l.cond]
				if !seen {
					open, openCount = l, openCount+1
					continue
				}
				if v == l.value {
					satisfied = true
					break
				}
			}
			if satisfied {
				continue
			}
			if openCount == 0 {
				return nil, false // every literal is known false
			}
			if openCount == 1 {
				known[open.cond] = open.value
				changed = true
			}
		}
	}
	return known, true
}

// A downstream is a condition somewhere in the subtree of the node being filled.
type downstream struct {
	branch bool
	mid    []literal
	cond   Condition
}

func (n *Node) downstreamConditions() []downstream {
	if n == nil || n.IsAction() {
		return nil
	}
	var out []downstream
	var walk func(cur *Node, branch bool, mid []literal)
	walk = func(cur *Node, branch bool, mid []literal) {
		if cur == nil || cur.IsAction() {
			return
		}
		cond, ok := cur.NodeType.(Condition)
		if !ok {
			return
		}
		out = append(out, downstream{branch: branch, mid: mid, cond: cond})
		walk(cur.YesNode, branch, append(append([]literal{}, mid...), yes(cond)))
		walk(cur.NoNode, branch, append(append([]literal{}, mid...), no(cond)))
	}
	walk(n.YesNode, true, nil)
	walk(n.NoNode, false, nil)
	return out
}

// usefulConditions filters a pool down to the conditions worth asking at one spot in a tree.
func usefulConditions(pool []Condition, above []literal, below []downstream) []Condition {
	known, ok := entail(above)
	if !ok {
		return pool
	}

	out := make([]Condition, 0, len(pool))
	for _, cond := range pool {
		if _, settled := known[cond]; settled {
			continue
		}
		if strandsSomethingBelow(above, cond, below) {
			continue
		}
		out = append(out, cond)
	}
	if len(out) == 0 {
		return pool
	}
	return out
}

// strandsSomethingBelow reports whether putting cond at this node would settle the answer to a condition already asked beneath it.
func strandsSomethingBelow(above []literal, cond Condition, below []downstream) bool {
	for _, d := range below {
		path := append(append([]literal{}, above...), d.mid...)
		if settled(path, d.cond) {
			continue
		}
		withCandidate := append(append([]literal{}, above...), literal{cond: cond, value: d.branch})
		withCandidate = append(withCandidate, d.mid...)
		if settled(withCandidate, d.cond) {
			return true
		}
	}
	return false
}

// settled reports whether facts fix target's answer.
func settled(facts []literal, target Condition) bool {
	known, ok := entail(facts)
	if !ok {
		return true
	}
	_, isSettled := known[target]
	return isSettled
}

// factsAbove returns the conditions answered on the way down from n to target, each with the branch taken to reach it.
func (n *Node) factsAbove(target *Node) ([]literal, bool) {
	if n == nil {
		return nil, false
	}
	if n == target {
		return nil, true
	}
	cond, isCond := n.NodeType.(Condition)
	if !isCond {
		return nil, false
	}
	if facts, found := n.YesNode.factsAbove(target); found {
		return append([]literal{yes(cond)}, facts...), true
	}
	if facts, found := n.NoNode.factsAbove(target); found {
		return append([]literal{no(cond)}, facts...), true
	}
	return nil, false
}
