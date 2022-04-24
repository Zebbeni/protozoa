package decision

import "testing"

func TestUpdateNodeIDs(t *testing.T) {
	testCases := []struct {
		tree     *Tree
		expected string
	}{
		{TreeFromAction(ActAttack), "00"},
		{&Tree{ID: "080002", Node: &Node{NodeType: CanMove, YesNode: NodeFromAction(ActAttack), NoNode: NodeFromAction(ActEat)}}, "080002"},
	}

	for index, testCase := range testCases {
		actual := testCase.tree.Serialize()
		expected := testCase.expected
		if actual != expected {
			t.Errorf("tree ID %d was %s, expected %s\n", index, actual, expected)
		}
	}
}
