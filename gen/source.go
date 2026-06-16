// Package gen is the pure value layer: a Source of entropy and Generators that
// decode it into values. Nothing here holds cross-value state — that lives in
// the dataset layer. See DESIGN.md.
package gen

import "math/rand/v2"

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

// --- Recording source: linear, for property testing + shrinking. -----------
//
// Records every draw so the choice-sequence becomes the seed; the shrinker
// minimizes that sequence and replays. Split returns the same stream (v0
// shrinking is over a flat sequence; structured/tree shrinking is a rough edge
// noted in DESIGN.md).

type recSource struct {
	rng    *rand.Rand
	forced []uint64 // replay tape; consumed positionally
	pos    int
	trace  []uint64 // realized draws, in [0,n)
}

func newRecSource(rng *rand.Rand) *recSource { return &recSource{rng: rng} }
func replaySource(tape []uint64) *recSource  { return &recSource{forced: tape} }

func (s *recSource) Draw(n uint64) uint64 {
	if n == 0 {
		return 0
	}
	var v uint64
	if s.pos < len(s.forced) {
		v = s.forced[s.pos] % n
	} else if s.rng != nil {
		v = s.rng.Uint64() % n
	} // else: past the tape with no rng -> 0 (simplest)
	s.pos++
	s.trace = append(s.trace, v)
	return v
}

func (s *recSource) Split() Source { return s }
