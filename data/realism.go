package data

import (
	"fmt"
	"strings"

	"github.com/scalecode-solutions/mvfaker/gen"
)

// The 249-country dataset (dataset_gen.go) is the authoritative reference: name,
// ISO codes, calling code, currency, capital, continent. City/region/postal
// detail comes from locale files (data/locales/, see locale.go), looked up by
// the country's ISO alpha-2; countries without a locale fall back to the capital
// and a generic postal format. Coherence rides the `from` mechanism: derive code
// / calling code / city / currency from a chosen country.

var fallbackStreets = []string{"Oak St", "Maple Ave", "Main St", "Park Rd", "Cedar Ln", "Elm St", "Hill Rd"}

var (
	countryByName = map[string]Country{}
	countryByA2   = map[string]Country{}
)

func init() {
	for _, c := range countries {
		countryByName[c.Name] = c
		countryByA2[c.A2] = c
	}
}

// resolveCountry returns the country named by dep, or a random one if dep is empty.
func resolveCountry(dep any, s gen.Source) Country {
	if name, ok := dep.(string); ok {
		if c, found := countryByName[name]; found {
			return c
		}
	}
	return countries[s.Draw(uint64(len(countries)))]
}

// expandFmt turns a pattern (# = digit, @ = uppercase letter) into a generator.
func expandFmt(format string) gen.Generator[string] {
	return gen.New(func(s gen.Source) string {
		var b strings.Builder
		for _, r := range format {
			switch r {
			case '#':
				b.WriteByte(byte('0' + s.Draw(10)))
			case '@':
				b.WriteByte(byte('A' + s.Draw(26)))
			default:
				b.WriteRune(r)
			}
		}
		return b.String()
	})
}

// zipfPick favors earlier entries (weight ~ 1/rank), so common names dominate —
// realistic skew. The name tables are pre-sorted by census frequency.
func zipfPick(xs []string) gen.Generator[string] {
	cum := make([]float64, len(xs))
	total := 0.0
	for i := range xs {
		total += 1.0 / float64(i+1)
		cum[i] = total
	}
	return gen.New(func(s gen.Source) string {
		const scale = 1 << 20
		r := float64(s.Draw(scale)) / float64(scale) * total
		lo, hi := 0, len(xs)-1
		for lo < hi {
			mid := (lo + hi) / 2
			if r <= cum[mid] {
				hi = mid
			} else {
				lo = mid + 1
			}
		}
		return xs[lo]
	})
}

// cgen builds a from-country generator: derive a field from the chosen country.
func cgen(f func(Country) string) MakeFn {
	return func(dep any) gen.Generator[any] {
		return gen.New(func(s gen.Source) any { return f(resolveCountry(dep, s)) })
	}
}

func reg0(name string, mk MakeFn) {
	Register(name, func(Params) (MakeFn, error) { return mk, nil })
}

