package population

import (
	"image/color"
	"math/rand"
	"testing"

	"github.com/Zebbeni/protozoa/organism"
)

// buildTree makes a synthetic descendant tree of nodes entries spread
// over cycles, with lifetimes short enough that a realistic fraction is
// alive at any one cycle.
func buildTree(nodes, cycles int) (map[int]*organism.DescendantNode, []int) {
	rng := rand.New(rand.NewSource(1))
	root := &organism.DescendantNode{ID: 0, Color: color.White, StartCycle: 0}
	all := []*organism.DescendantNode{root}
	for i := 1; i < nodes; i++ {
		parent := all[rng.Intn(len(all))]
		start := min(parent.StartCycle+rng.Intn(cycles/8+1), cycles)
		n := &organism.DescendantNode{ID: i, Color: color.White, StartCycle: start}
		if life := rng.Intn(cycles / 4); start+life < cycles {
			n.EndCycle = start + life
		}
		parent.AddChild(n)
		all = append(all, n)
	}
	return map[int]*organism.DescendantNode{0: root}, []int{0}
}

func ids(nodes []*organism.DescendantNode) []int {
	out := make([]int, len(nodes))
	for i, n := range nodes {
		out[i] = n.ID
	}
	return out
}

// TestAliveCacheMatchesWalk: cached answers, hit or miss, are what
// walking the trees gives.
func TestAliveCacheMatchesWalk(t *testing.T) {
	trees, roots := buildTree(2000, 4000)
	cache := NewAliveCache()
	for pass := 0; pass < 2; pass++ { // second pass reads the cache
		for cycle := 0; cycle <= 4000; cycle += 200 {
			want := collectAliveInTrees(trees, roots, cycle, nil)
			got := cache.AliveAt(trees, roots, cycle)
			if len(got) != len(want) {
				t.Fatalf("pass %d cycle %d: %d alive, want %d", pass, cycle, len(got), len(want))
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("pass %d cycle %d: organism %d differs", pass, cycle, i)
				}
			}
			if n := cache.CountAt(trees, roots, cycle); n != len(want) {
				t.Fatalf("pass %d cycle %d: count %d, want %d", pass, cycle, n, len(want))
			}
		}
	}
}

// TestAliveCacheKeepsItsOwnCopies: a cached set isn't disturbed by later
// misses reusing the walk buffer.
func TestAliveCacheKeepsItsOwnCopies(t *testing.T) {
	trees, roots := buildTree(2000, 4000)
	cache := NewAliveCache()
	first := ids(cache.AliveAt(trees, roots, 400))
	for cycle := 600; cycle <= 3000; cycle += 200 {
		cache.AliveAt(trees, roots, cycle)
	}
	if got := ids(cache.AliveAt(trees, roots, 400)); len(got) != len(first) {
		t.Fatalf("cached set changed length from %d to %d", len(first), len(got))
	} else {
		for i := range first {
			if got[i] != first[i] {
				t.Fatalf("cached set changed at %d: %d, want %d", i, got[i], first[i])
			}
		}
	}
}

// TestAliveCacheInvalidateDropsEverything: after the trees are rebuilt,
// nothing stale is handed back.
func TestAliveCacheInvalidateDropsEverything(t *testing.T) {
	trees, roots := buildTree(500, 1000)
	cache := NewAliveCache()
	cache.AliveAt(trees, roots, 500)
	cache.Invalidate()
	if len(cache.alive) != 0 || len(cache.counts) != 0 || cache.pointers != 0 {
		t.Errorf("after Invalidate: %d sets, %d counts, %d pointers", len(cache.alive), len(cache.counts), cache.pointers)
	}

	other, otherRoots := buildTree(400, 1000)
	want := len(collectAliveInTrees(other, otherRoots, 500, nil))
	if got := len(cache.AliveAt(other, otherRoots, 500)); got != want {
		t.Errorf("after Invalidate, cycle 500 has %d alive, want %d from the new trees", got, want)
	}
}

// TestAliveCacheStaysWithinBudget: a long run evicts rather than growing
// without limit.
func TestAliveCacheStaysWithinBudget(t *testing.T) {
	cache := NewAliveCache()
	big := make([]*organism.DescendantNode, 100_000)
	for i := range big {
		big[i] = &organism.DescendantNode{ID: i}
	}
	for cycle := 0; cycle < 200; cycle++ {
		cache.store(cycle, big)
	}
	if cache.pointers > maxCachedPointers {
		t.Errorf("cache holds %d pointers, over the %d budget", cache.pointers, maxCachedPointers)
	}
	// Past the budget the cache still answers, by walking.
	trees, roots := buildTree(500, 1000)
	want := len(collectAliveInTrees(trees, roots, 500, nil))
	if got := len(cache.AliveAt(trees, roots, 500)); got != want {
		t.Errorf("a full cache returned %d alive, want %d", got, want)
	}
}

const (
	benchNodes    = 40000
	benchCycles   = 20000
	benchInterval = 20
)

// BenchmarkColourSwitchUncached is what a population colour switch used
// to cost: a walk per bar for the y-axis pass and another for drawing.
func BenchmarkColourSwitchUncached(b *testing.B) {
	trees, roots := buildTree(benchNodes, benchCycles)
	bars := benchCycles / benchInterval
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf []*organism.DescendantNode
		for bar := 0; bar < bars; bar++ {
			countAliveInTrees(trees, roots, bar*benchInterval)
		}
		for bar := 0; bar < bars; bar++ {
			buf = collectAliveInTrees(trees, roots, bar*benchInterval, buf[:0])
		}
	}
}

// BenchmarkColourSwitchCached is the same work against a warm cache.
func BenchmarkColourSwitchCached(b *testing.B) {
	trees, roots := buildTree(benchNodes, benchCycles)
	bars := benchCycles / benchInterval
	cache := NewAliveCache()
	for bar := 0; bar < bars; bar++ {
		cache.AliveAt(trees, roots, bar*benchInterval)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for bar := 0; bar < bars; bar++ {
			cache.CountAt(trees, roots, bar*benchInterval)
		}
		for bar := 0; bar < bars; bar++ {
			cache.AliveAt(trees, roots, bar*benchInterval)
		}
	}
}
