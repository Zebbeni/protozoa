package ux

import (
	"testing"

	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/organism"
)

func familyTestTree() map[int]*organism.DescendantNode {
	nodes := map[int]*organism.DescendantNode{}
	add := func(id, parent int) {
		n := &organism.DescendantNode{ID: id}
		nodes[id] = n
		if p, ok := nodes[parent]; ok {
			p.AddChild(n)
		}
	}
	add(1, 0)
	add(2, 1)
	add(3, 1)
	add(4, 2)
	add(5, 2)
	add(6, 3)
	add(7, 4)
	add(8, 5)
	add(9, 7)
	add(10, 9)
	add(100, 0) // its own root, unconnected
	return nodes
}

func TestKinshipClassifiesTheTree(t *testing.T) {
	nodes := familyTestTree()
	const selID = 4
	ft := newFamilyTinter(nodes[selID], selID, 0)

	for _, tc := range []struct {
		id       int
		up, down int
		related  bool
		what     string
	}{
		{4, 0, 0, true, "the selected organism"},
		{7, 0, 1, true, "its child"},
		{9, 0, 2, true, "its grandchild"},
		{10, 0, 3, true, "its great-grandchild"},
		{2, 1, 0, true, "its parent"},
		{1, 2, 0, true, "its grandparent"},
		{5, 1, 1, true, "its sibling"},
		{8, 1, 2, true, "its sibling's child"},
		{3, 2, 1, true, "its parent's sibling"},
		{6, 2, 2, true, "its first cousin"},
		{100, 0, 0, false, "a different founder"},
	} {
		got := ft.kinshipOf(nodes[tc.id])
		want := kinship{up: tc.up, down: tc.down, related: tc.related}
		if got != want {
			t.Errorf("%s (id %d): got %+v, want %+v", tc.what, tc.id, got, want)
		}
	}
}

func TestKinshipMemoAgreesWithAFreshWalk(t *testing.T) {
	nodes := familyTestTree()
	const selID = 4

	// One tinter asked deepest-first, so every answer back-fills a chain.
	warm := newFamilyTinter(nodes[selID], selID, 0)
	for _, id := range []int{10, 6, 8, 100} {
		warm.kinshipOf(nodes[id])
	}

	for id, n := range nodes {
		fresh := newFamilyTinter(nodes[selID], selID, 0)
		if got, want := warm.kinshipOf(n), fresh.kinshipOf(n); got != want {
			t.Errorf("id %d: memoised %+v, fresh walk %+v", id, got, want)
		}
	}
}

func TestFamilyTinterInvalidation(t *testing.T) {
	nodes := familyTestTree()
	ft := newFamilyTinter(nodes[4], 4, 7)

	if !ft.valid(4, 7) {
		t.Error("a tinter should be valid for the selection and trees it was built for")
	}
	if ft.valid(5, 7) {
		t.Error("a tinter must not answer for a different selection")
	}
	if ft.valid(4, 8) {
		t.Error("a tinter must not survive the trees being rebuilt")
	}
	var nilTinter *familyTinter
	if nilTinter.valid(4, 7) {
		t.Error("valid must be safe on a nil tinter, since that is the starting state")
	}
}

func TestFamilyColorsSeparateTheDirections(t *testing.T) {
	hue := func(k kinship) float64 {
		h, _, _ := familyColor(k).HSLuv()
		return h
	}
	sat := func(k kinship) float64 {
		_, s, _ := familyColor(k).HSLuv()
		return s
	}

	child := kinship{down: 1, related: true}
	grandchild := kinship{down: 2, related: true}
	parent := kinship{up: 1, related: true}
	grandparent := kinship{up: 2, related: true}

	// Descendants head for blue, ancestors for yellow — opposite ways around from the selection's green.
	if !(hue(grandchild) > hue(child)) {
		t.Errorf("descendants should move toward blue with depth: child %v, grandchild %v", hue(child), hue(grandchild))
	}
	if !(hue(grandparent) < hue(parent)) {
		t.Errorf("ancestors should move toward yellow with height: parent %v, grandparent %v", hue(parent), hue(grandparent))
	}

	// Direct kin stay saturated however far out; cousins fade.
	far := kinship{down: familyFadeGenerations * 3, related: true}
	if sat(far) < 0.5 {
		t.Errorf("a distant direct descendant should stay saturated, got %v", sat(far))
	}
	near, mid := kinship{up: 1, down: 1, related: true}, kinship{up: 2, down: 2, related: true}
	if !(sat(mid) < sat(near)) {
		t.Errorf("cousins should desaturate with distance: near %v, mid %v", sat(near), sat(mid))
	}
}

func TestDistantCousinsAreJustBackground(t *testing.T) {
	gray := familyUnrelatedColor()

	far := kinship{up: familyCousinFade, down: familyCousinFade, related: true}
	if got := familyColor(far); !sameColor(got, gray) {
		t.Errorf("a cousin %d generations out is %v, want the unrelated gray %v",
			far.up+far.down, got, gray)
	}
	if got := familyColor(kinship{}); !sameColor(got, gray) {
		t.Errorf("an unrelated organism is %v, want gray %v", got, gray)
	}

	if !isNeutral(gray) {
		t.Errorf("the unrelated colour %v is not neutral", gray)
	}
}

func isNeutral(c colorful.Color) bool {
	const eps = 1.0 / 512
	return absDiff(c.R, c.G) < eps && absDiff(c.G, c.B) < eps
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func TestSelectedOrganismIsTheBrightest(t *testing.T) {
	_, selSat, _ := familyColor(kinship{related: true}).HSLuv()
	_, childSat, _ := familyColor(kinship{down: 1, related: true}).HSLuv()

	if selSat < childSat {
		t.Errorf("the selection (%v) should be at least as saturated as its children (%v)", selSat, childSat)
	}
	selHue, _, _ := familyColor(kinship{related: true}).HSLuv()
	if selHue != familyHueSelected {
		t.Errorf("the selection should be the green the key names, got hue %v", selHue)
	}
}
