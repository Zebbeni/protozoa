package decision

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"
)

// Node contains an Action or Condition NodeType and (if a Condition), child
// references for its conditional branches
type Node struct {
	NodeType                      interface{}
	InDecisionTree, UsedLastCycle bool
	WasTravelled                  bool
	YesNode, NoNode               *Node
	size                          int

	mutex sync.Mutex
}

// PrintLine represents a single line of decision tree output with
// metadata. UsedLastCycle is true for nodes on the path chooseAction
// took on the most recent cycle. WasTravelled is the lifetime-
// cumulative flag — true for any node ever visited. The panel renders
// a three-tier highlight: brightest for the current path, mid-tone
// for ever-travelled-but-not-this-cycle, dim for never-visited.
//
// NodeType carries the underlying decision.Action or decision.Condition
// so the panel can ask the physiology layer whether the node is
// currently gated (the organism's lineage has lost the feature that
// unlocked it) and render gated nodes with a strikethrough — visible
// "junk DNA" that the organism is carrying but cannot use.
//
// Prefix is the tree-drawing portion of Text (indent + ├─/└─); the
// panel uses textAdvance(Prefix) and textAdvance(Text) to bound the
// strikethrough so the line-drawing characters stay clean.
type PrintLine struct {
	Text          string
	Prefix        string
	UsedLastCycle bool
	WasTravelled  bool
	NodeType      interface{}
}

// NodeFromAction creates a simple Node object from an Action type
func NodeFromAction(action Action) *Node {
	return &Node{
		NodeType: action,
		size:     1,
	}
}

// IsAction returns true if Tree's type is Action (false if Condition)
func (n *Node) IsAction() bool {
	return isAction(n.NodeType)
}

// IsCondition returns true if Tree's type is Action (false if Condition)
func (n *Node) IsCondition() bool {
	return isCondition(n.NodeType)
}

// CopyNode returns a new Node with the same structure as the original
func (n Node) CopyNode() *Node {
	copy := &Node{
		NodeType:      n.NodeType,
		UsedLastCycle: n.UsedLastCycle,
		WasTravelled:  n.WasTravelled,
		size:          n.size,
	}
	if n.IsAction() {
		return copy
	}
	copy.YesNode = n.YesNode.CopyNode()
	copy.NoNode = n.NoNode.CopyNode()
	return copy
}

// SetUsedInCurrentTree sets whether this Node is contained in a
// currently-used decision tree
func (n *Node) SetUsedInCurrentTree(isUsing bool) {
	n.InDecisionTree = isUsing
	if n.IsCondition() {
		n.YesNode.SetUsedInCurrentTree(isUsing)
		n.NoNode.SetUsedInCurrentTree(isUsing)
	}
}

// ResetUsedLastCycle triggers this Node (and any previously-used child Nodes)
// to set UsedLastCycle to false
func (n *Node) ResetUsedLastCycle() {
	n.UsedLastCycle = false
	if n.IsCondition() {
		if n.YesNode.UsedLastCycle {
			n.YesNode.ResetUsedLastCycle()
		} else {
			n.NoNode.ResetUsedLastCycle()
		}
	}
}

// Serialize generates and returns a string representing a Node's
// full Tree structure.
//
// Recursively walks through the Node tree to accumulate a string representing
// itself and all its children
func (n *Node) Serialize() string {
	var buffer bytes.Buffer
	nodeTypeString := fmt.Sprintf("%02d", n.NodeType)
	buffer.WriteString(nodeTypeString)
	if n.IsCondition() {
		buffer.WriteString(n.YesNode.Serialize())
		buffer.WriteString(n.NoNode.Serialize())
	}
	return buffer.String()
}

// getNodes returns a list of all nodes in a tree starting with the given root
func (n *Node) getNodes() (nodes []*Node) {
	nodes = make([]*Node, 0, n.size)
	nodes = append(nodes, n)
	if n.IsAction() {
		return
	}

	nodes = append(nodes, n.YesNode.getNodes()...)
	nodes = append(nodes, n.NoNode.getNodes()...)
	return
}

