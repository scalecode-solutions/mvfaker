package gen

// Generator is a pure function from entropy to a value. Composition happens
// through the combinators below; coherence (a field derived from another) is
// just Bind.
type Generator[T any] interface {
	Generate(s Source) T
}

type genFunc[T any] func(Source) T

func (f genFunc[T]) Generate(s Source) T { return f(s) }

// New adapts a plain function into a Generator.
func New[T any](f func(Source) T) Generator[T] { return genFunc[T](f) }

// Const always yields v.
func Const[T any](v T) Generator[T] { return New(func(Source) T { return v }) }

// Map transforms a generator's output.
func Map[A, B any](g Generator[A], f func(A) B) Generator[B] {
	return New(func(s Source) B { return f(g.Generate(s.Split())) })
}

// Bind sequences generators with a data dependency — the spine of coherence.
// The first value is drawn, then it chooses the next generator.
func Bind[A, B any](g Generator[A], f func(A) Generator[B]) Generator[B] {
	return New(func(s Source) B {
		a := g.Generate(s.Split())
		return f(a).Generate(s.Split())
	})
}

// OneOf picks one generator uniformly.
func OneOf[T any](gs ...Generator[T]) Generator[T] {
	return New(func(s Source) T {
		i := s.Draw(uint64(len(gs)))
		return gs[i].Generate(s.Split())
	})
}

// W is a weighted choice for Weighted.
type W[T any] struct {
	Weight int
	Gen    Generator[T]
}

// Weighted samples by weight. Draw==0 lands on the first choice, so order your
// choices simplest-first to keep shrinking well-behaved.
func Weighted[T any](choices ...W[T]) Generator[T] {
	total := 0
	for _, c := range choices {
		if c.Weight > 0 {
			total += c.Weight
		}
	}
	return New(func(s Source) T {
		if total <= 0 {
			var zero T
			return zero
		}
		r := int(s.Draw(uint64(total)))
		for _, c := range choices {
			if c.Weight <= 0 {
				continue
			}
			if r < c.Weight {
				return c.Gen.Generate(s.Split())
			}
			r -= c.Weight
		}
		return choices[len(choices)-1].Gen.Generate(s.Split())
	})
}

// Slice generates a slice whose length comes from n.
func Slice[T any](n Generator[int], g Generator[T]) Generator[[]T] {
	return New(func(s Source) []T {
		count := n.Generate(s.Split())
		if count < 0 {
			count = 0
		}
		out := make([]T, count)
		for i := range out {
			out[i] = g.Generate(s.Split())
		}
		return out
	})
}

// List generates up to max elements, each in its own subtree (a per-element
// "continue?" draw plus the element). Because every element is a distinct child,
// the tree shrinker can prune any element — including from the middle — cleanly.
// Prefer this over Slice when the values feed property tests.
func List[T any](max int, g Generator[T]) Generator[[]T] {
	if max < 0 {
		max = 0
	}
	return New(func(s Source) []T {
		out := make([]T, 0, max)
		for len(out) < max {
			c := s.Split()        // one span per element: holds the continue-bit + value
			if c.Draw(100) < 30 { // ~30% stop
				break
			}
			out = append(out, g.Generate(c.Split()))
		}
		return out
	})
}

// Optional yields nil with probability ~ (1-p). p in [0,1]. Draw biased so that
// "present" is the simpler value the shrinker won't strip.
func Optional[T any](g Generator[T], p float64) Generator[*T] {
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	const scale = 1_000_000
	thresh := uint64(p * scale)
	return New(func(s Source) *T {
		if s.Draw(scale) < thresh {
			v := g.Generate(s.Split())
			return &v
		}
		return nil
	})
}
