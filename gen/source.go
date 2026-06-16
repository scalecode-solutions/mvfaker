// Package gen is the pure value layer: a Source of entropy and Generators that
// decode it into values. Nothing here holds cross-value state — that lives in
// the dataset layer. See DESIGN.md.
package gen

// Source is the only place randomness enters the system. The primitive is
// splittable, not merely drawable: every structural step (a field, a slice
// index, a Bind) takes a fresh child via Split. Invariant: the all-zero draw
// decodes to each generator's canonical-simplest value, so shrinking always has
// a direction to move toward.
type Source interface {
	// Draw returns a value in [0, n). n must be > 0.
	Draw(n uint64) uint64
	// Split returns an independent child stream.
	Split() Source
}

func splitmix64(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}

// --- Positional source: value = pure function of (seed, path). -------------
//
// Used for --fixt/--mock/--seed. Reproducible, order-independent and trivially
// parallel: row #1,000,000 is generable instantly via At(seed, entity, row)
// without touching rows 0..999,999.

type posSource struct {
	key   uint64
	draws uint64
	kids  uint64
}

// Positional returns a root source for a seed.
func Positional(seed uint64) Source { return &posSource{key: splitmix64(seed)} }

// At derives the source at a deterministic path under seed. This is how the
// dataset runner addresses an individual row/cell in parallel.
func At(seed uint64, path ...uint64) Source {
	k := splitmix64(seed)
	for _, p := range path {
		k = splitmix64(k + splitmix64(p+1))
	}
	return &posSource{key: k}
}

func (s *posSource) Draw(n uint64) uint64 {
	if n == 0 {
		return 0
	}
	h := splitmix64(s.key ^ splitmix64(s.draws+1))
	s.draws++
	return h % n
}

func (s *posSource) Split() Source {
	child := splitmix64(s.key + 0x9E3779B97F4A7C15*(s.kids+1))
	s.kids++
	return &posSource{key: child}
}

// The recording source used for property testing + shrinking is tree-structured;
// it lives in shrink.go alongside the shrinker.
