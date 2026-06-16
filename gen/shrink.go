package gen

import "math/rand/v2"

// Result reports a property check.
type Result[T any] struct {
	Failed  bool // a counterexample was found
	Value   T    // the (shrunk) counterexample, valid when Failed
	Runs    int  // generated cases tried before failure (or total if passed)
	Shrinks int  // successful shrink steps applied
}

// --- Tree-structured recording source --------------------------------------
//
// Draws are recorded in a tree that mirrors Split(): each Split is a child node,
// each Draw a slot on the current node. This is what makes shrinking structural
// — a whole subtree (e.g. one List element) can be pruned without disturbing its
// siblings, which a flat tape cannot do (deleting mid-tape shifts everything).

type node struct {
	draws    []uint64
	children []*node
}

func cloneNode(n *node) *node {
	c := &node{draws: append([]uint64(nil), n.draws...)}
	for _, ch := range n.children {
		c.children = append(c.children, cloneNode(ch))
	}
	return c
}

type treeSource struct {
	node *node
	di   int        // draw cursor
	ci   int        // child cursor
	rng  *rand.Rand // non-nil = record fresh draws; nil = replay (0 past the tree)
}

func (t *treeSource) Draw(n uint64) uint64 {
	if n == 0 {
		return 0
	}
	if t.di < len(t.node.draws) {
		v := t.node.draws[t.di] % n
		t.di++
		return v
	}
	var v uint64
	if t.rng != nil {
		v = t.rng.Uint64() % n
		t.node.draws = append(t.node.draws, v)
	}
	t.di++
	return v
}

func (t *treeSource) Split() Source {
	if t.ci < len(t.node.children) {
		c := t.node.children[t.ci]
		t.ci++
		return &treeSource{node: c, rng: t.rng}
	}
	c := &node{}
	t.node.children = append(t.node.children, c)
	t.ci++
	return &treeSource{node: c, rng: t.rng}
}

// Check runs prop against generated values. On the first failure it shrinks the
// recorded draw-tree toward the simplest value that still fails, then returns it.
func Check[T any](seed uint64, runs int, g Generator[T], prop func(T) bool) Result[T] {
	rng := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))

	var root *node
	var failVal T
	found := false
	tried := 0
	for i := 0; i < runs; i++ {
		tried++
		r := &node{}
		v := g.Generate(&treeSource{node: r, rng: rng})
		if !prop(v) {
			root, failVal, found = r, v, true
			break
		}
	}
	if !found {
		return Result[T]{Failed: false, Runs: tried}
	}

	shrinks := 0
	for {
		next, nv, improved := shrinkPass(g, prop, root)
		if !improved {
			break
		}
		root, failVal = next, nv
		shrinks++
	}
	return Result[T]{Failed: true, Value: failVal, Runs: tried, Shrinks: shrinks}
}

// shrinkPass tries one improvement, simplest reductions first: prune a subtree
// (structural — removes a whole List element), then lower individual draws. A
// trial is accepted only if it still fails AND is strictly smaller than the
// current tree — the size gate guarantees termination and rejects "pruning a
// child the generator immediately regrows on replay" (same size → not progress).
func shrinkPass[T any](g Generator[T], prop func(T) bool, root *node) (*node, T, bool) {
	var zero T
	cur := measure(root)

	// 1) prune whole subtrees — biggest, most structural reductions
	for _, p := range nodePaths(root) {
		n := nodeAt(root, p)
		for ci := len(n.children) - 1; ci >= 0; ci-- {
			trial := cloneNode(root)
			tn := nodeAt(trial, p)
			tn.children = append(tn.children[:ci], tn.children[ci+1:]...)
			if v, ok := accepts(g, prop, trial, cur); ok {
				return trial, v, true
			}
		}
	}

	// 2) lower individual draws toward 0
	for _, p := range nodePaths(root) {
		n := nodeAt(root, p)
		for di := 0; di < len(n.draws); di++ {
			for _, cand := range candidates(n.draws[di]) {
				trial := cloneNode(root)
				nodeAt(trial, p).draws[di] = cand
				if v, ok := accepts(g, prop, trial, cur); ok {
					return trial, v, true
				}
			}
		}
	}
	return root, zero, false
}

// accepts replays trial (no rng → 0 past the recorded tree) and reports the
// value plus whether to adopt it: the property must still fail and the replayed
// tree must be strictly smaller than cur. Replay mutates trial to reflect what
// the generator actually read, so the adopted tree stays consistent.
func accepts[T any](g Generator[T], prop func(T) bool, trial *node, cur treeSize) (T, bool) {
	v := g.Generate(&treeSource{node: trial})
	if prop(v) {
		return v, false
	}
	return v, less(measure(trial), cur)
}

type treeSize struct {
	nodes int
	draws int
	sum   uint64
}

func measure(n *node) treeSize {
	s := treeSize{nodes: 1, draws: len(n.draws)}
	for _, d := range n.draws {
		s.sum += d
	}
	for _, c := range n.children {
		cs := measure(c)
		s.nodes += cs.nodes
		s.draws += cs.draws
		s.sum += cs.sum
	}
	return s
}

// less reports whether a is strictly simpler than b (lexicographic: fewer nodes,
// then fewer draws, then smaller draw-value sum).
func less(a, b treeSize) bool {
	if a.nodes != b.nodes {
		return a.nodes < b.nodes
	}
	if a.draws != b.draws {
		return a.draws < b.draws
	}
	return a.sum < b.sum
}

func nodeAt(root *node, path []int) *node {
	n := root
	for _, i := range path {
		n = n.children[i]
	}
	return n
}

// nodePaths enumerates every node as a path of child indices (root = empty path).
func nodePaths(root *node) [][]int {
	var out [][]int
	var walk func(n *node, p []int)
	walk = func(n *node, p []int) {
		out = append(out, append([]int(nil), p...))
		for i := range n.children {
			walk(n.children[i], append(p, i))
		}
	}
	walk(root, nil)
	return out
}

// candidates proposes strictly-smaller replacements for a draw, simplest first.
func candidates(v uint64) []uint64 {
	if v == 0 {
		return nil
	}
	out := []uint64{0}
	if v > 1 {
		out = append(out, v/2)
	}
	out = append(out, v-1)
	return out
}
