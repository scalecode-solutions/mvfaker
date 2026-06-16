package gen

import "math/rand/v2"

// Result reports a property check.
type Result[T any] struct {
	Failed  bool // a counterexample was found
	Value   T    // the (shrunk) counterexample, valid when Failed
	Runs    int  // generated cases tried before failure (or total if passed)
	Shrinks int  // successful shrink steps applied
}

// Check runs prop against generated values. prop returns true when the property
// holds. On the first failure it shrinks the recorded draw-sequence toward the
// simplest value that still fails, then returns it.
func Check[T any](seed uint64, runs int, g Generator[T], prop func(T) bool) Result[T] {
	rng := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))

	var failTape []uint64
	var failVal T
	found := false
	tried := 0
	for i := 0; i < runs; i++ {
		tried++
		s := newRecSource(rng)
		v := g.Generate(s)
		if !prop(v) {
			failTape = append([]uint64(nil), s.trace...)
			failVal = v
			found = true
			break
		}
	}
	if !found {
		return Result[T]{Failed: false, Runs: tried}
	}

	// Greedy shrink. Two move kinds, replaying after each and keeping any trial
	// that still fails: (1) lower a draw toward 0, (2) delete a draw entirely —
	// deletion is what collapses structure (e.g. shortens a generated slice),
	// since dropping the length/continuation draw re-derives a smaller value.
	// Restart whenever something improves until a full pass is dry.
	shrinks := 0
	improved := true
	for improved {
		improved = false

		// (1) lower each draw
		for i := 0; i < len(failTape); i++ {
			for _, cand := range candidates(failTape[i]) {
				trial := append([]uint64(nil), failTape...)
				trial[i] = cand
				if adopt(g, prop, trial, &failTape, &failVal) {
					shrinks++
					improved = true
					break
				}
			}
		}

		// (2) delete each draw
		for i := 0; i < len(failTape); i++ {
			trial := make([]uint64, 0, len(failTape)-1)
			trial = append(trial, failTape[:i]...)
			trial = append(trial, failTape[i+1:]...)
			if adopt(g, prop, trial, &failTape, &failVal) {
				shrinks++
				improved = true
				break
			}
		}
	}
	return Result[T]{Failed: true, Value: failVal, Runs: tried, Shrinks: shrinks}
}

// adopt replays trial; if it still fails, it becomes the new current case
// (adopting the realized trace, since the generator may draw differently).
func adopt[T any](g Generator[T], prop func(T) bool, trial []uint64, tape *[]uint64, val *T) bool {
	s := replaySource(trial)
	v := g.Generate(s)
	if prop(v) {
		return false
	}
	*tape = append([]uint64(nil), s.trace...)
	*val = v
	return true
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
	if v > 0 {
		out = append(out, v-1)
	}
	return out
}
