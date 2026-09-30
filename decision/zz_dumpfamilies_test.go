package decision

import (
	"os"
	"sort"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

func TestDumpFamilies(t *testing.T) {
	if os.Getenv("FAMILY_DUMP") == "" {
		t.Skip("set FAMILY_DUMP=1 to print the condition families")
	}
	loadDefaults(t)
	enabled := map[Condition]bool{}
	for _, c := range EnabledConditions() {
		enabled[c] = true
	}
	mark := func(c Condition) string {
		if enabled[c] {
			return ""
		}
		return "   [disabled by default]"
	}
	var walk func(c Condition, depth int)
	walk = func(c Condition, depth int) {
		pad := ""
		for i := 0; i < depth; i++ {
			pad += "    "
		}
		arrow := ""
		if depth > 0 {
			arrow = "└─ "
		}
		t.Logf("%s%s%-28s %-30s%s", pad, arrow, Names[c], Map[c], mark(c))
		kids := append([]Condition{}, conditionChildren[c]...)
		sort.Slice(kids, func(i, j int) bool { return Names[kids[i]] < Names[kids[j]] })
		for _, k := range kids {
			walk(k, depth+1)
		}
	}
	t.Logf("=== %d basic reads (a NEW condition node may only ask these) ===", len(basicConditions))
	for _, root := range basicConditions {
		walk(root, 0)
	}
	_ = config.SmartTreeMutation()
}
