package simulation

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/utils"
)

// TestMovementMeasurement is a scratch diagnostic: why do lineages spread
// by reproduction rather than by moving? It records, over the second half
// of each run on default settings:
//   - the status mix (what organisms actually do each cycle)
//   - what's in the cell ahead and how many free neighbours they have
//   - whether their trees contain Move at all, and whether it ever runs
//   - how much better chemosynthesis would be within a few cells, and
//     how many cycles of that better gain it would take to repay the trip
func TestMovementMeasurement(t *testing.T) {
	seedsEnv := os.Getenv("MEASURE_SEEDS")
	if seedsEnv == "" {
		t.Skip("set MEASURE_SEEDS")
	}
	var seeds []int
	for _, f := range strings.Split(seedsEnv, ",") {
		n, _ := strconv.Atoi(f)
		seeds = append(seeds, n)
	}
	loadGlobalsForMeasure(t)

	const capCycles = 10000
	const landscapeEvery = 25
	const treeEvery = 500
	radii := []int{1, 3, 6}

	for _, seed := range seeds {
		sim := NewSimulation(&config.Options{IsHeadless: true, Seed: seed, CheckpointInterval: 1 << 30})
		W, H := config.GridUnitsWide(), config.GridUnitsHigh()

		status := map[organism.Status]float64{}
		var orgCycles float64
		var aheadEmpty, aheadOrg, aheadFood, aheadWall, freeNbrs, noFreeNbr, nbrSamples float64
		var treeSamples, hasMove, moveTravelled, hasTurn float64
		var landSamples, effHere float64
		bestGain := make([]float64, len(radii))
		bigGain := make([]float64, len(radii)) // share where best-in-radius beats here by >= 0.2 efficiency
		var paybacks []float64
		var localSpread, occupancy, occSamples float64
		var chemoFailed float64

		cycles := 0
		for cycles < capCycles && !sim.IsDone() {
			sim.Update()
			cycles++
			if cycles < capCycles/2 {
				continue
			}
			orgs := sim.organismManager.Organisms()
			for _, o := range orgs {
				status[o.Status]++
				orgCycles++
				if o.Status == organism.StatusChemoFailed {
					chemoFailed++
				}
			}

			if cycles%landscapeEvery == 0 {
				occ := make(map[utils.Point]bool, len(orgs))
				for _, o := range orgs {
					occ[o.Location] = true
				}
				occupied := func(p utils.Point) (org, food, wall bool) {
					if sim.IsWallAtPoint(p) {
						return false, false, true
					}
					if _, ok := sim.GetFoodAtPoint(p); ok {
						return false, true, false
					}
					return occ[p], false, false
				}
				occupancy += float64(len(orgs)) / float64(W*H)
				occSamples++

				window := config.ChemosynthesisPhWindow()
				eff := func(ideal float64, p utils.Point) float64 {
					dist := math.Abs(ideal - sim.GetPhAtPoint(p))
					if dist >= window {
						return 0
					}
					if k := config.ChemoPhFalloff(); k > 0 {
						return max(0, 1-math.Pow(dist/window, k))
					}
					return 1
				}

				for _, o := range orgs {
					if o.Status == organism.StatusDying {
						continue
					}
					ahead := o.Location.Add(o.Direction)
					org, food, wall := occupied(ahead)
					switch {
					case wall:
						aheadWall++
					case food:
						aheadFood++
					case org:
						aheadOrg++
					default:
						aheadEmpty++
					}
					free := 0
					dir := utils.Point{X: 0, Y: -1}
					for i := 0; i < 4; i++ {
						a, b, c := occupied(o.Location.Add(dir))
						if !a && !b && !c {
							free++
						}
						dir = dir.Left()
					}
					freeNbrs += float64(free)
					if free == 0 {
						noFreeNbr++
					}
					nbrSamples++

					ideal := o.Traits().IdealPh
					here := eff(ideal, o.Location)
					effHere += here
					landSamples++

					minPh, maxPh := math.Inf(1), math.Inf(-1)
					for ri, r := range radii {
						best, bestSteps := here, 0
						for dx := -r; dx <= r; dx++ {
							for dy := -r; dy <= r; dy++ {
								p := o.Location.Add(utils.Point{X: dx, Y: dy})
								if e := eff(ideal, p); e > best {
									best, bestSteps = e, abs(dx)+abs(dy)
								}
								if r == 3 {
									ph := sim.GetPhAtPoint(p)
									minPh, maxPh = min(minPh, ph), max(maxPh, ph)
								}
							}
						}
						bestGain[ri] += best - here
						if best-here >= 0.2 {
							bigGain[ri]++
						}
						// Payback at the widest radius: cycles of improved
						// chemosynthesis needed to repay the steps there
						// (turns ignored, so this is a lower bound).
						if ri == len(radii)-1 && best-here > 0.01 {
							gainPerCycle := config.HealthChangeFromChemosynthesis() *
								o.AbilityMultiplier(physiology.AbilityChemosynthesis) * (best - here)
							costPerStep := -config.HealthChangeFromMoving() * o.AbilityMultiplier(physiology.AbilityMovement)
							paybacks = append(paybacks, float64(bestSteps)*costPerStep/gainPerCycle)
						}
					}
					localSpread += maxPh - minPh
				}
			}

			if cycles%treeEvery == 0 {
				for _, o := range orgs {
					lines := o.GetDecisionTreeCopy().PrintLines()
					var move, travelled, turn bool
					for _, l := range lines {
						switch l.NodeType {
						case d.ActMove:
							move = true
							travelled = travelled || l.WasTravelled
						case d.ActTurnLeft, d.ActTurnRight:
							turn = true
						}
					}
					treeSamples++
					if move {
						hasMove++
					}
					if travelled {
						moveTravelled++
					}
					if turn {
						hasTurn++
					}
				}
			}
		}

		end := fmt.Sprintf("ended@%d", cycles)
		if cycles >= capCycles && !sim.IsDone() {
			end = "survived"
		}
		pct := func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return 100 * a / b
		}
		avg := func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		}
		sort.Float64s(paybacks)
		medPayback := 0.0
		if len(paybacks) > 0 {
			medPayback = paybacks[len(paybacks)/2]
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "RESULT seed=%d end=%s occ%%=%.1f", seed, end, 100*avg(occupancy, occSamples))
		for _, st := range []struct {
			name string
			s    organism.Status
		}{
			{"chemoOK", organism.StatusChemoSuccess}, {"chemoFail", organism.StatusChemoFailed},
			{"eatOK", organism.StatusEatSuccess}, {"eatFail", organism.StatusEatFailed},
			{"moveOK", organism.StatusMoveSuccess}, {"moveBlocked", organism.StatusMoveBlocked},
			{"turn", organism.StatusTurnLeft}, {"turnR", organism.StatusTurnRight},
			{"attack", organism.StatusAttacking}, {"spawn", organism.StatusSpawning},
			{"dig", organism.StatusDigging}, {"idle", organism.StatusIdle},
		} {
			fmt.Fprintf(&sb, " %s%%=%.2f", st.name, pct(status[st.s], orgCycles))
		}
		fmt.Fprintf(&sb, " aheadEmpty%%=%.1f aheadOrg%%=%.1f aheadFood%%=%.1f aheadWall%%=%.1f freeNbrs=%.2f noFreeNbr%%=%.1f",
			pct(aheadEmpty, nbrSamples), pct(aheadOrg, nbrSamples), pct(aheadFood, nbrSamples), pct(aheadWall, nbrSamples),
			avg(freeNbrs, nbrSamples), pct(noFreeNbr, nbrSamples))
		fmt.Fprintf(&sb, " treeHasMove%%=%.1f moveNodeRan%%=%.1f treeHasTurn%%=%.1f",
			pct(hasMove, treeSamples), pct(moveTravelled, treeSamples), pct(hasTurn, treeSamples))
		fmt.Fprintf(&sb, " effHere=%.3f", avg(effHere, landSamples))
		for ri, r := range radii {
			fmt.Fprintf(&sb, " gainR%d=%.3f bigGainR%d%%=%.1f", r, avg(bestGain[ri], landSamples), r, pct(bigGain[ri], landSamples))
		}
		fmt.Fprintf(&sb, " phSpreadR3=%.3f paybackMed=%.1f", avg(localSpread, landSamples), medPayback)
		t.Log(sb.String())
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func loadGlobalsForMeasure(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	config.SetGlobals(&g)
}