func init() {
	reg0("country", cgen(func(c Country) string { return c.Name }))
	reg0("country.code", cgen(func(c Country) string { return c.A2 }))
	reg0("country.code3", cgen(func(c Country) string { return c.A3 }))
	reg0("country.callingcode", cgen(func(c Country) string { return "+" + c.Dial }))
	reg0("country.currency", cgen(func(c Country) string { return c.Currency }))
	reg0("country.capital", cgen(func(c Country) string { return c.Capital }))
	reg0("country.continent", cgen(func(c Country) string { return c.Continent }))
	reg0("currency.code", cgen(func(c Country) string { return c.Currency }))
	reg0("address.country", cgen(func(c Country) string { return c.Name }))

	reg0("address.city", func(dep any) gen.Generator[any] {
		return gen.New(func(s gen.Source) any {
			c := resolveCountry(dep, s)
			if l := localeForCountry(c.A2); l != nil && len(l.Cities) > 0 {
				return l.Cities[s.Draw(uint64(len(l.Cities)))]
			}
			if c.Capital != "" {
				return c.Capital
			}
			return "Springfield"
		})
	})

	reg0("address.region", func(dep any) gen.Generator[any] {
		return gen.New(func(s gen.Source) any {
			c := resolveCountry(dep, s)
			if l := localeForCountry(c.A2); l != nil && len(l.Regions) > 0 {
				return l.Regions[s.Draw(uint64(len(l.Regions)))]
			}
			return ""
		})
	})

	reg0("address.postal", func(dep any) gen.Generator[any] {
		return gen.New(func(s gen.Source) any {
			c := resolveCountry(dep, s)
			f := "#####"
			if l := localeForCountry(c.A2); l != nil && l.PostalFormat != "" {
				f = l.PostalFormat
			}
			return expandFmt(f).Generate(s.Split())
		})
	})

	reg0("phone", func(dep any) gen.Generator[any] {
		return gen.New(func(s gen.Source) any {
			c := resolveCountry(dep, s)
			dial := c.Dial
			if dial == "" {
				dial = "1"
			}
			return "+" + dial + " " + expandFmt("##########").Generate(s.Split())
		})
	})

	reg0("us.state", func(any) gen.Generator[any] {
		return gen.New(func(s gen.Source) any { return usStates[s.Draw(uint64(len(usStates)))].Name })
	})
	reg0("us.state.code", func(dep any) gen.Generator[any] {
		return gen.New(func(s gen.Source) any {
			if name, ok := dep.(string); ok {
				if ab, found := stateByName[name]; found {
					return ab
				}
			}
			return usStates[s.Draw(uint64(len(usStates)))].Abbr
		})
	})

	reg0("address.full", func(any) gen.Generator[any] {
		return gen.New(func(s gen.Source) any {
			c := countries[s.Draw(uint64(len(countries)))]
			num := 100 + s.Draw(9900)
			city := c.Capital
			f := "#####"
			streets := fallbackStreets
			if l := localeForCountry(c.A2); l != nil {
				if len(l.Cities) > 0 {
					city = l.Cities[s.Draw(uint64(len(l.Cities)))]
				}
				if l.PostalFormat != "" {
					f = l.PostalFormat
				}
				if len(l.Streets) > 0 {
					streets = l.Streets
				}
			}
			street := streets[s.Draw(uint64(len(streets)))]
			postal := expandFmt(f).Generate(s.Split())
			return fmt.Sprintf("%d %s, %s %s, %s", num, street, city, postal, c.Name)
		})
	})

	// NB: year bounds are min/max, not from/to — `from` is reserved.
	Register("date", func(p Params) (MakeFn, error) {
		lo, hi := p.Int("min", 2000), p.Int("max", 2025)
		span := hi - lo + 1
		if span < 1 {
			span = 1
		}
		g := gen.New(func(s gen.Source) string {
			return fmt.Sprintf("%04d-%02d-%02d", lo+int(s.Draw(uint64(span))), 1+int(s.Draw(12)), 1+int(s.Draw(28)))
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	Register("datetime", func(p Params) (MakeFn, error) {
		lo, hi := p.Int("min", 2000), p.Int("max", 2025)
		span := hi - lo + 1
		if span < 1 {
			span = 1
		}
		g := gen.New(func(s gen.Source) string {
			return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02dZ",
				lo+int(s.Draw(uint64(span))), 1+int(s.Draw(12)), 1+int(s.Draw(28)),
				int(s.Draw(24)), int(s.Draw(60)), int(s.Draw(60)))
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	money := func(p Params) (MakeFn, error) {
		min, max := p.Int("min", 1), p.Int("max", 1000)
		sym := p.Str("symbol", "$")
		span := (max-min)*100 + 1
		if span < 1 {
			span = 1
		}
		g := gen.New(func(s gen.Source) string {
			cents := min*100 + int(s.Draw(uint64(span)))
			return fmt.Sprintf("%s%d.%02d", sym, cents/100, cents%100)
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	}
	Register("money", money)
	Register("price", money)
}
