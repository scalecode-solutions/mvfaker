package schema

import "strings"

// Uniqueness lives here, in the dataset layer — never inside a generator. The
// trick that keeps it parallel and deterministic: the unique value is derived
// from the row index, which is already unique, so no mutable "seen" set is
// needed and rows don't depend on each other.
//
//   - int fields  → a Feistel permutation of [0,count): unique, shuffled,
//     no suffix. The base numeric value is replaced (documented behaviour).
//   - string fields → the base value plus a compact index-derived tag, woven in
//     before '@' for emails so the result stays well-formed and still coherent.
func makeUnique(v any, idx, count int, seed uint64, entity, field string) any {
	key := hashStr(entity+"."+field) ^ seed ^ 0x9E3779B97F4A7C15
	switch x := v.(type) {
	case int:
		return permuteIndex(idx, count, key)
	case string:
		tag := base36(permuteIndex(idx, count, key))
		if at := strings.IndexByte(x, '@'); at >= 0 {
			return x[:at] + "+" + tag + x[at:]
		}
		return x + "." + tag
	default:
		return v // type we can't uniquify; left as-is
	}
}

func umix(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}

// permuteIndex maps i (in [0,n)) to a distinct value in [0,n) via a
// format-preserving Feistel permutation with cycle-walking. Bijective, so the
// whole [0,n) range is covered exactly once — guaranteed uniqueness.
func permuteIndex(i, n int, key uint64) int {
	if n <= 1 || i < 0 {
		return i
	}
	half := halfBits(n)
	v := uint64(i)
	for {
		v = feistel(v, half, key)
		if int(v) < n {
			return int(v)
		}
		// cycle-walk: same permutation re-applied until back in range
	}
}

func feistel(x uint64, half uint, key uint64) uint64 {
	mask := (uint64(1) << half) - 1
	l := (x >> half) & mask
	r := x & mask
	for round := uint64(0); round < 4; round++ {
		f := umix(r^key^round) & mask
		l, r = r, l^f
	}
	return (l << half) | r
}

func halfBits(n int) uint {
	var b uint
	for (uint64(1) << (2 * b)) < uint64(n) {
		b++
	}
	return b
}

func base36(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	var buf [13]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%36]
		n /= 36
	}
	return string(buf[i:])
}
