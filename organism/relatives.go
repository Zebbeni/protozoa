package organism

// areRelatives reports whether two organisms are in one direct line of descent.
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
