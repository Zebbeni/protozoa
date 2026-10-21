package organism

// HealthSource is one origin of a health change, for the per-cycle ledger the
// panel shows under the portrait.
type HealthSource int

const (
	HealthFromAttack HealthSource = iota // damage taken from attacks aimed at this organism
	HealthFromThorns                     // damage taken from attacking a defender that answers back
	HealthFromPh
	HealthFromAction // the chosen action's own cost
	HealthFromChemo
	HealthFromFood
	HealthFromPredation // fed back from a hit this organism landed
	HealthFromSpawn
	HealthFromBrain // per-cycle upkeep of a neural net, by node count
	healthSourceCount
)

// AllHealthSources is the canonical list, in the order the panel stacks them.
var AllHealthSources = []HealthSource{
	HealthFromAttack,
	HealthFromThorns,
	HealthFromPh,
	HealthFromAction,
	HealthFromChemo,
	HealthFromFood,
	HealthFromPredation,
	HealthFromSpawn,
	HealthFromBrain,
}

// MaxConcurrentHealthSources is how many of the sources can be non-zero on
// one cycle, which is what the panel reserves room for rather than the whole
// list.
//
// An organism takes one action a cycle, and that is what bounds it: the pH
// cost, the action's own cost and — in a world running on neural nets — the
// brain's upkeep are always available, being attacked and spawning are
// independent of what it chose, and the action itself contributes either
// thorns and predation (it attacked) or one of chemosynthesis and eating.
// Seven. Measured over 5.4M organism-cycles on decision trees the most ever
// seen together is five — hit, thorns, ph, act, prey — and
// TestNoCycleMovesMoreSourcesThanThePanelReserves is the live-world guard.
//
// It was six before the nets: brain upkeep is charged every cycle, so a net
// world adds one to whatever a tree world reaches.
const MaxConcurrentHealthSources = 7

var healthSourceLabels = [healthSourceCount]string{
	HealthFromAttack:    "hit",
	HealthFromThorns:    "thorns",
	HealthFromPh:        "ph",
	HealthFromAction:    "act",
	HealthFromChemo:     "chemo",
	HealthFromFood:      "eat",
	HealthFromPredation: "prey",
	HealthFromSpawn:     "spawn",
	HealthFromBrain:     "brain",
}

func (s HealthSource) Label() string {
	if s < 0 || s >= healthSourceCount {
		return ""
	}
	return healthSourceLabels[s]
}

// HealthLedger is one cycle's health changes split by source.
//
// An observation, not the applied value: it is recorded beside each health
// change rather than through it, so that no accounting here can alter the
// float arithmetic a replay depends on.
type HealthLedger struct {
	Amounts [healthSourceCount]float64
	// Recorded is false until a cycle has resolved for this organism, which
	// is how a restored snapshot is told apart from a cycle that happened to
	// cost nothing.
	Recorded bool
}

func (l HealthLedger) Total() float64 {
	sum := 0.0
	for _, v := range l.Amounts {
		sum += v
	}
	return sum
}

func (o *Organism) ResetHealthLedger() {
	o.healthLedger = HealthLedger{Recorded: true}
}

func (o *Organism) RecordHealth(src HealthSource, amount float64) {
	if src < 0 || src >= healthSourceCount {
		return
	}
	o.healthLedger.Amounts[src] += amount
}

func (o *Organism) HealthLedger() HealthLedger { return o.healthLedger }
