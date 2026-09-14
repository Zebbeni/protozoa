package organism

// areRelatives reports whether two organisms are in one direct line of
// descent: one is the other's parent, grandparent, great-grandparent and
// so on.
//
// It walks up from the younger organism's parent and stops at the first
// of two things: the older organism, which makes them relatives, or an
// ancestor born before the older organism, which rules it out — every
// ancestor further up is older still, so the older organism can't appear.
// The walk never visits more ancestors than were born between the two
// organisms' births.
//
// Siblings and cousins are not relatives by this rule: a sibling's line
// passes through the shared parent, never through the other sibling.
// Nil nodes (an organism with no recorded family tree) are never related.
func areRelatives(a, b *DescendantNode) bool {
	if a == nil || b == nil || a.ID == b.ID {
		return false
	}
	older, younger := a, b
	if a.StartCycle > b.StartCycle {
		older, younger = b, a
	}
	for n := younger.Parent; n != nil && n.StartCycle >= older.StartCycle; n = n.Parent {
		if n.ID == older.ID {
			return true
		}
	}
	return false
}
