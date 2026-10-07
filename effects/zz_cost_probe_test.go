package effects

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
)

// COST_PROBE=1 go test ./effects/ -run TestCostProbe -v
func TestCostProbe(t *testing.T) {
	if os.Getenv("COST_PROBE") == "" {
		t.Skip("set COST_PROBE=1")
	}
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	g.Repair()
	const size = 10
	t.Logf("per-cycle cost at size %v, by score (move vs attack):", float64(size))
	for _, score := range []int{0, 2, 5, 10} {
		mv := MoveCost(&g, score, size)
		atk := AttackCost(&g, score, size)
		t.Logf("  score %2d: move %+.4f   attack %+.4f   attack is %.2fx the move",
			score, mv, atk, atk/mv)
	}
	t.Logf("cross terms (an organism moves on Movement, lunges on Attack):")
	for _, mvScore := range []int{0, 3, 10} {
		for _, atkScore := range []int{0, 3, 10} {
			mv, atk := MoveCost(&g, mvScore, size), AttackCost(&g, atkScore, size)
			cheaper := ""
			if atk > mv {
				cheaper = "  <-- lunging is CHEAPER than moving"
			}
			t.Logf("  Movement %2d (%+.4f) vs Attack %2d (%+.4f)%s", mvScore, mv, atkScore, atk, cheaper)
		}
	}
}
