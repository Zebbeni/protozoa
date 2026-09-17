package population

import (
	"sync"

	"github.com/Zebbeni/protozoa/organism"
)

// AliveCache remembers which organisms were alive at each graphed cycle.
//
// Finding them means walking every descendant tree, once per bar, and
// that walk is what makes a population render slow — around 300ms per
// pass over a 20,000-cycle run. The colouring only decides what colour
// each organism already found gets, so every colour mode walks the same
// trees for the same answer. One cache shared by the population
// renderers turns a colour switch from two full walks (the y-axis pass
// and the drawing pass) into a redraw.
//
// Alive sets never change as a run plays forward: an organism alive at a
// past cycle stays alive at that cycle whatever happens later. They do
// change when the trees themselves are rebuilt — a replay seek restores
// them from a snapshot — so callers must Invalidate then.
//
// A nil *AliveCache is usable and simply computes every answer, which is
// what the selection sub-tree renderers do: their alive sets are a
// different question (one sub-tree, not the world) and they are rebuilt
// whenever the selection changes.
type AliveCache struct {
	mu sync.Mutex
	// counts is every cycle's alive count; alive holds the organisms
	// themselves for the cycles actually drawn as columns. Counts are
	//4 bytes a bar, so they're kept for every bar; the alive sets are 8
	// bytes an organism and bounded by maxCachedPointers.
	counts   map[int]int
	alive    map[int][]*organism.DescendantNode
	pointers int
	// scratch is the walk buffer for cache misses, reused between them.
	scratch []*organism.DescendantNode
}

// maxCachedPointers bounds the cached alive sets: 8M pointers, so 64MiB
// on a 64-bit build. A 20,000-cycle run with 4,000 organisms alive fits
// inside it. Past the budget the cache stops taking new cycles and the
// rest are walked, rather than evicting: passes run through the cycles in
// order, so evicting the oldest would throw away exactly what the next
// pass reads first and cache nothing usefully.
const maxCachedPointers = 8 << 20

// NewAliveCache returns an empty cache.
func NewAliveCache() *AliveCache {
	return &AliveCache{counts: map[int]int{}, alive: map[int][]*organism.DescendantNode{}}
}

// Invalidate drops everything cached. Call it whenever the descendant
// trees are rebuilt.
func (c *AliveCache) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts = map[int]int{}
	c.alive = map[int][]*organism.DescendantNode{}
	c.pointers = 0
}

// CountAt is how many organisms were alive at cycle, from the cache when
// it knows and by walking the trees otherwise.
func (c *AliveCache) CountAt(trees map[int]*organism.DescendantNode, ancestorIDs []int, cycle int) int {
	if c == nil {
		return countAliveInTrees(trees, ancestorIDs, cycle)
	}
	c.mu.Lock()
	if n, ok := c.counts[cycle]; ok {
		c.mu.Unlock()
		return n
	}
	c.mu.Unlock()

	n := countAliveInTrees(trees, ancestorIDs, cycle)
	c.mu.Lock()
	c.counts[cycle] = n
	c.mu.Unlock()
	return n
}

// AliveAt returns the organisms alive at cycle. The slice belongs to the
// cache, so callers must only read it.
func (c *AliveCache) AliveAt(trees map[int]*organism.DescendantNode, ancestorIDs []int, cycle int) []*organism.DescendantNode {
	c.mu.Lock()
	if alive, ok := c.alive[cycle]; ok {
		c.mu.Unlock()
		return alive
	}
	scratch := c.scratch
	c.mu.Unlock()

	alive := collectAliveInTrees(trees, ancestorIDs, cycle, scratch[:0])
	kept := c.store(cycle, alive)

	c.mu.Lock()
	if cap(alive) > cap(c.scratch) {
		c.scratch = alive[:0]
	}
	c.mu.Unlock()
	if kept == nil {
		// Over budget: the walk buffer is the answer. Safe to hand back
		// read-only, since the next call is what overwrites it.
		return alive
	}
	return kept
}

// store keeps a copy of a cycle's alive set — the caller's slice is a
// reused walk buffer — and returns it, or returns nil when the cache is
// full and the caller should use its own copy.
func (c *AliveCache) store(cycle int, alive []*organism.DescendantNode) []*organism.DescendantNode {
	c.mu.Lock()
	if existing, ok := c.alive[cycle]; ok {
		c.mu.Unlock()
		return existing
	}
	full := c.pointers+len(alive) > maxCachedPointers
	c.counts[cycle] = len(alive)
	c.mu.Unlock()
	if full {
		return nil
	}

	kept := make([]*organism.DescendantNode, len(alive))
	copy(kept, alive)

	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.alive[cycle]; ok {
		return existing
	}
	c.alive[cycle] = kept
	c.pointers += len(kept)
	return kept
}

// collectAliveInTrees appends every organism alive at cycle to dst, in
// the order the renderer stacks them.
func collectAliveInTrees(trees map[int]*organism.DescendantNode, ancestorIDs []int, cycle int, dst []*organism.DescendantNode) []*organism.DescendantNode {
	for _, id := range ancestorIDs {
		if root := trees[id]; root != nil {
			collectAlive(root, cycle, &dst)
		}
	}
	return dst
}