func (n *Node) printLines(indent string, first, last bool) []PrintLine {
	prefix := indent
	newIndent := indent
	if first {
		// root node, no prefix
	} else if last {
		prefix = fmt.Sprintf("%s└─", prefix)
		newIndent = fmt.Sprintf("%s  ", newIndent)
	} else {
		prefix = fmt.Sprintf("%s├─", prefix)
		newIndent = fmt.Sprintf("%s│ ", newIndent)
	}
	lineText := prefix + Map[n.NodeType]
	if n.UsedLastCycle {
		lineText += " ◀◀"
	}
	lines := []PrintLine{{
		Text:          lineText,
		Prefix:        prefix,
		UsedLastCycle: n.UsedLastCycle,
		WasTravelled:  n.WasTravelled,
		NodeType:      n.NodeType,
	}}
	if n.IsCondition() {
		lines = append(lines, n.YesNode.printLines(newIndent, false, false)...)
		lines = append(lines, n.NoNode.printLines(newIndent, false, true)...)
	}
	return lines
}

func (n *Node) print(indent string, first, last bool) string {
	toPrint := indent
	newIndent := indent
	if first {
		toPrint = fmt.Sprintf("%s", toPrint)
	} else if last {
		toPrint = fmt.Sprintf("%s└─", toPrint)
		newIndent = fmt.Sprintf("%s  ", newIndent)
	} else {
		toPrint = fmt.Sprintf("%s├─", toPrint)
		newIndent = fmt.Sprintf("%s│ ", newIndent)
	}
	if n.UsedLastCycle {
		toPrint = fmt.Sprintf("%s%s ◀◀\n", toPrint, Map[n.NodeType])
	} else {
		toPrint = fmt.Sprintf("%s%s\n", toPrint, Map[n.NodeType])
	}
	if n.IsCondition() {
		toPrint = fmt.Sprintf("%s%s", toPrint, n.YesNode.print(newIndent, false, false))
		toPrint = fmt.Sprintf("%s%s", toPrint, n.NoNode.print(newIndent, false, true))
	}
	return toPrint
}

// intToNodeType maps a serialized int code back to an Action or Condition.
// Built once at init from the Actions and Conditions arrays.
var codeToNodeType map[int]interface{}

func init() {
	codeToNodeType = make(map[int]interface{})
	for _, a := range Actions {
		codeToNodeType[int(a)] = a
	}
	for _, c := range Conditions {
		codeToNodeType[int(c)] = c
	}
	// ActSpawn isn't in Actions array but can appear in serialized trees
	codeToNodeType[int(ActSpawn)] = ActSpawn
	// Trait-tree placeholder actions: not in Actions array (selection
	// is feature-gated, not by random pick from the global slice), but
	// must round-trip through serialization once trees start to contain
	// them in later slices.
	codeToNodeType[int(ActSting)] = ActSting
	codeToNodeType[int(ActDig)] = ActDig
	codeToNodeType[int(ActHunker)] = ActHunker
	codeToNodeType[int(ActFlare)] = ActFlare
	codeToNodeType[int(ActHide)] = ActHide
	codeToNodeType[int(ActBurrow)] = ActBurrow
}

// Deserialize parses a serialized tree string back into a Node tree.
// Returns the node and the number of characters consumed from the string.
func Deserialize(s string) (*Node, int) {
	if len(s) < 2 {
		return nil, 0
	}
	code, err := strconv.Atoi(s[0:2])
	if err != nil {
		return nil, 0
	}
	nodeType, ok := codeToNodeType[code]
	if !ok {
		return nil, 0
	}

	node := &Node{NodeType: nodeType, size: 1}
	consumed := 2

	if isCondition(nodeType) {
		yes, yesConsumed := Deserialize(s[consumed:])
		consumed += yesConsumed
		no, noConsumed := Deserialize(s[consumed:])
		consumed += noConsumed
		node.YesNode = yes
		node.NoNode = no
		node.size = 1 + yes.size + no.size
	}

	return node, consumed
}
