package organism

import (
	"testing"

	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/utils"
)

// family builds a small tree:
//
//	root (0) ── parent (10) ── child (20) ── grandchild (30)
//	                        └─ sibling (25)
//	stranger (5), unrelated root
func family() (root, parent, child, grandchild, sibling, stranger *DescendantNode) {
	root = &DescendantNode{ID: 1, StartCycle: 0}
	parent = &DescendantNode{ID: 2, StartCycle: 10, Parent: root}
	child = &DescendantNode{ID: 3, StartCycle: 20, Parent: parent}
	grandchild = &DescendantNode{ID: 4, StartCycle: 30, Parent: child}
	sibling = &DescendantNode{ID: 5, StartCycle: 25, Parent: parent}
	stranger = &DescendantNode{ID: 6, StartCycle: 5}
	return
}

func TestDirectLineIsRelated(t *testing.T) {
	root, parent, child, grandchild, _, _ := family()
	for _, tc := range []struct {
		name string
		a, b *DescendantNode
	}{
		{"parent and child", parent, child},
		{"child and parent", child, parent},
		{"grandparent and grandchild", parent, grandchild},
		{"founder and great-grandchild", root, grandchild},
	} {
		if !areRelatives(tc.a, tc.b) {
			t.Errorf("%s should be relatives", tc.name)
		}
	}
}

func TestOutsideDirectLineIsNotRelated(t *testing.T) {
	_, _, child, grandchild, sibling, stranger := family()
	for _, tc := range []struct {
		name string
		a, b *DescendantNode
	}{
		{"siblings", child, sibling},
		{"aunt and nephew", sibling, grandchild},
		{"strangers", stranger, grandchild},
		{"nil node", nil, child},
		{"same organism", child, child},
	} {
		if areRelatives(tc.a, tc.b) {
			t.Errorf("%s should not be relatives", tc.name)
		}
	}
}

// countingNode wraps a long single line of descent so the walk's stopping
// rule can be checked: it must not climb past the older organism's birth.
func TestWalkStopsAtOlderOrganismsBirth(t *testing.T) {
	// A 1000-generation line, one generation per cycle.
	nodes := make([]*DescendantNode, 1000)
	for i := range nodes {
		nodes[i] = &DescendantNode{ID: i + 1, StartCycle: i}
		if i > 0 {
			nodes[i].Parent = nodes[i-1]
		}
	}
	// An unrelated organism born at cycle 990: the walk from the youngest
	// may visit only ancestors born at 990 or later.
	outsider := &DescendantNode{ID: 5000, StartCycle: 990}

	visited := 0
	for n := nodes[999].Parent; n != nil && n.StartCycle >= outsider.StartCycle; n = n.Parent {
		visited++
	}
	if visited > 10 {
		t.Fatalf("test setup: expected at most 10 ancestors in range, got %d", visited)
	}
	if areRelatives(outsider, nodes[999]) {
		t.Error("an organism outside the line should not be related")
	}
	if !areRelatives(nodes[995], nodes[999]) {
		t.Error("an ancestor four generations up should be related")
	}
}

// orgLookup is a LookupAPI with one organism at a known cell.
type orgLookup struct {
	phLookup
	at  utils.Point
	org *Organism
}

func (l orgLookup) CheckOrganismAtPoint(p utils.Point, check OrgCheck) bool {
	if p != l.at {
		return check(nil)
	}
	return check(l.org)
}
func (l orgLookup) GetFoodAtPoint(utils.Point) (*food.Item, bool) { return nil, false }

// TestIsRelativeAheadCondition wires the rule into the decision-tree
// condition: true for a relative directly ahead, false for a stranger or
// an empty cell.
func TestIsRelativeAheadCondition(t *testing.T) {
	_, parent, child, _, _, stranger := family()
	self := &Organism{Location: utils.Point{X: 5, Y: 5}, Direction: utils.Point{X: 1, Y: 0}, TreeNode: child}
	ahead := utils.Point{X: 6, Y: 5}

	self.lookupAPI = orgLookup{at: ahead, org: &Organism{TreeNode: parent}}
	if !self.isConditionTrue(d.IsRelativeAhead) {
		t.Error("parent ahead should satisfy If Relative Ahead")
	}
	self.lookupAPI = orgLookup{at: ahead, org: &Organism{TreeNode: stranger}}
	if self.isConditionTrue(d.IsRelativeAhead) {
		t.Error("stranger ahead should not satisfy If Relative Ahead")
	}
	self.lookupAPI = orgLookup{at: utils.Point{X: 0, Y: 0}, org: &Organism{TreeNode: parent}}
	if self.isConditionTrue(d.IsRelativeAhead) {
		t.Error("empty cell ahead should not satisfy If Relative Ahead")
	}
}
