package organism

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lucasb-eyer/go-colorful"

	c "github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// DesignsDir is where saved organism designs live, one JSON file each.
const DesignsDir = "designs"

type Design struct {
	Name           string `json:"name"`
	Color          string `json:"color"`
	SecondaryColor string `json:"secondary_color"`

	MaxSize                float64 `json:"max_size"`
	SpawnHealth            float64 `json:"spawn_health"`
	MinHealthToSpawn       float64 `json:"min_health_to_spawn"`
	MinCyclesBetweenSpawns int     `json:"min_cycles_between_spawns"`
	IdealPh                float64 `json:"ideal_ph"`

	// Abilities in physiology.AllAbilities order.
	Abilities    []int  `json:"abilities"`
	DecisionTree string `json:"decision_tree"`
}

func NewDesign(name string) Design {
	scores := physiology.BalancedScores()
	abilities := make([]int, physiology.AbilityCount)
	for _, a := range physiology.AllAbilities {
		abilities[a] = scores[a]
	}
	return Design{
		Name:                   name,
		Color:                  "#7fb2d9",
		SecondaryColor:         "#d9c27f",
		MaxSize:                c.MinimumMaxSize(),
		SpawnHealth:            c.MaximumInitialSpawnHealth(),
		MinHealthToSpawn:       c.MinimumMaxSize() / 2,
		MinCyclesBetweenSpawns: 1,
		IdealPh:                c.InitialPh(),
		Abilities:              abilities,
		DecisionTree:           StarterTree().Serialize(),
	}
}

// StarterTree is the tree a new design opens with: one condition and the two actions it chooses between.
func StarterTree() *d.Tree {
	root := d.NodeFromCondition(d.CanChemosynthesizeHere)
	root.YesNode = d.NodeFromAction(d.ActChemosynthesis)
	root.NoNode = d.NodeFromAction(d.ActMove)
	return d.TreeFromNode(root)
}

// Scores converts the design's ability list, checking the budget.
func (ds Design) Scores() (physiology.Scores, error) {
	return physiology.ScoresFromSlice(ds.Abilities)
}

func (ds Design) Tree() (*d.Tree, error) {
	tree := d.DeserializeTree(ds.DecisionTree)
	if tree == nil {
		return nil, fmt.Errorf("design %q: decision tree is unreadable", ds.Name)
	}
	return tree, nil
}

// Traits converts the design into the traits an organism carries, clamping each value into the range the current configuration allows.
func (ds Design) Traits() (Traits, error) {
	scores, err := ds.Scores()
	if err != nil {
		return Traits{}, fmt.Errorf("design %q: %w", ds.Name, err)
	}
	primary, err := colorful.Hex(ds.Color)
	if err != nil {
		return Traits{}, fmt.Errorf("design %q: colour %q: %w", ds.Name, ds.Color, err)
	}
	secondary, err := colorful.Hex(ds.SecondaryColor)
	if err != nil {
		return Traits{}, fmt.Errorf("design %q: secondary colour %q: %w", ds.Name, ds.SecondaryColor, err)
	}

	maxSize := clampFloat(ds.MaxSize, c.MinimumMaxSize(), c.MaximumMaxSize())
	spawnHealth := clampFloat(ds.SpawnHealth, c.MinSpawnHealth(), spawnHealthCap(maxSize))
	return Traits{
		OrganismColor:          primary,
		SecondaryColor:         secondary,
		MaxSize:                maxSize,
		SpawnHealth:            spawnHealth,
		MinHealthToSpawn:       clampFloat(ds.MinHealthToSpawn, spawnThresholdFloor(spawnHealth, maxSize), maxSize),
		MinCyclesBetweenSpawns: clampInt(ds.MinCyclesBetweenSpawns, 0, c.MaxCyclesBetweenSpawns()),
		IdealPh:                clampFloat(ds.IdealPh, c.MinIdealPh(), c.MaxIdealPh()),
		Abilities:              scores,
	}, nil
}

// TreeSize is how many nodes the design's decision tree holds, or 0 if it won't parse.
func (ds Design) TreeSize() int {
	tree, err := ds.Tree()
	if err != nil {
		return 0
	}
	return tree.Size()
}

// ExceedsTreeLimit reports whether the design's tree is bigger than limit allows.
func (ds Design) ExceedsTreeLimit(limit int) bool {
	return limit > 0 && ds.TreeSize() > limit
}

func (ds Design) Validate() error {
	if strings.TrimSpace(ds.Name) == "" {
		return fmt.Errorf("a design needs a name")
	}
	if _, err := ds.Traits(); err != nil {
		return err
	}
	if _, err := ds.Tree(); err != nil {
		return err
	}
	return nil
}

func clampFloat(v, lo, hi float64) float64 { return min(hi, max(lo, v)) }
func clampInt(v, lo, hi int) int           { return min(hi, max(lo, v)) }

// NewDesigned builds an organism from a design, at the given location and facing a random direction.
func NewDesigned(rng *simrand.RNG, id int, point utils.Point, api LookupAPI, ds Design) (*Organism, error) {
	traits, err := ds.Traits()
	if err != nil {
		return nil, err
	}
	tree, err := ds.Tree()
	if err != nil {
		return nil, err
	}
	return &Organism{
		ID:                 id,
		Health:             traits.SpawnHealth,
		Size:               traits.SpawnHealth,
		Location:           point,
		Direction:          utils.GetRandomDirection(rng),
		OriginalAncestorID: id,

		traits:       traits,
		decisionTree: tree,
		action:       d.ActChemosynthesis,
		appearance:   physiology.AppearanceFor(traits.Abilities, tree),

		lookupAPI: api,
	}, nil
}

func DesignFileName(name string) string {
	clean := strings.ToLower(strings.TrimSpace(name))
	clean = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r == ' ', r == '-', r == '_':
			return '-'
		default:
			return -1
		}
	}, clean)
	if clean == "" {
		clean = "design"
	}
	return clean + ".json"
}

// SaveDesign writes a design into dir, creating the directory if needed.
func SaveDesign(dir string, ds Design) (string, error) {
	if err := ds.Validate(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, DesignFileName(ds.Name))
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func DeleteDesign(dir, name string) error {
	err := os.Remove(filepath.Join(dir, DesignFileName(name)))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// LoadDesigns reads every design in dir, sorted by name.
func LoadDesigns(dir string) []Design {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Design
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var ds Design
		if err := json.Unmarshal(data, &ds); err != nil {
			continue
		}
		if ds.Validate() != nil {
			continue
		}
		out = append(out, ds)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// DesignsByName picks the named designs out of dir, in the order named, skipping any that aren't there.
func DesignsByName(dir string, names []string) []Design {
	all := LoadDesigns(dir)
	byName := make(map[string]Design, len(all))
	for _, ds := range all {
		byName[ds.Name] = ds
	}
	var out []Design
	for _, n := range names {
		if ds, ok := byName[n]; ok {
			out = append(out, ds)
		}
	}
	return out
}

// OversizedDesigns names the designs in the list whose decision trees are over limit.
func OversizedDesigns(designs []Design, limit int) []string {
	var out []string
	for _, ds := range designs {
		if ds.ExceedsTreeLimit(limit) {
			out = append(out, ds.Name)
		}
	}
	return out
}
