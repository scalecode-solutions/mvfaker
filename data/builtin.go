package data

import (
	"strings"

	"github.com/tmarq/mvfaker/gen"
)

func init() {
	Register("name.first", func(p Params) (MakeFn, error) {
		return func(any) gen.Generator[any] { return boxed(gen.PickSlice(firstNames)) }, nil
	})
	Register("name.last", func(p Params) (MakeFn, error) {
		return func(any) gen.Generator[any] { return boxed(gen.PickSlice(lastNames)) }, nil
	})
	Register("name.full", func(p Params) (MakeFn, error) {
		g := gen.New(func(s gen.Source) string {
			return gen.PickSlice(firstNames).Generate(s.Split()) + " " +
				gen.PickSlice(lastNames).Generate(s.Split())
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	// Coherent email: if it derives `from` a name, the local-part is built from
	// that name; otherwise a random local-part. This is the headline capability
	// no flat function-bag faker can do.
	Register("internet.email", func(p Params) (MakeFn, error) {
		return func(dep any) gen.Generator[any] {
			return gen.New(func(s gen.Source) any {
				local := ""
				if name, ok := dep.(string); ok && strings.TrimSpace(name) != "" {
					local = slug(name)
				} else {
					local = slug(gen.PickSlice(firstNames).Generate(s.Split()))
					local += itoa(int(s.Split().Draw(900) + 100))
				}
				dom := gen.PickSlice(domains).Generate(s.Split())
				return local + "@" + dom
			})
		}, nil
	})

	Register("number", func(p Params) (MakeFn, error) {
		min := p.Int("min", 0)
		max := p.Int("max", 100)
		switch p.Str("dist", "uniform") {
		case "normal":
			mode := p.Int("mode", (min+max)/2)
			g := gen.NormalInt(min, max, mode)
			return func(any) gen.Generator[any] { return boxed(g) }, nil
		default:
			g := gen.IntRange(min, max)
			return func(any) gen.Generator[any] { return boxed(g) }, nil
		}
	})

	Register("bool", func(p Params) (MakeFn, error) {
		g := gen.Bool(p.Float("p", 0.5))
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	Register("lorem.word", func(p Params) (MakeFn, error) {
		return func(any) gen.Generator[any] { return boxed(gen.PickSlice(words)) }, nil
	})
	Register("lorem.words", func(p Params) (MakeFn, error) {
		n := p.Int("n", 5)
		g := gen.New(func(s gen.Source) string {
			parts := make([]string, n)
			for i := range parts {
				parts[i] = gen.PickSlice(words).Generate(s.Split())
			}
			return strings.Join(parts, " ")
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	Register("uuid", func(p Params) (MakeFn, error) {
		g := gen.New(func(s gen.Source) string {
			var b strings.Builder
			for i := 0; i < 32; i++ {
				if i == 8 || i == 12 || i == 16 || i == 20 {
					b.WriteByte('-')
				}
				b.WriteByte(hexDigits[s.Draw(16)])
			}
			return b.String()
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})
}

func slug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", ".")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
