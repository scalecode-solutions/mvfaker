package gen

// IntRange yields an int in [min, max] inclusive.
func IntRange(min, max int) Generator[int] {
	if max < min {
		min, max = max, min
	}
	span := uint64(max-min) + 1
	return New(func(s Source) int { return min + int(s.Draw(span)) })
}

// Float64Range yields a float in [min, max).
func Float64Range(min, max float64) Generator[float64] {
	if max < min {
		min, max = max, min
	}
	const res = 1 << 53
	return New(func(s Source) float64 {
		f := float64(s.Draw(res)) / float64(res)
		return min + f*(max-min)
	})
}

// NormalInt yields an int clustered around mode within [min,max], via a simple
// average-of-draws approximation (cheap, monotone-friendly for shrinking).
func NormalInt(min, max, mode int) Generator[int] {
	base := IntRange(min, max)
	return New(func(s Source) int {
		a := base.Generate(s.Split())
		b := base.Generate(s.Split())
		avg := (a + b) / 2
		// nudge toward mode
		return (avg + mode) / 2
	})
}

// Bool yields true with probability p.
func Bool(p float64) Generator[bool] {
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	const scale = 1_000_000
	thresh := uint64(p * scale)
	return New(func(s Source) bool { return s.Draw(scale) < thresh })
}

// Pick chooses one element of xs uniformly.
func Pick[T any](xs ...T) Generator[T] {
	return New(func(s Source) T {
		if len(xs) == 0 {
			var zero T
			return zero
		}
		return xs[s.Draw(uint64(len(xs)))]
	})
}

// PickSlice is Pick over an existing slice.
func PickSlice[T any](xs []T) Generator[T] {
	return New(func(s Source) T {
		if len(xs) == 0 {
			var zero T
			return zero
		}
		return xs[s.Draw(uint64(len(xs)))]
	})
}
