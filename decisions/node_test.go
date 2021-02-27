package decisions

import "testing"

func TestUpdateNodeIDs(t *testing.T) {
	testCases := []struct {
		node     *Node
		expected string
	}{
		{TreeFromAction(ActAttack), "00"},
		{&Node{NodeType: CanMove, YesNode: TreeFromAction(ActAttack), NoNode: TreeFromAction(ActEat)}, "080002"},
	}

	for index, testCase := range testCases {
		actual := testCase.node.UpdateNodeIDs()
		expected := testCase.expected
		if actual != expected {
			t.Errorf("node ID %d was %s, expected %s\n", index, actual, expected)
		}
	}
}
