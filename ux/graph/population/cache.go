package population

import (
	"sync"

	"github.com/Zebbeni/protozoa/organism"
)

type AliveCache struct {
	mu       sync.Mutex
	counts   map[int]int
	alive    map[int][]*organism.DescendantNode
	pointers int
	// scratch is the walk buffer for cache misses, reused between them.
	scratch []*organism.DescendantNode
}

// maxCachedPointers bounds the cached alive sets: 8M pointers, so 64MiB on a 64-bit build.
const maxCachedPointers = 8 << 20

func NewAliveCache() *AliveCache {
	return &AliveCache{counts: map[int]int{}, alive: map[int][]*organism.DescendantNode{}}
}

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

// CountAt is how many organisms were alive at cycle, from the cache when it knows and by walking the trees otherwise.
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
		// Over budget: the walk buffer is the answer.
		return alive
	}
	return kept
}

// store keeps a copy of a cycle's alive set.
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

// collectAliveInTrees appends every organism alive at cycle to dst, in the order the renderer stacks them.
func collectAliveInTrees(trees map[int]*organism.DescendantNode, ancestorIDs []int, cycle int, dst []*organism.DescendantNode) []*organism.DescendantNode {
	for _, id := range ancestorIDs {
		if root := trees[id]; root != nil {
			collectAlive(root, cycle, &dst)
		}
	}
	return dst
}
