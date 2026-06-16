package data

import (
	"fmt"
	"strings"

	"github.com/tmarq/mvfaker/gen"
)

// A place bundles internally-consistent locale data. Coherence across address
// fields rides the existing `from` mechanism: pick a country, then city/region/
// postal/phone derive from it — the same Bind pattern as email-from-name.
type place struct {
	country   string
	cities    []string
	regions   []string
	postalFmt string // # = digit, @ = uppercase letter
	phoneCC   string
	phoneFmt  string
}

var places = []place{
	{"USA", []string{"Springfield", "Austin", "Portland", "Denver", "Tampa"}, []string{"IL", "TX", "OR", "CO", "FL"}, "#####", "+1", "(###) ###-####"},
	{"UK", []string{"London", "Manchester", "Bristol", "Leeds"}, []string{"England", "Scotland", "Wales"}, "@@## #@@", "+44", "## #### ####"},
	{"Germany", []string{"Berlin", "Munich", "Hamburg", "Cologne"}, []string{"BY", "NW", "BE", "HH"}, "#####", "+49", "### #######"},
	{"Japan", []string{"Tokyo", "Osaka", "Kyoto", "Nagoya"}, []string{"Kanto", "Kansai", "Chubu"}, "###-####", "+81", "##-####-####"},
	{"Brazil", []string{"Sao Paulo", "Rio de Janeiro", "Salvador"}, []string{"SP", "RJ", "BA"}, "#####-###", "+55", "(##) #####-####"},
}

var streets = []string{"Oak St", "Maple Ave", "Main St", "Park Rd", "Cedar Ln", "Elm St", "Hill Rd"}

func placeByCountry(name string) (place, bool) {
	for _, p := range places {
		if p.country == name {
			return p, true
		}
	}
	return place{}, false
}

// resolvePlace returns the place named by dep, or a random one when dep is empty.
func resolvePlace(dep any, s gen.Source) place {
	if c, ok := dep.(string); ok {
		if p, found := placeByCountry(c); found {
			return p
		}
	}
	return places[s.Draw(uint64(len(places)))]
}

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
// realistic skew instead of uniform mush. Order tables most-common first.
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

func init() {
	Register("address.country", func(p Params) (MakeFn, error) {
		g := gen.New(func(s gen.Source) string { return places[s.Draw(uint64(len(places)))].country })
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	Register("address.city", func(p Params) (MakeFn, error) {
		return func(dep any) gen.Generator[any] {
			return gen.New(func(s gen.Source) any {
				pl := resolvePlace(dep, s)
				return pl.cities[s.Draw(uint64(len(pl.cities)))]
			})
		}, nil
	})

	Register("address.region", func(p Params) (MakeFn, error) {
		return func(dep any) gen.Generator[any] {
			return gen.New(func(s gen.Source) any {
				pl := resolvePlace(dep, s)
				return pl.regions[s.Draw(uint64(len(pl.regions)))]
			})
		}, nil
	})

	Register("address.postal", func(p Params) (MakeFn, error) {
		return func(dep any) gen.Generator[any] {
			return gen.New(func(s gen.Source) any {
				pl := resolvePlace(dep, s)
				return expandFmt(pl.postalFmt).Generate(s.Split())
			})
		}, nil
	})

	Register("phone", func(p Params) (MakeFn, error) {
		return func(dep any) gen.Generator[any] {
			return gen.New(func(s gen.Source) any {
				pl := resolvePlace(dep, s)
				return pl.phoneCC + " " + expandFmt(pl.phoneFmt).Generate(s.Split())
			})
		}, nil
	})

	Register("address.full", func(p Params) (MakeFn, error) {
		g := gen.New(func(s gen.Source) string {
			pl := places[s.Draw(uint64(len(places)))]
			num := 100 + s.Draw(9900)
			street := streets[s.Draw(uint64(len(streets)))]
			city := pl.cities[s.Draw(uint64(len(pl.cities)))]
			region := pl.regions[s.Draw(uint64(len(pl.regions)))]
			postal := expandFmt(pl.postalFmt).Generate(s.Split())
			return fmt.Sprintf("%d %s, %s, %s %s, %s", num, street, city, region, postal, pl.country)
		})
		return func(any) gen.Generator[any] { return boxed(g) }, nil
	})

	// NB: year bounds are min/max, not from/to — `from` is reserved for the
	// coherence dependency keyword.
	Register("date", func(p Params) (MakeFn, error) {
		lo, hi := p.Int("min", 2000), p.Int("max", 2025)
		span := hi - lo + 1
		if span < 1 {
			span = 1
		}
		g := gen.New(func(s gen.Source) string {
			y := lo + int(s.Draw(uint64(span)))
			m := 1 + int(s.Draw(12))
			d := 1 + int(s.Draw(28))
			return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
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
			y := lo + int(s.Draw(uint64(span)))
			return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02dZ",
				y, 1+int(s.Draw(12)), 1+int(s.Draw(28)),
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
