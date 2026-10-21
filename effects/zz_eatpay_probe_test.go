package effects

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

// EAT_PAY=1 go test ./effects/ -run TestEatPayProbe -v
//
// What one SUCCESSFUL eating attempt is worth, per Eating score: the health
// the food carries against the health the attempt costs. Asserts nothing.
func TestEatPayProbe(t *testing.T) {
	if os.Getenv("EAT_PAY") == "" {
		t.Skip("set EAT_PAY=1")
	}
	path := os.Getenv("EAT_PAY_CONFIG")
	if path == "" {
		path = filepath.Join("..", "settings", "default.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	g.Repair()
	t.Logf("%s: bite %.3f@full %.3f@zero (curve %s), health/unit %.2f@full %.2f@zero (curve %s), "+
		"attempt %.3f -> %.3f",
		filepath.Base(path), g.MaxFoodPerEatAtFullEating, g.MaxFoodPerEatAtZeroEating,
		g.EatingCurveShape, g.HealthPerFoodUnit, g.HealthPerFoodUnitAtZero,
		g.EatingDigestionCurveShape, g.HealthChangeFromEatingAttempt,
		g.HealthChangeFromEatingAttemptAtMax)

	for _, size := range []float64{5, 20} {
		t.Logf("  size %.0f:", size)
		for score := 0; score <= physiology.MaxAbilityScore; score++ {
			food := MaxFoodPerEat(&g, score, size)
			gain := HealthFromFood(&g, score, food)
			cost := EatCost(&g, score, size)
			t.Logf("    eating %2d: food %7.3f units, gain %7.3f, cost %7.3f, net %+8.3f",
				score, food, gain, cost, gain+cost)
		}
	}
}
